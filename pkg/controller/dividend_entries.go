package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	displayformat "github.com/geschke/schrevind/pkg/format"
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
	ContextGroupID int64  `json:"ContextGroupID"`
	DepotID        int64  `json:"DepotID"`
	SecurityID     int64  `json:"SecurityID"`
	PayDate        string `json:"PayDate"`
	ExDate         string `json:"ExDate"`

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

	InlandTaxAmount   string               `json:"InlandTaxAmount"`
	InlandTaxCurrency string               `json:"InlandTaxCurrency"`
	InlandTaxDetails  []db.InlandTaxDetail `json:"InlandTaxDetails"`

	ForeignFeesAmount   string `json:"ForeignFeesAmount"`
	ForeignFeesCurrency string `json:"ForeignFeesCurrency"`

	Note string `json:"Note"`
}

type updateDividendEntryRequest struct {
	ContextGroupID *int64  `json:"ContextGroupID"`
	DepotID        *int64  `json:"DepotID"`
	SecurityID     *int64  `json:"SecurityID"`
	PayDate        *string `json:"PayDate"`
	ExDate         *string `json:"ExDate"`

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

	InlandTaxAmount   *string               `json:"InlandTaxAmount"`
	InlandTaxCurrency *string               `json:"InlandTaxCurrency"`
	InlandTaxDetails  *[]db.InlandTaxDetail `json:"InlandTaxDetails"`

	ForeignFeesAmount   *string `json:"ForeignFeesAmount"`
	ForeignFeesCurrency *string `json:"ForeignFeesCurrency"`

	Note *string `json:"Note"`
}

func (req updateDividendEntryRequest) requiresInlandTaxRecalculation() bool {
	return req.GrossAmount != nil ||
		req.PayoutAmount != nil ||
		req.WithholdingTaxAmount != nil ||
		req.InlandTaxDetails != nil
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

type withholdingTaxRefundCalculation struct {
	Amount        string
	Currency      string
	Default       db.WithholdingTaxDefault
	RefundPercent string
	Source        string
	Capped        bool
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

func (ct DividendEntriesController) currentSessionUserLocale(c *gin.Context) (string, bool, error) {
	userID, ok := ct.currentSessionUserID(c)
	if !ok {
		return "", false, nil
	}

	u, found, err := ct.DB.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}

	return normalizeUserLocaleForController(u.Locale), true, nil
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

// parseDividendEntryContextGroupID parses and validates the context_group_id query parameter.
func parseDividendEntryContextGroupID(c *gin.Context) (int64, bool) {
	groupID, err := strconv.ParseInt(strings.TrimSpace(c.Query("context_group_id")), 10, 64)
	if err != nil || groupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return 0, false
	}
	return groupID, true
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
func validateDividendEntryCurrencyPairs(database *db.DB, groupID int64, entry *db.DividendEntry, errors fieldErrors) error {
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

		item, err := database.GetCurrencyByCurrencyAndGroupID(currency, groupID)
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

// validateInlandTaxCurrency validates the backend-owned inland tax amount/currency pair.
func validateInlandTaxCurrency(database *db.DB, groupID int64, entry *db.DividendEntry, errors fieldErrors) error {
	entry.InlandTaxAmount = strings.TrimSpace(entry.InlandTaxAmount)
	entry.InlandTaxCurrency = strings.TrimSpace(entry.InlandTaxCurrency)

	if entry.InlandTaxAmount == "" && entry.InlandTaxCurrency == "" {
		return nil
	}
	if entry.InlandTaxAmount != "" && entry.InlandTaxCurrency == "" {
		errors["InlandTaxCurrency"] = errCurrencyRequired
		return nil
	}
	if entry.InlandTaxCurrency == "" {
		return nil
	}
	if !isStrictCurrencyCode(entry.InlandTaxCurrency) {
		errors["InlandTaxCurrency"] = errCurrencyInvalidFormat
		return nil
	}

	known, err := isKnownCurrency(database, groupID, entry.InlandTaxCurrency)
	if err != nil {
		return err
	}
	if !known {
		errors["InlandTaxCurrency"] = errCurrencyUnknown
	}

	return nil
}

// validateInlandTaxDetails validates and normalizes optional inland tax snapshot rows.
func validateInlandTaxDetails(database *db.DB, groupID int64, entry *db.DividendEntry, errors fieldErrors) error {
	knownCurrencies := make(map[string]bool)

	for i := range entry.InlandTaxDetails {
		detail := &entry.InlandTaxDetails[i]
		detail.Code = strings.TrimSpace(detail.Code)
		detail.Label = strings.TrimSpace(detail.Label)
		detail.Amount = strings.TrimSpace(detail.Amount)
		detail.Currency = strings.TrimSpace(detail.Currency)

		amountField := "InlandTaxDetails[" + strconv.Itoa(i) + "].Amount"
		currencyField := "InlandTaxDetails[" + strconv.Itoa(i) + "].Currency"

		if detail.Amount != "" {
			normalized, err := validate.NormalizeDecimalString(detail.Amount, false)
			if err != nil {
				errors[amountField] = err.Error()
			} else {
				detail.Amount = normalized
			}
		}

		if detail.Amount != "" && detail.Currency == "" {
			errors[currencyField] = errCurrencyRequired
			continue
		}
		if detail.Currency == "" {
			continue
		}
		if !isStrictCurrencyCode(detail.Currency) {
			errors[currencyField] = errCurrencyInvalidFormat
			continue
		}
		if known, ok := knownCurrencies[detail.Currency]; ok {
			if !known {
				errors[currencyField] = errCurrencyUnknown
			}
			continue
		}

		item, err := database.GetCurrencyByCurrencyAndGroupID(detail.Currency, groupID)
		if err != nil {
			return err
		}
		knownCurrencies[detail.Currency] = item != nil
		if item == nil {
			errors[currencyField] = errCurrencyUnknown
		}
	}

	return nil
}

// validateDividendEntryFXRate validates FXRateLabel/FXRate and normalizes the default rate.
func validateDividendEntryFXRate(database *db.DB, groupID int64, entry *db.DividendEntry, errors fieldErrors) error {
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

	if known, err := isKnownCurrency(database, groupID, left); err != nil {
		return err
	} else if !known {
		errors["FXRateLabel"] = errFXRateLabelUnknownCurrency
		return nil
	}
	if known, err := isKnownCurrency(database, groupID, right); err != nil {
		return err
	} else if !known {
		errors["FXRateLabel"] = errFXRateLabelUnknownCurrency
	}

	return nil
}

// validateDividendEntrySecurity validates that the selected security exists in the active group context.
func validateDividendEntrySecurity(database *db.DB, groupID int64, entry *db.DividendEntry, errors fieldErrors) error {
	if entry.SecurityID <= 0 {
		return nil
	}

	item, err := database.GetSecurityByIDAndGroupID(entry.SecurityID, groupID)
	if err != nil {
		return err
	}
	if item == nil {
		errors["SecurityID"] = "INVALID_SECURITY_ID"
	}
	return nil
}

func decimalValueOrZero(value string) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(value)
}

// prepareInlandTaxFields calculates inland tax amount when it was not explicitly submitted.
func prepareInlandTaxFields(entry *db.DividendEntry, errors fieldErrors) {
	entry.InlandTaxAmount = strings.TrimSpace(entry.InlandTaxAmount)
	entry.InlandTaxCurrency = strings.TrimSpace(entry.InlandTaxCurrency)

	if entry.InlandTaxAmount != "" {
		return
	}

	if len(entry.InlandTaxDetails) > 0 {
		sum := decimal.Zero
		for i := range entry.InlandTaxDetails {
			amount, err := decimalValueOrZero(entry.InlandTaxDetails[i].Amount)
			if err != nil {
				errors["InlandTaxDetails["+strconv.Itoa(i)+"].Amount"] = err.Error()
				continue
			}
			sum = sum.Add(amount)
		}
		entry.InlandTaxAmount = sum.String()
	} else {
		gross, err := decimalValueOrZero(entry.GrossAmount)
		if err != nil {
			errors["GrossAmount"] = err.Error()
			return
		}
		withholding, err := decimalValueOrZero(entry.WithholdingTaxAmount)
		if err != nil {
			errors["WithholdingTaxAmount"] = err.Error()
			return
		}
		payout, err := decimalValueOrZero(entry.PayoutAmount)
		if err != nil {
			errors["PayoutAmount"] = err.Error()
			return
		}
		entry.InlandTaxAmount = gross.Sub(withholding).Sub(payout).String()
	}

	if entry.InlandTaxCurrency == "" {
		entry.InlandTaxCurrency = entry.PayoutCurrency
	}
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
func isKnownCurrency(database *db.DB, groupID int64, currency string) (bool, error) {
	item, err := database.GetCurrencyByCurrencyAndGroupID(currency, groupID)
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
	entry.InlandTaxAmount = strings.TrimSpace(entry.InlandTaxAmount)
	entry.InlandTaxCurrency = strings.TrimSpace(entry.InlandTaxCurrency)
	for i := range entry.InlandTaxDetails {
		entry.InlandTaxDetails[i].Code = strings.TrimSpace(entry.InlandTaxDetails[i].Code)
		entry.InlandTaxDetails[i].Label = strings.TrimSpace(entry.InlandTaxDetails[i].Label)
		entry.InlandTaxDetails[i].Amount = strings.TrimSpace(entry.InlandTaxDetails[i].Amount)
		entry.InlandTaxDetails[i].Currency = strings.TrimSpace(entry.InlandTaxDetails[i].Currency)
	}
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
		{FieldName: "InlandTaxAmount", Value: &entry.InlandTaxAmount},
		{FieldName: "ForeignFeesAmount", Value: &entry.ForeignFeesAmount},
	})

	return entry, errors
}

// prepareCalculatedDividendFields calculates backend-owned base-currency amount fields.
func prepareCalculatedDividendFields(database *db.DB, groupID int64, entry *db.DividendEntry, errors fieldErrors) error {
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
	baseCurrencyItem, err := database.GetCurrencyByCurrencyAndGroupID(baseCurrency, groupID)
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

func validateWithholdingTaxRefundCalculationInput(database *db.DB, groupID int64, entry *db.DividendEntry, errors fieldErrors) error {
	entry.GrossAmount = strings.TrimSpace(entry.GrossAmount)
	entry.GrossCurrency = strings.TrimSpace(entry.GrossCurrency)
	entry.WithholdingTaxCountryCode = strings.ToUpper(strings.TrimSpace(entry.WithholdingTaxCountryCode))
	entry.WithholdingTaxAmount = strings.TrimSpace(entry.WithholdingTaxAmount)
	entry.WithholdingTaxCurrency = strings.TrimSpace(entry.WithholdingTaxCurrency)
	entry.FXRateLabel = strings.TrimSpace(entry.FXRateLabel)
	entry.FXRate = strings.TrimSpace(entry.FXRate)

	if groupID <= 0 {
		errors["ContextGroupID"] = "INVALID_GROUP_ID"
	}
	if entry.DepotID <= 0 {
		errors["DepotID"] = "INVALID_DEPOT_ID"
	}
	if entry.GrossAmount == "" {
		errors["GrossAmount"] = "MISSING_GROSS_AMOUNT"
	} else if normalized, err := validate.NormalizeDecimalString(entry.GrossAmount, false); err != nil {
		errors["GrossAmount"] = err.Error()
	} else {
		entry.GrossAmount = normalized
	}
	if entry.GrossCurrency == "" {
		errors["GrossCurrency"] = "MISSING_GROSS_CURRENCY"
	} else if !isStrictCurrencyCode(entry.GrossCurrency) {
		errors["GrossCurrency"] = errCurrencyInvalidFormat
	} else if groupID > 0 {
		known, err := isKnownCurrency(database, groupID, entry.GrossCurrency)
		if err != nil {
			return err
		}
		if !known {
			errors["GrossCurrency"] = errCurrencyUnknown
		}
	}
	if entry.WithholdingTaxCountryCode == "" {
		errors["WithholdingTaxCountryCode"] = "MISSING_WITHHOLDING_TAX_COUNTRY_CODE"
	}
	if entry.WithholdingTaxAmount == "" {
		errors["WithholdingTaxAmount"] = "MISSING_WITHHOLDING_TAX_AMOUNT"
	} else if normalized, err := validate.NormalizeDecimalString(entry.WithholdingTaxAmount, false); err != nil {
		errors["WithholdingTaxAmount"] = err.Error()
	} else {
		entry.WithholdingTaxAmount = normalized
	}
	if entry.WithholdingTaxCurrency == "" {
		errors["WithholdingTaxCurrency"] = "MISSING_WITHHOLDING_TAX_CURRENCY"
	} else if !isStrictCurrencyCode(entry.WithholdingTaxCurrency) {
		errors["WithholdingTaxCurrency"] = errCurrencyInvalidFormat
	} else if groupID > 0 {
		known, err := isKnownCurrency(database, groupID, entry.WithholdingTaxCurrency)
		if err != nil {
			return err
		}
		if !known {
			errors["WithholdingTaxCurrency"] = errCurrencyUnknown
		}
	}

	return nil
}

func calculateWithholdingTaxRefund(database *db.DB, groupID int64, entry *db.DividendEntry, errors fieldErrors) (withholdingTaxRefundCalculation, error) {
	depot, found, err := database.GetDepotByID(entry.DepotID)
	if err != nil {
		return withholdingTaxRefundCalculation{}, err
	}
	if !found {
		errors["DepotID"] = errDepotNotFound
		return withholdingTaxRefundCalculation{}, nil
	}

	baseCurrency := strings.TrimSpace(depot.BaseCurrency)
	if baseCurrency == "" {
		errors["DepotID"] = errBaseCurrencyMissing
		return withholdingTaxRefundCalculation{}, nil
	}
	if !isStrictCurrencyCode(baseCurrency) {
		errors["DepotID"] = errCurrencyInvalidFormat
		return withholdingTaxRefundCalculation{}, nil
	}

	baseCurrencyItem, err := database.GetCurrencyByCurrencyAndGroupID(baseCurrency, groupID)
	if err != nil {
		return withholdingTaxRefundCalculation{}, err
	}
	if baseCurrencyItem == nil {
		errors["DepotID"] = errCurrencyUnknown
		return withholdingTaxRefundCalculation{}, nil
	}

	if err := validateDividendEntryFXRate(database, groupID, entry, errors); err != nil {
		return withholdingTaxRefundCalculation{}, err
	}
	if len(errors) > 0 {
		return withholdingTaxRefundCalculation{}, nil
	}

	defaults, err := database.GetEffectiveWithholdingTaxDefault(groupID, entry.DepotID, entry.WithholdingTaxCountryCode)
	if err != nil {
		return withholdingTaxRefundCalculation{}, err
	}
	if defaults == nil {
		errors["WithholdingTaxCountryCode"] = "WITHHOLDING_TAX_DEFAULT_NOT_FOUND"
		return withholdingTaxRefundCalculation{}, nil
	}

	withholdingPercentRaw := strings.TrimSpace(defaults.WithholdingTaxPercentDefault)
	if withholdingPercentRaw == "" {
		errors["WithholdingTaxPercentDefault"] = "WITHHOLDING_TAX_PERCENT_MISSING"
		return withholdingTaxRefundCalculation{}, nil
	}
	withholdingPercentRaw, err = validate.NormalizeDecimalString(withholdingPercentRaw, false)
	if err != nil {
		errors["WithholdingTaxPercentDefault"] = "INVALID_WITHHOLDING_TAX_PERCENT"
		return withholdingTaxRefundCalculation{}, nil
	}
	creditPercentRaw := strings.TrimSpace(defaults.WithholdingTaxPercentCreditDefault)
	if creditPercentRaw == "" {
		errors["WithholdingTaxPercentCreditDefault"] = "WITHHOLDING_TAX_CREDIT_PERCENT_MISSING"
		return withholdingTaxRefundCalculation{}, nil
	}
	creditPercentRaw, err = validate.NormalizeDecimalString(creditPercentRaw, false)
	if err != nil {
		errors["WithholdingTaxPercentCreditDefault"] = "INVALID_WITHHOLDING_TAX_CREDIT_PERCENT"
		return withholdingTaxRefundCalculation{}, nil
	}

	withholdingPercent, err := decimal.NewFromString(withholdingPercentRaw)
	if err != nil {
		errors["WithholdingTaxPercentDefault"] = "INVALID_WITHHOLDING_TAX_PERCENT"
		return withholdingTaxRefundCalculation{}, nil
	}
	creditPercent, err := decimal.NewFromString(creditPercentRaw)
	if err != nil {
		errors["WithholdingTaxPercentCreditDefault"] = "INVALID_WITHHOLDING_TAX_CREDIT_PERCENT"
		return withholdingTaxRefundCalculation{}, nil
	}

	refundPercent := withholdingPercent.Sub(creditPercent)
	if refundPercent.IsNegative() {
		refundPercent = decimal.Zero
	}

	grossBase, ok := convertAmountToBase(entry.GrossAmount, entry.GrossCurrency, "GrossAmount", baseCurrency, entry.FXRateLabel, entry.FXRate, errors)
	if !ok {
		return withholdingTaxRefundCalculation{}, nil
	}
	withholdingBase, ok := convertAmountToBase(entry.WithholdingTaxAmount, entry.WithholdingTaxCurrency, "WithholdingTaxAmount", baseCurrency, entry.FXRateLabel, entry.FXRate, errors)
	if !ok {
		return withholdingTaxRefundCalculation{}, nil
	}

	refundAmount := grossBase.Value.Mul(refundPercent).Div(decimal.NewFromInt(100))
	capped := false
	if refundAmount.GreaterThan(withholdingBase.Value) {
		refundAmount = withholdingBase.Value
		capped = true
	}
	if refundAmount.IsNegative() {
		refundAmount = decimal.Zero
	}

	source := "group"
	if defaults.DepotID > 0 {
		source = "depot"
	}

	return withholdingTaxRefundCalculation{
		Amount:        formatCalculatedDecimal(refundAmount, int32(baseCurrencyItem.DecimalPlaces)),
		Currency:      baseCurrency,
		Default:       *defaults,
		RefundPercent: refundPercent.String(),
		Source:        source,
		Capped:        capped,
	}, nil
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

type dividendEntryCurrencyFormatter struct {
	database          *db.DB
	securityGroups    map[int64]int64
	currencyPrecision map[string]int32
}

func newDividendEntryCurrencyFormatter(database *db.DB) *dividendEntryCurrencyFormatter {
	return &dividendEntryCurrencyFormatter{
		database:          database,
		securityGroups:    make(map[int64]int64),
		currencyPrecision: make(map[string]int32),
	}
}

func (f *dividendEntryCurrencyFormatter) groupIDForEntry(entry db.DividendEntry) (int64, error) {
	if entry.SecurityID <= 0 {
		return 0, nil
	}
	if groupID, ok := f.securityGroups[entry.SecurityID]; ok {
		return groupID, nil
	}

	groupID, found, err := f.database.GetSecurityGroupIDByID(entry.SecurityID)
	if err != nil {
		return 0, err
	}
	if !found {
		f.securityGroups[entry.SecurityID] = 0
		return 0, nil
	}

	f.securityGroups[entry.SecurityID] = groupID
	return groupID, nil
}

func (f *dividendEntryCurrencyFormatter) decimalPlaces(groupID int64, currency string) (int32, bool, error) {
	currency = strings.TrimSpace(currency)
	if groupID <= 0 || currency == "" {
		return 0, false, nil
	}

	cacheKey := strconv.FormatInt(groupID, 10) + ":" + currency
	if decimalPlaces, ok := f.currencyPrecision[cacheKey]; ok {
		return decimalPlaces, true, nil
	}

	item, err := f.database.GetCurrencyByCurrencyAndGroupID(currency, groupID)
	if err != nil {
		return 0, false, err
	}
	if item == nil {
		return 0, false, nil
	}

	decimalPlaces := int32(item.DecimalPlaces)
	f.currencyPrecision[cacheKey] = decimalPlaces
	return decimalPlaces, true, nil
}

func (f *dividendEntryCurrencyFormatter) formatAmount(value, currency, locale string, groupID int64) (string, error) {
	decimalPlaces, ok, err := f.decimalPlaces(groupID, currency)
	if err != nil {
		return "", err
	}
	if !ok {
		return displayformat.DecimalForLocale(value, locale), nil
	}
	return displayformat.DecimalForLocaleFixed(value, locale, decimalPlaces), nil
}

func formatDividendEntryForLocale(entry db.DividendEntry, locale string, formatter *dividendEntryCurrencyFormatter) (db.DividendEntry, error) {
	entry.Quantity = displayformat.DecimalForLocale(entry.Quantity, locale)
	entry.DividendPerUnitAmount = displayformat.DecimalForLocale(entry.DividendPerUnitAmount, locale)
	entry.FXRate = displayformat.DecimalForLocale(entry.FXRate, locale)
	entry.GrossAmount = displayformat.DecimalForLocale(entry.GrossAmount, locale)
	entry.PayoutAmount = displayformat.DecimalForLocale(entry.PayoutAmount, locale)
	entry.WithholdingTaxPercent = displayformat.DecimalForLocale(entry.WithholdingTaxPercent, locale)
	entry.WithholdingTaxAmount = displayformat.DecimalForLocale(entry.WithholdingTaxAmount, locale)
	entry.WithholdingTaxAmountCredit = displayformat.DecimalForLocale(entry.WithholdingTaxAmountCredit, locale)
	entry.WithholdingTaxAmountRefundable = displayformat.DecimalForLocale(entry.WithholdingTaxAmountRefundable, locale)

	groupID := int64(0)
	if formatter != nil {
		var err error
		groupID, err = formatter.groupIDForEntry(entry)
		if err != nil {
			return db.DividendEntry{}, err
		}
	}
	if formatter != nil {
		formatted, err := formatter.formatAmount(entry.InlandTaxAmount, entry.InlandTaxCurrency, locale, groupID)
		if err != nil {
			return db.DividendEntry{}, err
		}
		entry.InlandTaxAmount = formatted
		for i := range entry.InlandTaxDetails {
			formatted, err := formatter.formatAmount(entry.InlandTaxDetails[i].Amount, entry.InlandTaxDetails[i].Currency, locale, groupID)
			if err != nil {
				return db.DividendEntry{}, err
			}
			entry.InlandTaxDetails[i].Amount = formatted
		}
	} else {
		entry.InlandTaxAmount = displayformat.DecimalForLocale(entry.InlandTaxAmount, locale)
		for i := range entry.InlandTaxDetails {
			entry.InlandTaxDetails[i].Amount = displayformat.DecimalForLocale(entry.InlandTaxDetails[i].Amount, locale)
		}
	}
	entry.ForeignFeesAmount = displayformat.DecimalForLocale(entry.ForeignFeesAmount, locale)
	entry.CalcGrossAmountBase = displayformat.DecimalForLocale(entry.CalcGrossAmountBase, locale)
	entry.CalcAfterWithholdingAmountBase = displayformat.DecimalForLocale(entry.CalcAfterWithholdingAmountBase, locale)
	return entry, nil
}

func formatDividendEntriesForLocale(items []db.DividendEntry, locale string, formatter *dividendEntryCurrencyFormatter) ([]db.DividendEntry, error) {
	for i := range items {
		formatted, err := formatDividendEntryForLocale(items[i], locale, formatter)
		if err != nil {
			return nil, err
		}
		items[i] = formatted
	}
	return items, nil
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

	locale, ok, err := ct.currentSessionUserLocale(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}
	formatter := newDividendEntryCurrencyFormatter(ct.DB)
	item, err = formatDividendEntryForLocale(item, locale, formatter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
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

	locale, ok, err := ct.currentSessionUserLocale(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}
	formatter := newDividendEntryCurrencyFormatter(ct.DB)
	items, err = formatDividendEntriesForLocale(items, locale, formatter)
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

	locale, ok, err := ct.currentSessionUserLocale(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}
	formatter := newDividendEntryCurrencyFormatter(ct.DB)
	items, err = formatDividendEntriesForLocale(items, locale, formatter)
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

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	securityID, ok := parseSecurityIDParam(c)
	if !ok {
		return
	}
	groupID, ok := parseDividendEntryContextGroupID(c)
	if !ok {
		return
	}

	inGroup, err := ct.DB.IsUserInGroup(groupID, sessionUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !inGroup {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	security, err := ct.DB.GetSecurityByIDAndGroupID(securityID, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if security == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "SECURITY_NOT_FOUND"})
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

	items, err := ct.DB.ListAccessibleDividendEntriesBySecurityID(sessionUserID, scope.All, scope.Roles, securityID, limit, offset, sortBy, direction, fromDate, toDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	count, err := ct.DB.CountAccessibleDividendEntriesBySecurityID(sessionUserID, scope.All, scope.Roles, securityID, fromDate, toDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	locale, ok, err := ct.currentSessionUserLocale(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}
	formatter := newDividendEntryCurrencyFormatter(ct.DB)
	items, err = formatDividendEntriesForLocale(items, locale, formatter)
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

// PostCalculateWithholdingTaxRefund calculates a refundable withholding tax amount without storing data.
func (ct DividendEntriesController) PostCalculateWithholdingTaxRefund(c *gin.Context) {
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

	entry := db.DividendEntry{
		DepotID:                   req.DepotID,
		GrossAmount:               req.GrossAmount,
		GrossCurrency:             req.GrossCurrency,
		WithholdingTaxCountryCode: req.WithholdingTaxCountryCode,
		WithholdingTaxAmount:      req.WithholdingTaxAmount,
		WithholdingTaxCurrency:    req.WithholdingTaxCurrency,
		FXRateLabel:               req.FXRateLabel,
		FXRate:                    req.FXRate,
	}
	fieldErrors := fieldErrors{}

	if err := validateWithholdingTaxRefundCalculationInput(ct.DB, req.ContextGroupID, &entry, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if len(fieldErrors) > 0 {
		writeFieldErrors(c, fieldErrors)
		return
	}

	inGroup, err := ct.DB.IsUserInGroup(req.ContextGroupID, sessionUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !inGroup {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
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

	result, err := calculateWithholdingTaxRefund(ct.DB, req.ContextGroupID, &entry, fieldErrors)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if len(fieldErrors) > 0 {
		writeFieldErrors(c, fieldErrors)
		return
	}

	locale, ok, err := ct.currentSessionUserLocale(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	message := "WITHHOLDING_TAX_REFUND_CALCULATED"
	if result.Capped {
		message = "WITHHOLDING_TAX_REFUND_CALCULATED_CAPPED"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data": gin.H{
			"WithholdingTaxAmountRefundable":         displayformat.DecimalForLocale(result.Amount, locale),
			"WithholdingTaxAmountRefundableCurrency": result.Currency,
			"WithholdingTaxPercentDefault":           result.Default.WithholdingTaxPercentDefault,
			"WithholdingTaxPercentCreditDefault":     result.Default.WithholdingTaxPercentCreditDefault,
			"WithholdingTaxRefundPercent":            result.RefundPercent,
			"Source":                                 result.Source,
			"Capped":                                 result.Capped,
		},
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
		InlandTaxAmount:                        req.InlandTaxAmount,
		InlandTaxCurrency:                      req.InlandTaxCurrency,
		InlandTaxDetails:                       req.InlandTaxDetails,
		ForeignFeesAmount:                      req.ForeignFeesAmount,
		ForeignFeesCurrency:                    req.ForeignFeesCurrency,
		Note:                                   req.Note,
	})
	if req.ContextGroupID <= 0 {
		fieldErrors["ContextGroupID"] = "INVALID_GROUP_ID"
		writeFieldErrors(c, fieldErrors)
		return
	}
	inGroup, err := ct.DB.IsUserInGroup(req.ContextGroupID, sessionUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !inGroup {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}
	if err := validateDividendEntrySecurity(ct.DB, req.ContextGroupID, &entry, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if err := validateDividendEntryCurrencyPairs(ct.DB, req.ContextGroupID, &entry, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if err := validateInlandTaxDetails(ct.DB, req.ContextGroupID, &entry, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	prepareInlandTaxFields(&entry, fieldErrors)
	if err := validateInlandTaxCurrency(ct.DB, req.ContextGroupID, &entry, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if err := validateDividendEntryFXRate(ct.DB, req.ContextGroupID, &entry, fieldErrors); err != nil {
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

	if err := prepareCalculatedDividendFields(ct.DB, req.ContextGroupID, &entry, fieldErrors); err != nil {
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
	groupID := int64(0)
	if req.ContextGroupID != nil {
		groupID = *req.ContextGroupID
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
	if req.InlandTaxAmount != nil {
		updated.InlandTaxAmount = *req.InlandTaxAmount
	} else if req.requiresInlandTaxRecalculation() {
		updated.InlandTaxAmount = ""
	}
	if req.InlandTaxCurrency != nil {
		updated.InlandTaxCurrency = *req.InlandTaxCurrency
	} else if req.InlandTaxAmount == nil && req.requiresInlandTaxRecalculation() {
		updated.InlandTaxCurrency = ""
	}
	if req.InlandTaxDetails != nil {
		updated.InlandTaxDetails = *req.InlandTaxDetails
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
	if groupID <= 0 {
		fieldErrors["ContextGroupID"] = "INVALID_GROUP_ID"
		writeFieldErrors(c, fieldErrors)
		return
	}
	inGroup, err := ct.DB.IsUserInGroup(groupID, sessionUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !inGroup {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}
	if err := validateDividendEntrySecurity(ct.DB, groupID, &updated, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if err := validateDividendEntryCurrencyPairs(ct.DB, groupID, &updated, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if err := validateInlandTaxDetails(ct.DB, groupID, &updated, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	prepareInlandTaxFields(&updated, fieldErrors)
	if err := validateInlandTaxCurrency(ct.DB, groupID, &updated, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if err := validateDividendEntryFXRate(ct.DB, groupID, &updated, fieldErrors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if len(fieldErrors) > 0 {
		writeFieldErrors(c, fieldErrors)
		return
	}

	if err := prepareCalculatedDividendFields(ct.DB, groupID, &updated, fieldErrors); err != nil {
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
