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

type CurrenciesController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
}

// NewCurrenciesController constructs and returns a new instance.
func NewCurrenciesController(database *db.DB, store sessions.Store, sessionName string) *CurrenciesController {
	return &CurrenciesController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
	}
}

// Options handles the CORS preflight request.
func (ct CurrenciesController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type addCurrencyRequest struct {
	Currency string `json:"Currency"`
	Name     string `json:"Name"`
	Status   string `json:"Status"`
}

type updateCurrencyRequest struct {
	Currency *string `json:"Currency"`
	Name     *string `json:"Name"`
	Status   *string `json:"Status"`
}

// ensureAuthorized performs its package-specific operation.
func (ct CurrenciesController) ensureAuthorized(c *gin.Context) bool {
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

// parseCurrencyID performs its package-specific operation.
func parseCurrencyID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_CURRENCY_ID"})
		return 0, false
	}
	return id, true
}

// isValidCurrencyStatus performs its package-specific operation.
func isValidCurrencyStatus(status string) bool {
	switch status {
	case "active", "inactive", "deleted":
		return true
	default:
		return false
	}
}

// isValidCurrencyCode performs its package-specific operation.
func isValidCurrencyCode(value string) bool {
	if len(value) != 3 {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 'A' || value[i] > 'Z' {
			return false
		}
	}
	return true
}

// normalizeCurrencyPayload performs its package-specific operation.
func normalizeCurrencyPayload(item db.Currency) (db.Currency, string) {
	item.Currency = strings.ToUpper(strings.TrimSpace(item.Currency))
	item.Name = strings.TrimSpace(item.Name)
	item.Status = strings.ToLower(strings.TrimSpace(item.Status))

	if !isValidCurrencyCode(item.Currency) {
		return item, "INVALID_CURRENCY"
	}
	if item.Name == "" {
		return item, "MISSING_NAME"
	}
	if !isValidCurrencyStatus(item.Status) {
		return item, "INVALID_STATUS"
	}

	return item, ""
}

// GET /api/currencies/list
func (ct CurrenciesController) GetList(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	items, err := ct.DB.ListCurrencies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"items":   items,
	})
}

// GET /api/currencies/:id
func (ct CurrenciesController) GetByID(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	currencyID, ok := parseCurrencyID(c)
	if !ok {
		return
	}

	item, err := ct.DB.GetCurrencyByID(currencyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "CURRENCY_NOT_FOUND"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/currencies/add
func (ct CurrenciesController) PostAdd(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	var req addCurrencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	item, message := normalizeCurrencyPayload(db.Currency{
		Currency: req.Currency,
		Name:     req.Name,
		Status:   req.Status,
	})
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	existing, err := ct.DB.GetCurrencyByCurrency(item.Currency)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "CURRENCY_ALREADY_IN_USE"})
		return
	}

	if err := ct.DB.CreateCurrency(&item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/currencies/update/:id
func (ct CurrenciesController) PostUpdate(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	currencyID, ok := parseCurrencyID(c)
	if !ok {
		return
	}

	existing, err := ct.DB.GetCurrencyByID(currencyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "CURRENCY_NOT_FOUND"})
		return
	}

	var req updateCurrencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	updated := *existing
	if req.Currency != nil {
		updated.Currency = *req.Currency
	}
	if req.Name != nil {
		updated.Name = *req.Name
	}
	if req.Status != nil {
		updated.Status = *req.Status
	}

	updated, message := normalizeCurrencyPayload(updated)
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	other, err := ct.DB.GetCurrencyByCurrency(updated.Currency)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if other != nil && other.ID != updated.ID {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "CURRENCY_ALREADY_IN_USE"})
		return
	}

	if err := ct.DB.UpdateCurrency(&updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    updated,
	})
}

// POST /api/currencies/delete/:id
func (ct CurrenciesController) PostDelete(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	currencyID, ok := parseCurrencyID(c)
	if !ok {
		return
	}

	item, err := ct.DB.GetCurrencyByID(currencyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "CURRENCY_NOT_FOUND"})
		return
	}

	if err := ct.DB.DeleteCurrency(currencyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "CURRENCY_DELETED",
	})
}
