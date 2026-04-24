package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/grrt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

type DepotsController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
	G           *grrt.Grrt
}

// NewDepotsController constructs and returns a new instance.
func NewDepotsController(database *db.DB, store sessions.Store, sessionName string, g *grrt.Grrt) *DepotsController {
	return &DepotsController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
		G:           g,
	}
}

// Options handles the CORS preflight request.
func (ct DepotsController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type addDepotRequest struct {
	// ContextGroupID is used only for the permission check (depot:create requires group admin).
	// It is not stored on the depot itself.
	ContextGroupID int64  `json:"ContextGroupID"`
	Name           string `json:"Name"`
	BrokerName     string `json:"BrokerName"`
	AccountNumber  string `json:"AccountNumber"`
	BaseCurrency   string `json:"BaseCurrency"`
	Description    string `json:"Description"`
	Status         string `json:"Status"`
}

type updateDepotRequest struct {
	Name          *string `json:"Name"`
	BrokerName    *string `json:"BrokerName"`
	AccountNumber *string `json:"AccountNumber"`
	BaseCurrency  *string `json:"BaseCurrency"`
	Description   *string `json:"Description"`
	Status        *string `json:"Status"`
}

type depotAccessAddRequest struct {
	UserID int64  `json:"UserID"`
	Role   string `json:"Role"`
}

type depotAccessRemoveRequest struct {
	UserID int64 `json:"UserID"`
}

type depotAccessChangeRequest struct {
	UserID int64  `json:"UserID"`
	Role   string `json:"Role"`
}

// ensureAuthorized performs its package-specific operation.
func (ct DepotsController) ensureAuthorized(c *gin.Context) bool {
	if ct.DB == nil || ct.DB.SQL == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_NOT_INITIALIZED"})
		return false
	}
	if ct.Store == nil || strings.TrimSpace(ct.SessionName) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "AUTH_NOT_CONFIGURED"})
		return false
	}

	sess, _ := ct.Store.Get(c.Request, ct.SessionName)
	if sess == nil || sess.IsNew {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return false
	}
	if _, ok := sess.Values["id"]; !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return false
	}

	return true
}

// currentSessionUserID performs its package-specific operation.
func (ct DepotsController) currentSessionUserID(c *gin.Context) (int64, bool) {
	sess, _ := ct.Store.Get(c.Request, ct.SessionName)
	if sess == nil {
		return 0, false
	}
	raw, ok := sess.Values["id"]
	if !ok {
		return 0, false
	}
	id, ok := raw.(int64)
	if !ok {
		return 0, false
	}
	return id, true
}

// parseDepotID performs its package-specific operation.
func parseDepotID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_DEPOT_ID"})
		return 0, false
	}
	return id, true
}

// isValidDepotStatus performs its package-specific operation.
func isValidDepotStatus(status string) bool {
	switch status {
	case "active", "inactive", "deleted":
		return true
	default:
		return false
	}
}

// isValidBaseCurrency performs its package-specific operation.
func isValidBaseCurrency(v string) bool {
	if len(v) != 3 {
		return false
	}
	for i := 0; i < len(v); i++ {
		if v[i] < 'A' || v[i] > 'Z' {
			return false
		}
	}
	return true
}

// normalizeDepotPayload performs its package-specific operation.
func normalizeDepotPayload(item db.Depot) (db.Depot, string) {
	item.Name = strings.TrimSpace(item.Name)
	item.BrokerName = strings.TrimSpace(item.BrokerName)
	item.AccountNumber = strings.TrimSpace(item.AccountNumber)
	item.BaseCurrency = strings.ToUpper(strings.TrimSpace(item.BaseCurrency))
	item.Description = strings.TrimSpace(item.Description)
	item.Status = strings.ToLower(strings.TrimSpace(item.Status))

	if item.Name == "" {
		return item, "MISSING_NAME"
	}
	if !isValidBaseCurrency(item.BaseCurrency) {
		return item, "INVALID_BASE_CURRENCY"
	}
	if !isValidDepotStatus(item.Status) {
		return item, "INVALID_STATUS"
	}

	return item, ""
}

// GET /api/depots/list
func (ct DepotsController) GetList(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	userID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	items, err := ct.DB.ListDepotsByUserMembership(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   int64(len(items)),
		"items":   items,
	})
}

// GET /api/depots/:id
func (ct DepotsController) GetByID(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	depotID, ok := parseDepotID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	item, found, err := ct.DB.GetDepotByID(depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "DEPOT_NOT_FOUND"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "entries:list", depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/depots/add
func (ct DepotsController) PostAdd(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	var req addDepotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	// todo error handling... user is in > 1 groups. primary group? choose groups?
	if req.ContextGroupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeGroup, "depot:create", req.ContextGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_GROUP"})
		return
	}

	item, message := normalizeDepotPayload(db.Depot{
		Name:          req.Name,
		BrokerName:    req.BrokerName,
		AccountNumber: req.AccountNumber,
		BaseCurrency:  req.BaseCurrency,
		Description:   req.Description,
		Status:        req.Status,
	})
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	if err := ct.DB.CreateDepot(&item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	// Auto-grant the creating user as depot owner.
	if err := ct.DB.GrantMembership(&db.Membership{
		EntityType: db.EntityTypeDepot,
		EntityID:   item.ID,
		UserID:     sessionUserID,
		Role:       db.RoleDepotOwner,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionCreate,
		EntityType: db.EntityTypeDepot,
		EntityID:   item.ID,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/depots/update/:id
func (ct DepotsController) PostUpdate(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	depotID, ok := parseDepotID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	existing, found, err := ct.DB.GetDepotByID(depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "DEPOT_NOT_FOUND"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "depot:rename", depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	var req updateDepotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	updated := existing
	if req.Name != nil {
		updated.Name = *req.Name
	}
	if req.BrokerName != nil {
		updated.BrokerName = *req.BrokerName
	}
	if req.AccountNumber != nil {
		updated.AccountNumber = *req.AccountNumber
	}
	if req.BaseCurrency != nil {
		updated.BaseCurrency = *req.BaseCurrency
	}
	if req.Description != nil {
		updated.Description = *req.Description
	}
	if req.Status != nil {
		updated.Status = *req.Status
	}

	updated, message := normalizeDepotPayload(updated)
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	if err := ct.DB.UpdateDepot(&updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionUpdate,
		EntityType: db.EntityTypeDepot,
		EntityID:   depotID,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    updated,
	})
}

// POST /api/depots/delete/:id
func (ct DepotsController) PostDelete(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	depotID, ok := parseDepotID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	_, found, err := ct.DB.GetDepotByID(depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "DEPOT_NOT_FOUND"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "depot:delete", depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	if err := ct.DB.DeleteDepot(depotID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionDelete,
		EntityType: db.EntityTypeDepot,
		EntityID:   depotID,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "DEPOT_DELETED",
	})
}

// GET /api/depots/:id/access
func (ct DepotsController) GetAccess(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	depotID, ok := parseDepotID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "depot:access:list", depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	items, err := ct.DB.ListMembershipsByEntity(db.EntityTypeDepot, depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   int64(len(items)),
		"items":   items,
	})
}

// POST /api/depots/:id/access/add
func (ct DepotsController) PostAccessAdd(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	depotID, ok := parseDepotID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "depot:access:add", depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	var req depotAccessAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	if req.UserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_USER_ID"})
		return
	}
	if !db.IsValidDepotRole(req.Role) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_ROLE"})
		return
	}

	if err := ct.DB.GrantMembership(&db.Membership{
		EntityType: db.EntityTypeDepot,
		EntityID:   depotID,
		UserID:     req.UserID,
		Role:       req.Role,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionGrant,
		EntityType: db.EntityTypeDepot,
		EntityID:   depotID,
		Detail:     strings.Join([]string{strconv.FormatInt(req.UserID, 10), req.Role}, ":"),
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "ACCESS_GRANTED",
	})
}

// POST /api/depots/:id/access/remove
func (ct DepotsController) PostAccessRemove(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	depotID, ok := parseDepotID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "depot:access:remove", depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	var req depotAccessRemoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	if req.UserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_USER_ID"})
		return
	}

	// Guard: at least one owner must remain.
	existing, found, err := ct.DB.GetMembership(db.EntityTypeDepot, depotID, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "ACCESS_NOT_FOUND"})
		return
	}
	if existing.Role == db.RoleDepotOwner {
		count, err := ct.DB.CountDepotOwnerMemberships(depotID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if count <= 1 {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "LAST_OWNER"})
			return
		}
	}

	removed, err := ct.DB.RevokeMembership(db.EntityTypeDepot, depotID, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !removed {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "ACCESS_NOT_FOUND"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionRevoke,
		EntityType: db.EntityTypeDepot,
		EntityID:   depotID,
		Detail:     strconv.FormatInt(req.UserID, 10),
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "ACCESS_REVOKED",
	})
}

// POST /api/depots/:id/access/change
func (ct DepotsController) PostAccessChange(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	depotID, ok := parseDepotID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "depot:access:change", depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	var req depotAccessChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	if req.UserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_USER_ID"})
		return
	}
	if !db.IsValidDepotRole(req.Role) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_ROLE"})
		return
	}

	// Guard: degrading the last owner is not allowed.
	existing, found, err := ct.DB.GetMembership(db.EntityTypeDepot, depotID, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "ACCESS_NOT_FOUND"})
		return
	}
	if existing.Role == db.RoleDepotOwner && req.Role != db.RoleDepotOwner {
		count, err := ct.DB.CountDepotOwnerMemberships(depotID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if count <= 1 {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "LAST_OWNER"})
			return
		}
	}

	if err := ct.DB.GrantMembership(&db.Membership{
		EntityType: db.EntityTypeDepot,
		EntityID:   depotID,
		UserID:     req.UserID,
		Role:       req.Role,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionUpdate,
		EntityType: db.EntityTypeDepot,
		EntityID:   depotID,
		Detail:     strings.Join([]string{strconv.FormatInt(req.UserID, 10), req.Role}, ":"),
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "ACCESS_CHANGED",
	})
}
