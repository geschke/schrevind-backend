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

type CurrenciesController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
	G           *grrt.Grrt
}

// NewCurrenciesController constructs and returns a new instance.
func NewCurrenciesController(database *db.DB, store sessions.Store, sessionName string, g *grrt.Grrt) *CurrenciesController {
	return &CurrenciesController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
		G:           g,
	}
}

// Options handles the CORS preflight request.
func (ct CurrenciesController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type addCurrencyRequest struct {
	ContextGroupID int64  `json:"ContextGroupID"`
	Currency       string `json:"Currency"`
	Name           string `json:"Name"`
	DecimalPlaces  *int64 `json:"DecimalPlaces"`
	Status         string `json:"Status"`
}

type updateCurrencyRequest struct {
	ContextGroupID int64   `json:"ContextGroupID"`
	Currency       *string `json:"Currency"`
	Name           *string `json:"Name"`
	DecimalPlaces  *int64  `json:"DecimalPlaces"`
	Status         *string `json:"Status"`
}

type deleteCurrencyRequest struct {
	ContextGroupID int64 `json:"ContextGroupID"`
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

// currentSessionUserID returns the authenticated user ID from the current session.
func (ct CurrenciesController) currentSessionUserID(c *gin.Context) (int64, bool) {
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

// parseCurrencyID performs its package-specific operation.
func parseCurrencyID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_CURRENCY_ID"})
		return 0, false
	}
	return id, true
}

// parseContextGroupIDQuery parses and validates the context_group_id query parameter.
func parseContextGroupIDQuery(c *gin.Context) (int64, bool) {
	groupID, err := strconv.ParseInt(strings.TrimSpace(c.Query("context_group_id")), 10, 64)
	if err != nil || groupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return 0, false
	}
	return groupID, true
}

// isValidCurrencyStatus performs its package-specific operation.
func isValidCurrencyStatus(status string) bool {
	switch status {
	case db.CurrencyStatusActive, db.CurrencyStatusInactive, db.CurrencyStatusDeleted:
		return true
	default:
		return false
	}
}

// isValidCurrencyStatusFilter performs its package-specific operation.
func isValidCurrencyStatusFilter(status string) bool {
	if status == "" {
		return true
	}
	return isValidCurrencyStatus(status)
}

// parseCurrencyListParams performs its package-specific operation.
func parseCurrencyListParams(c *gin.Context) (int64, int, int, string, string, error) {
	groupID, err := strconv.ParseInt(strings.TrimSpace(c.Query("context_group_id")), 10, 64)
	if err != nil || groupID <= 0 {
		return 0, 0, 0, "", "", errors.New("INVALID_GROUP_ID")
	}

	limit := 10
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 100 {
			return 0, 0, 0, "", "", errors.New("INVALID_LIMIT")
		}
		limit = n
	}

	offset := 0
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, 0, 0, "", "", errors.New("INVALID_OFFSET")
		}
		offset = n
	}

	sortBy := "Currency"
	if v := strings.TrimSpace(c.Query("sort")); v != "" {
		switch v {
		case "Currency", "Name", "DecimalPlaces":
			sortBy = v
		default:
			return 0, 0, 0, "", "", errors.New("INVALID_SORT")
		}
	}

	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if !isValidCurrencyStatusFilter(status) {
		return 0, 0, 0, "", "", errors.New("INVALID_STATUS_FILTER")
	}

	return groupID, limit, offset, sortBy, status, nil
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
	if item.GroupID <= 0 {
		return item, "INVALID_GROUP_ID"
	}

	item.Currency = strings.ToUpper(strings.TrimSpace(item.Currency))
	item.Name = strings.TrimSpace(item.Name)
	item.Status = strings.ToLower(strings.TrimSpace(item.Status))

	if !isValidCurrencyCode(item.Currency) {
		return item, "INVALID_CURRENCY"
	}
	if item.Name == "" {
		return item, "MISSING_NAME"
	}
	if item.DecimalPlaces < 0 {
		return item, "INVALID_DECIMAL_PLACES"
	}
	if !isValidCurrencyStatus(item.Status) {
		return item, "INVALID_STATUS"
	}

	return item, ""
}

// ensureGroupMember requires that the current user belongs to the requested group.
func (ct CurrenciesController) ensureGroupMember(c *gin.Context, userID, groupID int64) bool {
	member, err := ct.DB.IsUserInGroup(groupID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return false
	}
	if !member {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return false
	}
	return true
}

// GET /api/currencies/list
func (ct CurrenciesController) GetList(c *gin.Context) {
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

	groupID, limit, offset, sortBy, status, err := parseCurrencyListParams(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if !ct.ensureGroupMember(c, userID, groupID) {
		return
	}

	items, err := ct.DB.ListCurrenciesByGroupID(groupID, limit, offset, sortBy, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	count, err := ct.DB.CountCurrenciesByGroupID(groupID, status)
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

// GET /api/currencies/:id
func (ct CurrenciesController) GetByID(c *gin.Context) {
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

	currencyID, ok := parseCurrencyID(c)
	if !ok {
		return
	}

	groupID, ok := parseContextGroupIDQuery(c)
	if !ok {
		return
	}

	if !ct.ensureGroupMember(c, userID, groupID) {
		return
	}

	item, err := ct.DB.GetCurrencyByIDAndGroupID(currencyID, groupID)
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

	userID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	var req addCurrencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	decimalPlaces := int64(2)
	if req.DecimalPlaces != nil {
		decimalPlaces = *req.DecimalPlaces
	}

	item, message := normalizeCurrencyPayload(db.Currency{
		GroupID:       req.ContextGroupID,
		Currency:      req.Currency,
		Name:          req.Name,
		DecimalPlaces: decimalPlaces,
		Status:        req.Status,
	})
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	if !ct.ensureGroupMember(c, userID, item.GroupID) {
		return
	}

	existing, err := ct.DB.GetCurrencyByCurrencyAndGroupID(item.Currency, item.GroupID)
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

	userID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	currencyID, ok := parseCurrencyID(c)
	if !ok {
		return
	}

	var req updateCurrencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	if req.ContextGroupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return
	}

	if !ct.ensureGroupMember(c, userID, req.ContextGroupID) {
		return
	}

	existing, err := ct.DB.GetCurrencyByIDAndGroupID(currencyID, req.ContextGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "CURRENCY_NOT_FOUND"})
		return
	}

	updated := *existing
	updated.GroupID = req.ContextGroupID
	if req.Currency != nil {
		updated.Currency = *req.Currency
	}
	if req.Name != nil {
		updated.Name = *req.Name
	}
	if req.DecimalPlaces != nil {
		updated.DecimalPlaces = *req.DecimalPlaces
	}
	if req.Status != nil {
		updated.Status = *req.Status
	}

	updated, message := normalizeCurrencyPayload(updated)
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	other, err := ct.DB.GetCurrencyByCurrencyAndGroupID(updated.Currency, updated.GroupID)
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

	userID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	currencyID, ok := parseCurrencyID(c)
	if !ok {
		return
	}

	var req deleteCurrencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	if req.ContextGroupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return
	}

	item, err := ct.DB.GetCurrencyByIDAndGroupID(currencyID, req.ContextGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "CURRENCY_NOT_FOUND"})
		return
	}

	allowed, err := ct.G.CanDo(userID, db.EntityTypeGroup, "currency:delete", req.ContextGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	if err := ct.DB.DeleteCurrencyByIDAndGroupID(currencyID, req.ContextGroupID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "CURRENCY_DELETED",
	})
}
