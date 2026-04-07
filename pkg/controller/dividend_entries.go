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

type DividendEntriesController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
	G           *grrt.Grrt
}

// NewDividendEntriesController constructs and returns a new instance.
func NewDividendEntriesController(database *db.DB, store sessions.Store, sessionName string, g *grrt.Grrt) *DividendEntriesController {
	return &DividendEntriesController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
		G:           g,
	}
}

// Options handles the CORS preflight request.
func (ct DividendEntriesController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type addDividendEntryRequest struct {
	DepotID    int64  `json:"DepotID"`
	SecurityID int64  `json:"SecurityID"`
	PayDate    string `json:"PayDate"`
	ExDate     string `json:"ExDate"`

	SecurityName   string `json:"SecurityName"`
	SecurityISIN   string `json:"SecurityISIN"`
	SecurityWKN    string `json:"SecurityWKN"`
	SecuritySymbol string `json:"SecuritySymbol"`

	Quantity string `json:"Quantity"`

	DividendPerUnitAmount   string `json:"DividendPerUnitAmount"`
	DividendPerUnitCurrency string `json:"DividendPerUnitCurrency"`

	FXRateLabel string `json:"FXRateLabel"`
	FXRate      string `json:"FXRate"`

	GrossAmount   string `json:"GrossAmount"`
	GrossCurrency string `json:"GrossCurrency"`

	PayoutAmount   string `json:"PayoutAmount"`
	PayoutCurrency string `json:"PayoutCurrency"`

	WithholdingTaxCountryCode string `json:"WithholdingTaxCountryCode"`
	WithholdingTaxPercent     string `json:"WithholdingTaxPercent"`

	WithholdingTaxAmount   string `json:"WithholdingTaxAmount"`
	WithholdingTaxCurrency string `json:"WithholdingTaxCurrency"`

	WithholdingTaxAmountCredit         string `json:"WithholdingTaxAmountCredit"`
	WithholdingTaxAmountCreditCurrency string `json:"WithholdingTaxAmountCreditCurrency"`

	WithholdingTaxAmountRefundable         string `json:"WithholdingTaxAmountRefundable"`
	WithholdingTaxAmountRefundableCurrency string `json:"WithholdingTaxAmountRefundableCurrency"`

	ForeignFeesAmount   string `json:"ForeignFeesAmount"`
	ForeignFeesCurrency string `json:"ForeignFeesCurrency"`

	Note string `json:"Note"`
}

type updateDividendEntryRequest struct {
	DepotID    *int64  `json:"DepotID"`
	SecurityID *int64  `json:"SecurityID"`
	PayDate    *string `json:"PayDate"`
	ExDate     *string `json:"ExDate"`

	SecurityName   *string `json:"SecurityName"`
	SecurityISIN   *string `json:"SecurityISIN"`
	SecurityWKN    *string `json:"SecurityWKN"`
	SecuritySymbol *string `json:"SecuritySymbol"`

	Quantity *string `json:"Quantity"`

	DividendPerUnitAmount   *string `json:"DividendPerUnitAmount"`
	DividendPerUnitCurrency *string `json:"DividendPerUnitCurrency"`

	FXRateLabel *string `json:"FXRateLabel"`
	FXRate      *string `json:"FXRate"`

	GrossAmount   *string `json:"GrossAmount"`
	GrossCurrency *string `json:"GrossCurrency"`

	PayoutAmount   *string `json:"PayoutAmount"`
	PayoutCurrency *string `json:"PayoutCurrency"`

	WithholdingTaxCountryCode *string `json:"WithholdingTaxCountryCode"`
	WithholdingTaxPercent     *string `json:"WithholdingTaxPercent"`

	WithholdingTaxAmount   *string `json:"WithholdingTaxAmount"`
	WithholdingTaxCurrency *string `json:"WithholdingTaxCurrency"`

	WithholdingTaxAmountCredit         *string `json:"WithholdingTaxAmountCredit"`
	WithholdingTaxAmountCreditCurrency *string `json:"WithholdingTaxAmountCreditCurrency"`

	WithholdingTaxAmountRefundable         *string `json:"WithholdingTaxAmountRefundable"`
	WithholdingTaxAmountRefundableCurrency *string `json:"WithholdingTaxAmountRefundableCurrency"`

	ForeignFeesAmount   *string `json:"ForeignFeesAmount"`
	ForeignFeesCurrency *string `json:"ForeignFeesCurrency"`

	Note *string `json:"Note"`
}

// ensureAuthorized performs its package-specific operation.
func (ct DividendEntriesController) ensureAuthorized(c *gin.Context) bool {
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
func (ct DividendEntriesController) currentSessionUserID(c *gin.Context) (int64, bool) {
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

// parseDividendEntryID performs its package-specific operation.
func parseDividendEntryID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_DIVIDEND_ENTRY_ID"})
		return 0, false
	}
	return id, true
}

// parseDepotIDParamForDividendEntries performs its package-specific operation.
func parseDepotIDParamForDividendEntries(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("depot_id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_DEPOT_ID"})
		return 0, false
	}
	return id, true
}

// parseSecurityIDParam performs its package-specific operation.
func parseSecurityIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("security_id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_SECURITY_ID"})
		return 0, false
	}
	return id, true
}

// parseDividendEntryListParams performs its package-specific operation.
func parseDividendEntryListParams(c *gin.Context) (int, int, string, string, string, bool) {
	limit := 20
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_LIMIT"})
			return 0, 0, "", "", "", false
		}
		limit = n
	}

	offset := 0
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_OFFSET"})
			return 0, 0, "", "", "", false
		}
		offset = n
	}

	sortBy := "PayDate"
	if v := strings.TrimSpace(c.Query("sort")); v != "" {
		switch v {
		case "PayDate", "ExDate", "SecurityName":
			sortBy = v
		default:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_SORT"})
			return 0, 0, "", "", "", false
		}
	}

	fromDate := strings.TrimSpace(c.Query("from"))
	toDate := strings.TrimSpace(c.Query("to"))
	if fromDate != "" && toDate != "" && fromDate > toDate {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_DATE_RANGE"})
		return 0, 0, "", "", "", false
	}

	return limit, offset, sortBy, fromDate, toDate, true
}

// normalizeDividendEntryPayload performs its package-specific operation.
func normalizeDividendEntryPayload(entry db.DividendEntry) (db.DividendEntry, string) {
	entry.PayDate = strings.TrimSpace(entry.PayDate)
	entry.ExDate = strings.TrimSpace(entry.ExDate)
	entry.SecurityName = strings.TrimSpace(entry.SecurityName)
	entry.SecurityISIN = strings.TrimSpace(entry.SecurityISIN)
	entry.SecurityWKN = strings.TrimSpace(entry.SecurityWKN)
	entry.SecuritySymbol = strings.TrimSpace(entry.SecuritySymbol)
	entry.Quantity = strings.TrimSpace(entry.Quantity)
	entry.DividendPerUnitAmount = strings.TrimSpace(entry.DividendPerUnitAmount)
	entry.DividendPerUnitCurrency = strings.TrimSpace(entry.DividendPerUnitCurrency)
	entry.FXRateLabel = strings.TrimSpace(entry.FXRateLabel)
	entry.FXRate = strings.TrimSpace(entry.FXRate)
	entry.GrossAmount = strings.TrimSpace(entry.GrossAmount)
	entry.GrossCurrency = strings.TrimSpace(entry.GrossCurrency)
	entry.PayoutAmount = strings.TrimSpace(entry.PayoutAmount)
	entry.PayoutCurrency = strings.TrimSpace(entry.PayoutCurrency)
	entry.WithholdingTaxCountryCode = strings.TrimSpace(entry.WithholdingTaxCountryCode)
	entry.WithholdingTaxPercent = strings.TrimSpace(entry.WithholdingTaxPercent)
	entry.WithholdingTaxAmount = strings.TrimSpace(entry.WithholdingTaxAmount)
	entry.WithholdingTaxCurrency = strings.TrimSpace(entry.WithholdingTaxCurrency)
	entry.WithholdingTaxAmountCredit = strings.TrimSpace(entry.WithholdingTaxAmountCredit)
	entry.WithholdingTaxAmountCreditCurrency = strings.TrimSpace(entry.WithholdingTaxAmountCreditCurrency)
	entry.WithholdingTaxAmountRefundable = strings.TrimSpace(entry.WithholdingTaxAmountRefundable)
	entry.WithholdingTaxAmountRefundableCurrency = strings.TrimSpace(entry.WithholdingTaxAmountRefundableCurrency)
	entry.ForeignFeesAmount = strings.TrimSpace(entry.ForeignFeesAmount)
	entry.ForeignFeesCurrency = strings.TrimSpace(entry.ForeignFeesCurrency)
	entry.Note = strings.TrimSpace(entry.Note)

	if entry.DepotID <= 0 {
		return entry, "INVALID_DEPOT_ID"
	}
	if entry.SecurityID <= 0 {
		return entry, "INVALID_SECURITY_ID"
	}
	if entry.PayDate == "" {
		return entry, "MISSING_PAY_DATE"
	}
	if entry.ExDate == "" {
		return entry, "MISSING_EX_DATE"
	}
	if entry.SecurityName == "" {
		return entry, "MISSING_SECURITY_NAME"
	}
	if entry.SecurityISIN == "" {
		return entry, "MISSING_SECURITY_ISIN"
	}
	if entry.Quantity == "" {
		return entry, "MISSING_QUANTITY"
	}
	if entry.DividendPerUnitAmount == "" {
		return entry, "MISSING_DIVIDEND_PER_UNIT_AMOUNT"
	}
	if entry.DividendPerUnitCurrency == "" {
		return entry, "MISSING_DIVIDEND_PER_UNIT_CURRENCY"
	}
	if entry.GrossAmount == "" {
		return entry, "MISSING_GROSS_AMOUNT"
	}
	if entry.GrossCurrency == "" {
		return entry, "MISSING_GROSS_CURRENCY"
	}
	if entry.PayoutAmount == "" {
		return entry, "MISSING_PAYOUT_AMOUNT"
	}
	if entry.PayoutCurrency == "" {
		return entry, "MISSING_PAYOUT_CURRENCY"
	}

	return entry, ""
}

// prepareCalculatedDividendFields performs its package-specific operation.
func prepareCalculatedDividendFields(entry db.DividendEntry) db.DividendEntry {
	// Keep calculated fields backend-owned until dedicated calculation logic is added.
	return entry
}

// GET /api/dividend-entries/:id
func (ct DividendEntriesController) GetByID(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	id, ok := parseDividendEntryID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	item, found, err := ct.DB.GetDividendEntryByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "DIVIDEND_ENTRY_NOT_FOUND"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "entries:list", item.DepotID)
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

// GET /api/dividend-entries/by-user/:user_id
func (ct DividendEntriesController) GetListByUser(c *gin.Context) {
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

	requestedUserID, err := strconv.ParseInt(strings.TrimSpace(c.Param("user_id")), 10, 64)
	if err != nil || requestedUserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_USER_ID"})
		return
	}
	if requestedUserID != sessionUserID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	allowed, err := ct.G.CanDoAny(sessionUserID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	scope, err := ct.G.ScopeForAction(sessionUserID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	limit, offset, sortBy, fromDate, toDate, ok := parseDividendEntryListParams(c)
	if !ok {
		return
	}

	items, err := ct.DB.ListAccessibleDividendEntriesByUser(requestedUserID, scope.All, scope.Roles, limit, offset, sortBy, fromDate, toDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	count, err := ct.DB.CountAccessibleDividendEntriesByUser(requestedUserID, scope.All, scope.Roles, fromDate, toDate)
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

// GET /api/dividend-entries/by-depot/:depot_id
func (ct DividendEntriesController) GetListByDepot(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	depotID, ok := parseDepotIDParamForDividendEntries(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
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

	limit, offset, sortBy, fromDate, toDate, ok := parseDividendEntryListParams(c)
	if !ok {
		return
	}

	items, err := ct.DB.ListDividendEntriesByDepotID(depotID, limit, offset, sortBy, fromDate, toDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	count, err := ct.DB.CountDividendEntriesByDepotID(depotID, fromDate, toDate)
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

// GET /api/dividend-entries/by-security/:security_id
func (ct DividendEntriesController) GetListBySecurity(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	securityID, ok := parseSecurityIDParam(c)
	if !ok {
		return
	}

	limit, offset, sortBy, fromDate, toDate, ok := parseDividendEntryListParams(c)
	if !ok {
		return
	}

	items, err := ct.DB.ListDividendEntriesBySecurityID(securityID, limit, offset, sortBy, fromDate, toDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	count, err := ct.DB.CountDividendEntriesBySecurityID(securityID, fromDate, toDate)
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

// POST /api/dividend-entries/add
func (ct DividendEntriesController) PostAdd(c *gin.Context) {
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

	var req addDividendEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "entries:create", req.DepotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	entry, message := normalizeDividendEntryPayload(db.DividendEntry{
		DepotID:                                req.DepotID,
		SecurityID:                             req.SecurityID,
		PayDate:                                req.PayDate,
		ExDate:                                 req.ExDate,
		SecurityName:                           req.SecurityName,
		SecurityISIN:                           req.SecurityISIN,
		SecurityWKN:                            req.SecurityWKN,
		SecuritySymbol:                         req.SecuritySymbol,
		Quantity:                               req.Quantity,
		DividendPerUnitAmount:                  req.DividendPerUnitAmount,
		DividendPerUnitCurrency:                req.DividendPerUnitCurrency,
		FXRateLabel:                            req.FXRateLabel,
		FXRate:                                 req.FXRate,
		GrossAmount:                            req.GrossAmount,
		GrossCurrency:                          req.GrossCurrency,
		PayoutAmount:                           req.PayoutAmount,
		PayoutCurrency:                         req.PayoutCurrency,
		WithholdingTaxCountryCode:              req.WithholdingTaxCountryCode,
		WithholdingTaxPercent:                  req.WithholdingTaxPercent,
		WithholdingTaxAmount:                   req.WithholdingTaxAmount,
		WithholdingTaxCurrency:                 req.WithholdingTaxCurrency,
		WithholdingTaxAmountCredit:             req.WithholdingTaxAmountCredit,
		WithholdingTaxAmountCreditCurrency:     req.WithholdingTaxAmountCreditCurrency,
		WithholdingTaxAmountRefundable:         req.WithholdingTaxAmountRefundable,
		WithholdingTaxAmountRefundableCurrency: req.WithholdingTaxAmountRefundableCurrency,
		ForeignFeesAmount:                      req.ForeignFeesAmount,
		ForeignFeesCurrency:                    req.ForeignFeesCurrency,
		Note:                                   req.Note,
	})
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	entry = prepareCalculatedDividendFields(entry)

	if err := ct.DB.CreateDividendEntry(&entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionCreate,
		EntityType: db.EntityTypeDividendEntry,
		EntityID:   entry.ID,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    entry,
	})
}

// POST /api/dividend-entries/update/:id
func (ct DividendEntriesController) PostUpdate(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	id, ok := parseDividendEntryID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	existing, found, err := ct.DB.GetDividendEntryByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "DIVIDEND_ENTRY_NOT_FOUND"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "entries:edit", existing.DepotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	var req updateDividendEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	updated := existing
	if req.DepotID != nil {
		updated.DepotID = *req.DepotID
	}
	if req.SecurityID != nil {
		updated.SecurityID = *req.SecurityID
	}
	if req.PayDate != nil {
		updated.PayDate = *req.PayDate
	}
	if req.ExDate != nil {
		updated.ExDate = *req.ExDate
	}
	if req.SecurityName != nil {
		updated.SecurityName = *req.SecurityName
	}
	if req.SecurityISIN != nil {
		updated.SecurityISIN = *req.SecurityISIN
	}
	if req.SecurityWKN != nil {
		updated.SecurityWKN = *req.SecurityWKN
	}
	if req.SecuritySymbol != nil {
		updated.SecuritySymbol = *req.SecuritySymbol
	}
	if req.Quantity != nil {
		updated.Quantity = *req.Quantity
	}
	if req.DividendPerUnitAmount != nil {
		updated.DividendPerUnitAmount = *req.DividendPerUnitAmount
	}
	if req.DividendPerUnitCurrency != nil {
		updated.DividendPerUnitCurrency = *req.DividendPerUnitCurrency
	}
	if req.FXRateLabel != nil {
		updated.FXRateLabel = *req.FXRateLabel
	}
	if req.FXRate != nil {
		updated.FXRate = *req.FXRate
	}
	if req.GrossAmount != nil {
		updated.GrossAmount = *req.GrossAmount
	}
	if req.GrossCurrency != nil {
		updated.GrossCurrency = *req.GrossCurrency
	}
	if req.PayoutAmount != nil {
		updated.PayoutAmount = *req.PayoutAmount
	}
	if req.PayoutCurrency != nil {
		updated.PayoutCurrency = *req.PayoutCurrency
	}
	if req.WithholdingTaxCountryCode != nil {
		updated.WithholdingTaxCountryCode = *req.WithholdingTaxCountryCode
	}
	if req.WithholdingTaxPercent != nil {
		updated.WithholdingTaxPercent = *req.WithholdingTaxPercent
	}
	if req.WithholdingTaxAmount != nil {
		updated.WithholdingTaxAmount = *req.WithholdingTaxAmount
	}
	if req.WithholdingTaxCurrency != nil {
		updated.WithholdingTaxCurrency = *req.WithholdingTaxCurrency
	}
	if req.WithholdingTaxAmountCredit != nil {
		updated.WithholdingTaxAmountCredit = *req.WithholdingTaxAmountCredit
	}
	if req.WithholdingTaxAmountCreditCurrency != nil {
		updated.WithholdingTaxAmountCreditCurrency = *req.WithholdingTaxAmountCreditCurrency
	}
	if req.WithholdingTaxAmountRefundable != nil {
		updated.WithholdingTaxAmountRefundable = *req.WithholdingTaxAmountRefundable
	}
	if req.WithholdingTaxAmountRefundableCurrency != nil {
		updated.WithholdingTaxAmountRefundableCurrency = *req.WithholdingTaxAmountRefundableCurrency
	}
	if req.ForeignFeesAmount != nil {
		updated.ForeignFeesAmount = *req.ForeignFeesAmount
	}
	if req.ForeignFeesCurrency != nil {
		updated.ForeignFeesCurrency = *req.ForeignFeesCurrency
	}
	if req.Note != nil {
		updated.Note = *req.Note
	}

	updated, message := normalizeDividendEntryPayload(updated)
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	updated = prepareCalculatedDividendFields(updated)

	if err := ct.DB.UpdateDividendEntry(&updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionUpdate,
		EntityType: db.EntityTypeDividendEntry,
		EntityID:   id,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    updated,
	})
}

// POST /api/dividend-entries/delete/:id
func (ct DividendEntriesController) PostDelete(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	id, ok := parseDividendEntryID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	item, found, err := ct.DB.GetDividendEntryByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "DIVIDEND_ENTRY_NOT_FOUND"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "entries:delete", item.DepotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	if err := ct.DB.DeleteDividendEntry(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionDelete,
		EntityType: db.EntityTypeDividendEntry,
		EntityID:   id,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "DIVIDEND_ENTRY_DELETED",
	})
}
