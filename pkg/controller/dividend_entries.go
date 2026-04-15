package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/grrt"
	"github.com/geschke/schrevind/pkg/validate"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

type DividendEntriesController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
	G           *grrt.Grrt
}

// NewDividendEntriesController creates a controller for dividend-entry HTTP handlers.
func NewDividendEntriesController(database *db.DB, store sessions.Store, sessionName string, g *grrt.Grrt) *DividendEntriesController {
	return &DividendEntriesController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
		G:           g,
	}
}

// Options handles CORS preflight requests for dividend-entry routes.
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

const (
	errCurrencyRequired      = "ERR_CURRENCY_REQUIRED"
	errCurrencyInvalidFormat = "ERR_CURRENCY_INVALID_FORMAT"
	errCurrencyUnknown       = "ERR_CURRENCY_UNKNOWN"

	errFXRateLabelRequired        = "ERR_FX_RATE_LABEL_REQUIRED"
	errFXRateLabelInvalidFormat   = "ERR_FX_RATE_LABEL_INVALID_FORMAT"
	errFXRateLabelUnknownCurrency = "ERR_FX_RATE_LABEL_UNKNOWN_CURRENCY"
	errFXRatePairMismatch         = "ERR_FX_RATE_PAIR_MISMATCH"
	errFXRateZero                 = "ERR_FX_RATE_ZERO"

	errDepotNotFound       = "ERR_DEPOT_NOT_FOUND"
	errBaseCurrencyMissing = "ERR_BASE_CURRENCY_MISSING"
	errCalculationFailed   = "ERR_CALCULATION_FAILED"
)

type fieldErrors map[string]string

type decimalFieldRule struct {
	FieldName     string
	Required      bool
	AllowNegative bool
	Value         *string
}

type amountCurrencyPairRule struct {
	AmountFieldName   string
	CurrencyFieldName string
	Amount            *string
	Currency          *string
}

type baseAmount struct {
	Value decimal.Decimal
}

// ensureAuthorized verifies database/session setup and requires an authenticated session.
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

// currentSessionUserID returns the authenticated user ID from the current session.
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

// parseDividendEntryID parses and validates the dividend-entry ID path parameter.
func parseDividendEntryID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_DIVIDEND_ENTRY_ID"})
		return 0, false
	}
	return id, true
}

// parseDepotIDParamForDividendEntries parses and validates the depot_id path parameter.
func parseDepotIDParamForDividendEntries(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("depot_id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_DEPOT_ID"})
		return 0, false
	}
	return id, true
}

// parseSecurityIDParam parses and validates the security_id path parameter.
func parseSecurityIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("security_id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_SECURITY_ID"})
		return 0, false
	}
	return id, true
}

// parseDividendEntryListParams parses pagination, sorting, direction, and optional date filters.
func parseDividendEntryListParams(c *gin.Context) (int, int, string, string, string, string, bool) {
	limit := 20
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_LIMIT"})
			return 0, 0, "", "", "", "", false
		}
		limit = n
	}

	offset := 0
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_OFFSET"})
			return 0, 0, "", "", "", "", false
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
			return 0, 0, "", "", "", "", false
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
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_DIRECTION"})
			return 0, 0, "", "", "", "", false
		}
	}

	fromDate := strings.TrimSpace(c.Query("from"))
	toDate := strings.TrimSpace(c.Query("to"))
	if fromDate != "" && toDate != "" && fromDate > toDate {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_DATE_RANGE"})
		return 0, 0, "", "", "", "", false
	}

	return limit, offset, sortBy, direction, fromDate, toDate, true
}

// addRequiredStringFieldError adds a field error when a required string is empty after trimming.
func addRequiredStringFieldError(errors fieldErrors, fieldName, value, message string) {
	if strings.TrimSpace(value) == "" {
		errors[fieldName] = message
	}
}

// validateDecimalFields validates and normalizes decimal fields according to their field rules.
func validateDecimalFields(errors fieldErrors, rules []decimalFieldRule) {
	for _, rule := range rules {
		if rule.Value == nil {
			continue
		}

		value := strings.TrimSpace(*rule.Value)
		if value == "" && !rule.Required {
			*rule.Value = ""
			continue
		}

		normalized, err := validate.NormalizeDecimalString(value, rule.AllowNegative)
		if err != nil {
			errors[rule.FieldName] = err.Error()
			*rule.Value = value
			continue
		}

		*rule.Value = normalized
	}
}

// writeFieldErrors writes a validation-error response containing all collected field errors.
func writeFieldErrors(c *gin.Context, errors fieldErrors) {
	c.JSON(http.StatusBadRequest, gin.H{
		"success":     false,
		"message":     "VALIDATION_ERROR",
		"fieldErrors": errors,
	})
}

// isStrictCurrencyCode reports whether value is exactly a three-letter uppercase currency code.
func isStrictCurrencyCode(value string) bool {
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

// validateDividendEntryCurrencyPairs validates amount/currency pairs and known currency codes.
func validateDividendEntryCurrencyPairs(database *db.DB, entry *db.DividendEntry, errors fieldErrors) error {
	rules := []amountCurrencyPairRule{
		{AmountFieldName: "DividendPerUnitAmount", CurrencyFieldName: "DividendPerUnitCurrency", Amount: &entry.DividendPerUnitAmount, Currency: &entry.DividendPerUnitCurrency},
		{AmountFieldName: "GrossAmount", CurrencyFieldName: "GrossCurrency", Amount: &entry.GrossAmount, Currency: &entry.GrossCurrency},
		{AmountFieldName: "PayoutAmount", CurrencyFieldName: "PayoutCurrency", Amount: &entry.PayoutAmount, Currency: &entry.PayoutCurrency},
		{AmountFieldName: "WithholdingTaxAmount", CurrencyFieldName: "WithholdingTaxCurrency", Amount: &entry.WithholdingTaxAmount, Currency: &entry.WithholdingTaxCurrency},
		{AmountFieldName: "WithholdingTaxAmountCredit", CurrencyFieldName: "WithholdingTaxAmountCreditCurrency", Amount: &entry.WithholdingTaxAmountCredit, Currency: &entry.WithholdingTaxAmountCreditCurrency},
		{AmountFieldName: "WithholdingTaxAmountRefundable", CurrencyFieldName: "WithholdingTaxAmountRefundableCurrency", Amount: &entry.WithholdingTaxAmountRefundable, Currency: &entry.WithholdingTaxAmountRefundableCurrency},
		{AmountFieldName: "ForeignFeesAmount", CurrencyFieldName: "ForeignFeesCurrency", Amount: &entry.ForeignFeesAmount, Currency: &entry.ForeignFeesCurrency},
	}

	knownCurrencies := make(map[string]bool)
	for _, rule := range rules {
		amount := strings.TrimSpace(*rule.Amount)
		currency := strings.TrimSpace(*rule.Currency)
		*rule.Amount = amount
		*rule.Currency = currency

		if amount == "" && currency == "" {
			continue
		}
		if amount != "" && currency == "" {
			errors[rule.CurrencyFieldName] = errCurrencyRequired
			continue
		}
		if amount == "" && currency != "" {
			if _, exists := errors[rule.AmountFieldName]; !exists {
				errors[rule.AmountFieldName] = validate.ErrDecimalEmpty
			}
		}

		if currency == "" {
			continue
		}
		if !isStrictCurrencyCode(currency) {
			errors[rule.CurrencyFieldName] = errCurrencyInvalidFormat
			continue
		}
		if known, ok := knownCurrencies[currency]; ok {
			if !known {
				errors[rule.CurrencyFieldName] = errCurrencyUnknown
			}
			continue
		}

		item, err := database.GetCurrencyByCurrency(currency)
		if err != nil {
			return err
		}
		knownCurrencies[currency] = item != nil
		if item == nil {
			errors[rule.CurrencyFieldName] = errCurrencyUnknown
		}
	}

	return nil
}

// validateDividendEntryFXRate validates FXRateLabel/FXRate and normalizes the default rate.
func validateDividendEntryFXRate(database *db.DB, entry *db.DividendEntry, errors fieldErrors) error {
	entry.FXRateLabel = strings.TrimSpace(entry.FXRateLabel)
	entry.FXRate = strings.TrimSpace(entry.FXRate)

	if entry.FXRateLabel == "" {
		switch entry.FXRate {
		case "", "0", "1":
			entry.FXRate = "1"
		default:
			errors["FXRateLabel"] = errFXRateLabelRequired
		}
		return nil
	}

	normalizedFXRate, err := validate.NormalizeDecimalString(entry.FXRate, false)
	if err != nil {
		errors["FXRate"] = err.Error()
	} else {
		entry.FXRate = normalizedFXRate
	}

	left, right, ok := parseFXRateLabel(entry.FXRateLabel)
	if !ok {
		errors["FXRateLabel"] = errFXRateLabelInvalidFormat
		return nil
	}

	if known, err := isKnownCurrency(database, left); err != nil {
		return err
	} else if !known {
		errors["FXRateLabel"] = errFXRateLabelUnknownCurrency
		return nil
	}
	if known, err := isKnownCurrency(database, right); err != nil {
		return err
	} else if !known {
		errors["FXRateLabel"] = errFXRateLabelUnknownCurrency
	}

	return nil
}

// parseFXRateLabel splits labels of the form AAA/BBB into their two currency codes.
func parseFXRateLabel(label string) (string, string, bool) {
	if len(label) != 7 || label[3] != '/' {
		return "", "", false
	}

	left := label[:3]
	right := label[4:]
	if !isStrictCurrencyCode(left) || !isStrictCurrencyCode(right) {
		return "", "", false
	}

	return left, right, true
}

// isKnownCurrency reports whether a currency code exists in the currencies table.
func isKnownCurrency(database *db.DB, currency string) (bool, error) {
	item, err := database.GetCurrencyByCurrency(currency)
	if err != nil {
		return false, err
	}
	return item != nil, nil
}

// normalizeDividendEntryPayload trims strings, validates required fields, and normalizes decimals.
func normalizeDividendEntryPayload(entry db.DividendEntry) (db.DividendEntry, fieldErrors) {
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

	errors := fieldErrors{}

	if entry.DepotID <= 0 {
		errors["DepotID"] = "INVALID_DEPOT_ID"
	}
	if entry.SecurityID <= 0 {
		errors["SecurityID"] = "INVALID_SECURITY_ID"
	}

	addRequiredStringFieldError(errors, "PayDate", entry.PayDate, "MISSING_PAY_DATE")
	addRequiredStringFieldError(errors, "ExDate", entry.ExDate, "MISSING_EX_DATE")
	addRequiredStringFieldError(errors, "SecurityName", entry.SecurityName, "MISSING_SECURITY_NAME")
	addRequiredStringFieldError(errors, "SecurityISIN", entry.SecurityISIN, "MISSING_SECURITY_ISIN")
	addRequiredStringFieldError(errors, "DividendPerUnitCurrency", entry.DividendPerUnitCurrency, "MISSING_DIVIDEND_PER_UNIT_CURRENCY")
	addRequiredStringFieldError(errors, "GrossCurrency", entry.GrossCurrency, "MISSING_GROSS_CURRENCY")
	addRequiredStringFieldError(errors, "PayoutCurrency", entry.PayoutCurrency, "MISSING_PAYOUT_CURRENCY")

	validateDecimalFields(errors, []decimalFieldRule{
		{FieldName: "Quantity", Required: true, Value: &entry.Quantity},
		{FieldName: "DividendPerUnitAmount", Required: true, Value: &entry.DividendPerUnitAmount},
		{FieldName: "GrossAmount", Required: true, Value: &entry.GrossAmount},
		{FieldName: "PayoutAmount", Required: true, Value: &entry.PayoutAmount},
		{FieldName: "WithholdingTaxPercent", Value: &entry.WithholdingTaxPercent},
		{FieldName: "WithholdingTaxAmount", Value: &entry.WithholdingTaxAmount},
		{FieldName: "WithholdingTaxAmountCredit", Value: &entry.WithholdingTaxAmountCredit},
		{FieldName: "WithholdingTaxAmountRefundable", Value: &entry.WithholdingTaxAmountRefundable},
		{FieldName: "ForeignFeesAmount", Value: &entry.ForeignFeesAmount},
	})

	return entry, errors
}

// prepareCalculatedDividendFields calculates backend-owned base-currency amount fields.
func prepareCalculatedDividendFields(database *db.DB, entry *db.DividendEntry, errors fieldErrors) error {
	depot, found, err := database.GetDepotByID(entry.DepotID)
	if err != nil {
		return err
	}
	if !found {
		errors["DepotID"] = errDepotNotFound
		return nil
	}

	baseCurrency := strings.TrimSpace(depot.BaseCurrency)
	if baseCurrency == "" {
		errors["DepotID"] = errBaseCurrencyMissing
		return nil
	}
	if !isStrictCurrencyCode(baseCurrency) {
		errors["DepotID"] = errCurrencyInvalidFormat
		return nil
	}
	baseCurrencyItem, err := database.GetCurrencyByCurrency(baseCurrency)
	if err != nil {
		return err
	}
	if baseCurrencyItem == nil {
		errors["DepotID"] = errCurrencyUnknown
		return nil
	}
	decimalPlaces := int32(baseCurrencyItem.DecimalPlaces)

	grossBase, ok := convertAmountToBase(entry.GrossAmount, entry.GrossCurrency, "GrossAmount", baseCurrency, entry.FXRateLabel, entry.FXRate, errors)
	if !ok {
		return nil
	}
	entry.CalcGrossAmountBase = formatCalculatedDecimal(grossBase.Value, decimalPlaces)

	if strings.TrimSpace(entry.WithholdingTaxAmount) == "" {
		entry.CalcAfterWithholdingAmountBase = entry.CalcGrossAmountBase
		return nil
	}

	taxBase, ok := convertAmountToBase(entry.WithholdingTaxAmount, entry.WithholdingTaxCurrency, "WithholdingTaxAmount", baseCurrency, entry.FXRateLabel, entry.FXRate, errors)
	if !ok {
		return nil
	}

	entry.CalcAfterWithholdingAmountBase = formatCalculatedDecimal(grossBase.Value.Sub(taxBase.Value), decimalPlaces)
	return nil
}

// convertAmountToBase converts an amount into the depot base currency using the validated FX pair.
func convertAmountToBase(amount, currency, amountFieldName, baseCurrency, fxRateLabel, fxRate string, errors fieldErrors) (baseAmount, bool) {
	amountDecimal, err := decimal.NewFromString(amount)
	if err != nil {
		errors[amountFieldName] = errCalculationFailed
		return baseAmount{}, false
	}

	if currency == baseCurrency {
		return baseAmount{
			Value: amountDecimal,
		}, true
	}

	if strings.TrimSpace(fxRateLabel) == "" {
		errors["FXRateLabel"] = errFXRateLabelRequired
		return baseAmount{}, false
	}

	labelLeft, labelRight, ok := parseFXRateLabel(fxRateLabel)
	if !ok {
		errors["FXRateLabel"] = errFXRateLabelInvalidFormat
		return baseAmount{}, false
	}

	rateDecimal, err := decimal.NewFromString(fxRate)
	if err != nil {
		errors["FXRate"] = errCalculationFailed
		return baseAmount{}, false
	}
	if rateDecimal.IsZero() {
		errors["FXRate"] = errFXRateZero
		return baseAmount{}, false
	}

	switch {
	case labelLeft == currency && labelRight == baseCurrency:
		converted := amountDecimal.Mul(rateDecimal)
		return baseAmount{Value: converted}, true
	case labelLeft == baseCurrency && labelRight == currency:
		converted := amountDecimal.Div(rateDecimal)
		return baseAmount{Value: converted}, true
	default:
		errors["FXRateLabel"] = errFXRatePairMismatch
		return baseAmount{}, false
	}
}

// formatCalculatedDecimal rounds calculated values to the currency precision and formats them.
func formatCalculatedDecimal(value decimal.Decimal, decimalPlaces int32) string {
	return value.Round(decimalPlaces).StringFixed(decimalPlaces)
}

// GetByID handles GET /api/dividend-entries/:id and returns one authorized entry.
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

// GetListByUser handles GET /api/dividend-entries/by-user/:user_id for the current user.
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

	limit, offset, sortBy, direction, fromDate, toDate, ok := parseDividendEntryListParams(c)
	if !ok {
		return
	}

	items, err := ct.DB.ListAccessibleDividendEntriesByUser(requestedUserID, scope.All, scope.Roles, limit, offset, sortBy, direction, fromDate, toDate)
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

// GetListByDepot handles GET /api/dividend-entries/by-depot/:depot_id for an authorized depot.
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

	limit, offset, sortBy, direction, fromDate, toDate, ok := parseDividendEntryListParams(c)
	if !ok {
		return
	}

	items, err := ct.DB.ListDividendEntriesByDepotID(depotID, limit, offset, sortBy, direction, fromDate, toDate)
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

// GetListBySecurity handles GET /api/dividend-entries/by-security/:security_id.
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

	limit, offset, sortBy, direction, fromDate, toDate, ok := parseDividendEntryListParams(c)
	if !ok {
		return
	}

	items, err := ct.DB.ListDividendEntriesBySecurityID(securityID, limit, offset, sortBy, direction, fromDate, toDate)
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

// PostAdd handles POST /api/dividend-entries/add and creates a validated dividend entry.
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

	entry, fieldErrors := normalizeDividendEntryPayload(db.DividendEntry{
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
	if err := validateDividendEntryCurrencyPairs(ct.DB, &entry, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if err := validateDividendEntryFXRate(ct.DB, &entry, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if len(fieldErrors) > 0 {
		writeFieldErrors(c, fieldErrors)
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeDepot, "entries:create", entry.DepotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_DEPOT"})
		return
	}

	if err := prepareCalculatedDividendFields(ct.DB, &entry, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if len(fieldErrors) > 0 {
		writeFieldErrors(c, fieldErrors)
		return
	}

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

// PostUpdate handles POST /api/dividend-entries/update/:id and updates a validated entry.
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

	updated, fieldErrors := normalizeDividendEntryPayload(updated)
	if err := validateDividendEntryCurrencyPairs(ct.DB, &updated, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if err := validateDividendEntryFXRate(ct.DB, &updated, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if len(fieldErrors) > 0 {
		writeFieldErrors(c, fieldErrors)
		return
	}

	if err := prepareCalculatedDividendFields(ct.DB, &updated, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if len(fieldErrors) > 0 {
		writeFieldErrors(c, fieldErrors)
		return
	}

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

// PostDelete handles POST /api/dividend-entries/delete/:id for an authorized entry.
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
