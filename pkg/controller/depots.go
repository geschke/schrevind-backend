package controller

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

type DepotsController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
}

// NewDepotsController constructs and returns a new instance.
func NewDepotsController(database *db.DB, store sessions.Store, sessionName string) *DepotsController {
	return &DepotsController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
	}
}

// Options handles the CORS preflight request.
func (ct DepotsController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type addDepotRequest struct {
	UserID        int64  `json:"UserID"`
	Name          string `json:"Name"`
	BrokerName    string `json:"BrokerName"`
	AccountNumber string `json:"AccountNumber"`
	BaseCurrency  string `json:"BaseCurrency"`
	Description   string `json:"Description"`
	Status        string `json:"Status"`
}

type updateDepotRequest struct {
	UserID        *int64  `json:"UserID"`
	Name          *string `json:"Name"`
	BrokerName    *string `json:"BrokerName"`
	AccountNumber *string `json:"AccountNumber"`
	BaseCurrency  *string `json:"BaseCurrency"`
	Description   *string `json:"Description"`
	Status        *string `json:"Status"`
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

	if item.UserID <= 0 {
		return item, "INVALID_USER_ID"
	}
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

	items, err := ct.DB.ListDepotsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
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

	item, err := ct.DB.GetDepotByID(depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "DEPOT_NOT_FOUND"})
		return
	}
	if item.UserID != sessionUserID {
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

	if req.UserID != sessionUserID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_USER"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := ct.DB.UserExistsByID(ctx, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "USER_NOT_FOUND"})
		return
	}

	item, message := normalizeDepotPayload(db.Depot{
		UserID:        req.UserID,
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

	existing, err := ct.DB.GetDepotByID(depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "DEPOT_NOT_FOUND"})
		return
	}
	if existing.UserID != sessionUserID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	var req updateDepotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	updated := *existing
	if req.UserID != nil {
		updated.UserID = *req.UserID
	}
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

	if updated.UserID != sessionUserID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_USER"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := ct.DB.UserExistsByID(ctx, updated.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "USER_NOT_FOUND"})
		return
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

	item, err := ct.DB.GetDepotByID(depotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "DEPOT_NOT_FOUND"})
		return
	}
	if item.UserID != sessionUserID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	if err := ct.DB.DeleteDepot(depotID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "DEPOT_DELETED",
	})
}
