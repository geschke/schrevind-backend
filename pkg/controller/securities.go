package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

type SecuritiesController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
}

// NewSecuritiesController constructs and returns a new instance.
func NewSecuritiesController(database *db.DB, store sessions.Store, sessionName string) *SecuritiesController {
	return &SecuritiesController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
	}
}

// Options handles the CORS preflight request.
func (ct SecuritiesController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type addSecurityRequest struct {
	Name   string `json:"Name"`
	ISIN   string `json:"ISIN"`
	WKN    string `json:"WKN"`
	Symbol string `json:"Symbol"`
	Status string `json:"Status"`
}

type updateSecurityRequest struct {
	Name   *string `json:"Name"`
	ISIN   *string `json:"ISIN"`
	WKN    *string `json:"WKN"`
	Symbol *string `json:"Symbol"`
	Status *string `json:"Status"`
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

// parseSecurityID performs its package-specific operation.
func parseSecurityID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_SECURITY_ID"})
		return 0, false
	}
	return id, true
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
func parseSecurityListParams(c *gin.Context) (int, int, string, string, error) {
	limit := 10
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 100 {
			return 0, 0, "", "", errors.New("INVALID_LIMIT")
		}
		limit = n
	}

	offset := 0
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, 0, "", "", errors.New("INVALID_OFFSET")
		}
		offset = n
	}

	sortBy := "Name"
	if v := strings.TrimSpace(c.Query("sort")); v != "" {
		switch v {
		case "Name", "ISIN", "WKN", "Symbol":
			sortBy = v
		default:
			return 0, 0, "", "", errors.New("INVALID_SORT")
		}
	}

	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if !isValidSecurityStatusFilter(status) {
		return 0, 0, "", "", errors.New("INVALID_STATUS_FILTER")
	}

	return limit, offset, sortBy, status, nil
}

// normalizeSecurityPayload performs its package-specific operation.
func normalizeSecurityPayload(item db.Security) (db.Security, string) {
	item.Name = strings.TrimSpace(item.Name)
	item.ISIN = strings.ToUpper(strings.TrimSpace(item.ISIN))
	item.WKN = strings.TrimSpace(item.WKN)
	item.Symbol = strings.TrimSpace(item.Symbol)
	item.Status = strings.ToLower(strings.TrimSpace(item.Status))

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

// GET /api/securities/list
func (ct SecuritiesController) GetList(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	limit, offset, sortBy, status, err := parseSecurityListParams(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	items, err := ct.DB.ListSecurities(limit, offset, sortBy, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
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

	securityID, ok := parseSecurityID(c)
	if !ok {
		return
	}

	item, err := ct.DB.GetSecurityByID(securityID)
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

	var req addSecurityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	item, message := normalizeSecurityPayload(db.Security{
		Name:   req.Name,
		ISIN:   req.ISIN,
		WKN:    req.WKN,
		Symbol: req.Symbol,
		Status: req.Status,
	})
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	existing, err := ct.DB.GetSecurityByISIN(item.ISIN)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "ISIN_ALREADY_IN_USE"})
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

	securityID, ok := parseSecurityID(c)
	if !ok {
		return
	}

	existing, err := ct.DB.GetSecurityByID(securityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "SECURITY_NOT_FOUND"})
		return
	}

	var req updateSecurityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
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

	other, err := ct.DB.GetSecurityByISIN(updated.ISIN)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if other != nil && other.ID != updated.ID {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "ISIN_ALREADY_IN_USE"})
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

	securityID, ok := parseSecurityID(c)
	if !ok {
		return
	}

	item, err := ct.DB.GetSecurityByID(securityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "SECURITY_NOT_FOUND"})
		return
	}

	if err := ct.DB.DeleteSecurity(securityID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SECURITY_DELETED",
	})
}
