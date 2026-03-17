package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

type WithholdingTaxDefaultsController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
}

// NewWithholdingTaxDefaultsController constructs and returns a new instance.
func NewWithholdingTaxDefaultsController(database *db.DB, store sessions.Store, sessionName string) *WithholdingTaxDefaultsController {
	return &WithholdingTaxDefaultsController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
	}
}

// Options handles the CORS preflight request.
func (ct WithholdingTaxDefaultsController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type addWithholdingTaxDefaultRequest struct {
	DepotID                            int64  `json:"DepotID"`
	CountryCode                        string `json:"CountryCode"`
	CountryName                        string `json:"CountryName"`
	WithholdingTaxPercentDefault       string `json:"WithholdingTaxPercentDefault"`
	WithholdingTaxPercentCreditDefault string `json:"WithholdingTaxPercentCreditDefault"`
}

type updateWithholdingTaxDefaultRequest struct {
	DepotID                            *int64  `json:"DepotID"`
	CountryCode                        *string `json:"CountryCode"`
	CountryName                        *string `json:"CountryName"`
	WithholdingTaxPercentDefault       *string `json:"WithholdingTaxPercentDefault"`
	WithholdingTaxPercentCreditDefault *string `json:"WithholdingTaxPercentCreditDefault"`
}

// ensureAuthorized performs its package-specific operation.
func (ct WithholdingTaxDefaultsController) ensureAuthorized(c *gin.Context) bool {
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

// parseWithholdingTaxDefaultID performs its package-specific operation.
func parseWithholdingTaxDefaultID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_WITHHOLDING_TAX_DEFAULT_ID"})
		return 0, false
	}
	return id, true
}

// parseDepotIDParam performs its package-specific operation.
func parseDepotIDParam(c *gin.Context, paramName string) (int64, bool) {
	depotID, err := strconv.ParseInt(strings.TrimSpace(c.Param(paramName)), 10, 64)
	if err != nil || depotID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_DEPOT_ID"})
		return 0, false
	}
	return depotID, true
}

// parseOptionalDepotIDQuery performs its package-specific operation.
func parseOptionalDepotIDQuery(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("depot_id"))
	if raw == "" {
		return 0, true
	}

	depotID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || depotID < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_DEPOT_ID"})
		return 0, false
	}
	return depotID, true
}

// normalizeWithholdingTaxDefaultPayload performs its package-specific operation.
func normalizeWithholdingTaxDefaultPayload(item db.WithholdingTaxDefault) (db.WithholdingTaxDefault, string) {
	if item.DepotID < 0 {
		return item, "INVALID_DEPOT_ID"
	}

	item.CountryCode = strings.ToUpper(strings.TrimSpace(item.CountryCode))
	item.CountryName = strings.TrimSpace(item.CountryName)
	item.WithholdingTaxPercentDefault = strings.TrimSpace(item.WithholdingTaxPercentDefault)
	item.WithholdingTaxPercentCreditDefault = strings.TrimSpace(item.WithholdingTaxPercentCreditDefault)

	if item.CountryCode == "" {
		return item, "MISSING_COUNTRY_CODE"
	}
	if item.CountryName == "" {
		return item, "MISSING_COUNTRY_NAME"
	}

	return item, ""
}

// GET /api/withholding-tax-defaults/list
func (ct WithholdingTaxDefaultsController) GetList(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	items, err := ct.DB.ListWithholdingTaxDefaults()
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

// GET /api/withholding-tax-defaults/by-depot/:depot_id
func (ct WithholdingTaxDefaultsController) GetListByDepot(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	depotID, ok := parseDepotIDParam(c, "depot_id")
	if !ok {
		return
	}

	items, err := ct.DB.ListWithholdingTaxDefaultsByDepotID(depotID)
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

// GET /api/withholding-tax-defaults/effective
func (ct WithholdingTaxDefaultsController) GetEffective(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	depotID, ok := parseOptionalDepotIDQuery(c)
	if !ok {
		return
	}

	countryCode := strings.TrimSpace(c.Query("country_code"))
	if countryCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_COUNTRY_CODE"})
		return
	}

	item, err := ct.DB.GetWithholdingTaxDefault(depotID, countryCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "WITHHOLDING_TAX_DEFAULT_NOT_FOUND"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// GET /api/withholding-tax-defaults/:id
func (ct WithholdingTaxDefaultsController) GetByID(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	id, ok := parseWithholdingTaxDefaultID(c)
	if !ok {
		return
	}

	item, err := ct.DB.GetWithholdingTaxDefaultByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "WITHHOLDING_TAX_DEFAULT_NOT_FOUND"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/withholding-tax-defaults/add
func (ct WithholdingTaxDefaultsController) PostAdd(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	var req addWithholdingTaxDefaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	item, message := normalizeWithholdingTaxDefaultPayload(db.WithholdingTaxDefault{
		DepotID:                            req.DepotID,
		CountryCode:                        req.CountryCode,
		CountryName:                        req.CountryName,
		WithholdingTaxPercentDefault:       req.WithholdingTaxPercentDefault,
		WithholdingTaxPercentCreditDefault: req.WithholdingTaxPercentCreditDefault,
	})
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	if err := ct.DB.CreateWithholdingTaxDefault(&item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/withholding-tax-defaults/update/:id
func (ct WithholdingTaxDefaultsController) PostUpdate(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	id, ok := parseWithholdingTaxDefaultID(c)
	if !ok {
		return
	}

	existing, err := ct.DB.GetWithholdingTaxDefaultByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "WITHHOLDING_TAX_DEFAULT_NOT_FOUND"})
		return
	}

	var req updateWithholdingTaxDefaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	updated := *existing
	if req.DepotID != nil {
		updated.DepotID = *req.DepotID
	}
	if req.CountryCode != nil {
		updated.CountryCode = *req.CountryCode
	}
	if req.CountryName != nil {
		updated.CountryName = *req.CountryName
	}
	if req.WithholdingTaxPercentDefault != nil {
		updated.WithholdingTaxPercentDefault = *req.WithholdingTaxPercentDefault
	}
	if req.WithholdingTaxPercentCreditDefault != nil {
		updated.WithholdingTaxPercentCreditDefault = *req.WithholdingTaxPercentCreditDefault
	}

	updated, message := normalizeWithholdingTaxDefaultPayload(updated)
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	if err := ct.DB.UpdateWithholdingTaxDefault(&updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    updated,
	})
}

// POST /api/withholding-tax-defaults/delete/:id
func (ct WithholdingTaxDefaultsController) PostDelete(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	id, ok := parseWithholdingTaxDefaultID(c)
	if !ok {
		return
	}

	item, err := ct.DB.GetWithholdingTaxDefaultByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "WITHHOLDING_TAX_DEFAULT_NOT_FOUND"})
		return
	}

	if err := ct.DB.DeleteWithholdingTaxDefault(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "WITHHOLDING_TAX_DEFAULT_DELETED",
	})
}
