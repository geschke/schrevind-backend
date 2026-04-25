package controller

import (
	"errors"
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

type SecuritiesController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
	G           *grrt.Grrt
}

// NewSecuritiesController constructs and returns a new instance.
func NewSecuritiesController(database *db.DB, store sessions.Store, sessionName string, g *grrt.Grrt) *SecuritiesController {
	return &SecuritiesController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
		G:           g,
	}
}

// Options handles the CORS preflight request.
func (ct SecuritiesController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type addSecurityRequest struct {
	ContextGroupID int64  `json:"ContextGroupID"`
	Name           string `json:"Name"`
	ISIN           string `json:"ISIN"`
	WKN            string `json:"WKN"`
	Symbol         string `json:"Symbol"`
	Status         string `json:"Status"`
}

type updateSecurityRequest struct {
	ContextGroupID int64   `json:"ContextGroupID"`
	Name           *string `json:"Name"`
	ISIN           *string `json:"ISIN"`
	WKN            *string `json:"WKN"`
	Symbol         *string `json:"Symbol"`
	Status         *string `json:"Status"`
}

type deleteSecurityRequest struct {
	ContextGroupID int64 `json:"ContextGroupID"`
}

// ensureAuthorized performs its package-specific operation.
func (ct SecuritiesController) ensureAuthorized(c *gin.Context) bool {
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

// currentSessionUserID returns the authenticated user ID from the current session.
func (ct SecuritiesController) currentSessionUserID(c *gin.Context) (int64, bool) {
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

// ensureGroupMember requires the current user to be a member of the active group context.
func (ct SecuritiesController) ensureGroupMember(c *gin.Context, userID, groupID int64) bool {
	if groupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return false
	}

	inGroup, err := ct.DB.IsUserInGroup(groupID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return false
	}
	if !inGroup {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return false
	}

	return true
}

// parseSecurityID performs its package-specific operation.
func parseSecurityID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_SECURITY_ID"})
		return 0, false
	}
	return id, true
}

// parseSecurityContextGroupIDQuery parses and validates the context_group_id query parameter.
func parseSecurityContextGroupIDQuery(c *gin.Context) (int64, bool) {
	groupID, err := strconv.ParseInt(strings.TrimSpace(c.Query("context_group_id")), 10, 64)
	if err != nil || groupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return 0, false
	}
	return groupID, true
}

// isValidSecurityStatus performs its package-specific operation.
func isValidSecurityStatus(status string) bool {
	switch status {
	case db.SecurityStatusActive, db.SecurityStatusInactive, db.SecurityStatusDeleted:
		return true
	default:
		return false
	}
}

// isValidSecurityStatusFilter performs its package-specific operation.
func isValidSecurityStatusFilter(status string) bool {
	if status == "" {
		return true
	}
	return isValidSecurityStatus(status)
}

// parseSecurityListParams performs its package-specific operation.
func parseSecurityListParams(c *gin.Context) (int, int, string, string, string, error) {
	limit := 10
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 100 {
			return 0, 0, "", "", "", errors.New("INVALID_LIMIT")
		}
		limit = n
	}

	offset := 0
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, 0, "", "", "", errors.New("INVALID_OFFSET")
		}
		offset = n
	}

	sortBy := "Name"
	if v := strings.TrimSpace(c.Query("sort")); v != "" {
		switch v {
		case "ID", "Name", "ISIN", "WKN", "Symbol", "Status", "CreatedAt", "UpdatedAt":
			sortBy = v
		default:
			return 0, 0, "", "", "", errors.New("INVALID_SORT")
		}
	}

	direction := "ASC"
	if v := strings.ToLower(strings.TrimSpace(c.Query("direction"))); v != "" {
		switch v {
		case "asc":
			direction = "ASC"
		case "desc":
			direction = "DESC"
		case "none":
			direction = "ASC"
		default:
			return 0, 0, "", "", "", errors.New("INVALID_DIRECTION")
		}
	}

	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if !isValidSecurityStatusFilter(status) {
		return 0, 0, "", "", "", errors.New("INVALID_STATUS_FILTER")
	}

	return limit, offset, sortBy, direction, status, nil
}

// normalizeSecurityPayload performs its package-specific operation.
func normalizeSecurityPayload(item db.Security) (db.Security, string) {
	item.Name = strings.TrimSpace(item.Name)
	item.ISIN = strings.ToUpper(strings.TrimSpace(item.ISIN))
	item.WKN = strings.TrimSpace(item.WKN)
	item.Symbol = strings.TrimSpace(item.Symbol)
	item.Status = strings.ToLower(strings.TrimSpace(item.Status))

	if item.GroupID <= 0 {
		return item, "INVALID_GROUP_ID"
	}
	if item.Name == "" {
		return item, "MISSING_NAME"
	}
	if item.ISIN == "" {
		return item, "MISSING_ISIN"
	}
	if !isValidSecurityStatus(item.Status) {
		return item, "INVALID_STATUS"
	}

	return item, ""
}

func (ct SecuritiesController) ensureNoSecurityConflicts(c *gin.Context, item db.Security) bool {
	existingByISIN, err := ct.DB.GetSecurityByISINAndGroupID(item.ISIN, item.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return false
	}
	if existingByISIN != nil && existingByISIN.ID != item.ID {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "ISIN_ALREADY_IN_USE"})
		return false
	}

	existingByName, err := ct.DB.GetSecurityByNameAndGroupID(item.Name, item.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return false
	}
	if existingByName != nil && existingByName.ID != item.ID {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "SECURITY_NAME_ALREADY_IN_USE"})
		return false
	}

	return true
}

// GET /api/securities/list
func (ct SecuritiesController) GetList(c *gin.Context) {
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
	groupID, ok := parseSecurityContextGroupIDQuery(c)
	if !ok {
		return
	}
	if !ct.ensureGroupMember(c, userID, groupID) {
		return
	}

	limit, offset, sortBy, direction, status, err := parseSecurityListParams(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	items, err := ct.DB.ListSecuritiesByGroupID(groupID, limit, offset, sortBy, direction, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	count, err := ct.DB.CountSecuritiesByGroupID(groupID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   count,
		"items":   items,
	})
}

// GET /api/securities/list-all
func (ct SecuritiesController) GetListAll(c *gin.Context) {
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
	groupID, ok := parseSecurityContextGroupIDQuery(c)
	if !ok {
		return
	}
	if !ct.ensureGroupMember(c, userID, groupID) {
		return
	}

	items, err := ct.DB.ListAllSecuritiesByGroupID(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   len(items),
		"items":   items,
	})
}

// GET /api/securities/:id
func (ct SecuritiesController) GetByID(c *gin.Context) {
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
	groupID, ok := parseSecurityContextGroupIDQuery(c)
	if !ok {
		return
	}
	if !ct.ensureGroupMember(c, userID, groupID) {
		return
	}

	securityID, ok := parseSecurityID(c)
	if !ok {
		return
	}

	item, err := ct.DB.GetSecurityByIDAndGroupID(securityID, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "SECURITY_NOT_FOUND"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/securities/add
func (ct SecuritiesController) PostAdd(c *gin.Context) {
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

	var req addSecurityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	if !ct.ensureGroupMember(c, userID, req.ContextGroupID) {
		return
	}

	item, message := normalizeSecurityPayload(db.Security{
		GroupID: req.ContextGroupID,
		Name:    req.Name,
		ISIN:    req.ISIN,
		WKN:     req.WKN,
		Symbol:  req.Symbol,
		Status:  req.Status,
	})
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	if !ct.ensureNoSecurityConflicts(c, item) {
		return
	}

	if err := ct.DB.CreateSecurity(&item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/securities/update/:id
func (ct SecuritiesController) PostUpdate(c *gin.Context) {
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

	securityID, ok := parseSecurityID(c)
	if !ok {
		return
	}

	var req updateSecurityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	if !ct.ensureGroupMember(c, userID, req.ContextGroupID) {
		return
	}

	existing, err := ct.DB.GetSecurityByIDAndGroupID(securityID, req.ContextGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "SECURITY_NOT_FOUND"})
		return
	}

	updated := *existing
	if req.Name != nil {
		updated.Name = *req.Name
	}
	if req.ISIN != nil {
		updated.ISIN = *req.ISIN
	}
	if req.WKN != nil {
		updated.WKN = *req.WKN
	}
	if req.Symbol != nil {
		updated.Symbol = *req.Symbol
	}
	if req.Status != nil {
		updated.Status = *req.Status
	}

	updated, message := normalizeSecurityPayload(updated)
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	if !ct.ensureNoSecurityConflicts(c, updated) {
		return
	}

	if err := ct.DB.UpdateSecurity(&updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    updated,
	})
}

// POST /api/securities/delete/:id
func (ct SecuritiesController) PostDelete(c *gin.Context) {
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

	securityID, ok := parseSecurityID(c)
	if !ok {
		return
	}

	var req deleteSecurityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	if req.ContextGroupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return
	}

	allowed, err := ct.G.CanDo(userID, db.EntityTypeGroup, "security:delete", req.ContextGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	item, err := ct.DB.GetSecurityByIDAndGroupID(securityID, req.ContextGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "SECURITY_NOT_FOUND"})
		return
	}

	hasEntries, err := ct.DB.SecurityHasDividendEntries(securityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if hasEntries {
		if err := ct.DB.SetSecurityStatus(securityID, req.ContextGroupID, db.SecurityStatusInactive); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "SECURITY_DEACTIVATED",
		})
		return
	}

	if err := ct.DB.DeleteSecurity(securityID, req.ContextGroupID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SECURITY_DELETED",
	})
}
