package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	displayformat "github.com/geschke/schrevind/pkg/format"
	"github.com/geschke/schrevind/pkg/grrt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

type AnalysesController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
	G           *grrt.Grrt
}

type AnalysisResponse struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Data    AnalysisData `json:"data"`
}

type AnalysisData struct {
	ID       string           `json:"id"`
	TitleKey string           `json:"title_key"`
	Type     string           `json:"type"`
	Columns  []AnalysisColumn `json:"columns"`
	Rows     []AnalysisRow    `json:"rows"`
}

type AnalysisColumn struct {
	Key      string `json:"key"`
	LabelKey string `json:"label_key"`
	Datatype string `json:"datatype"`
	Currency string `json:"currency,omitempty"`
	Align    string `json:"align"`
}

type AnalysisRow map[string]string

type dividendsByYearTotals struct {
	Gross            decimal.Decimal
	AfterWithholding decimal.Decimal
	Net              decimal.Decimal
}

type dividendsByYearMonthTotals struct {
	Gross            decimal.Decimal
	AfterWithholding decimal.Decimal
	Net              decimal.Decimal
}

// NewAnalysesController creates a controller for analysis HTTP handlers.
func NewAnalysesController(database *db.DB, store sessions.Store, sessionName string, g *grrt.Grrt) *AnalysesController {
	return &AnalysesController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
		G:           g,
	}
}

// Options handles CORS preflight requests for analysis routes.
func (ct AnalysesController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

// ensureAuthorized verifies database/session setup and requires an authenticated session.
func (ct AnalysesController) ensureAuthorized(c *gin.Context) bool {
	if ct.DB == nil || ct.DB.SQL == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_NOT_INITIALIZED"})
		return false
	}
	if ct.G == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "AUTH_NOT_CONFIGURED"})
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
func (ct AnalysesController) currentSessionUserID(c *gin.Context) (int64, bool) {
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

func (ct AnalysesController) currentSessionUserLocale(c *gin.Context) (string, bool, error) {
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

// GetDividendsByYear handles GET /api/analyses/dividends-by-year.
func (ct AnalysesController) GetDividendsByYear(c *gin.Context) {
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

	allowed, err := ct.G.CanDoAny(userID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	scope, err := ct.G.ScopeForAction(userID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	depots, err := ct.DB.ListDepotsForActionScope(userID, scope.All, scope.Roles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	baseCurrency, ok, currencies := uniqueDepotBaseCurrency(depots)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "ANALYSIS_MULTIPLE_BASE_CURRENCIES",
			"currencies": currencies,
		})
		return
	}

	decimalPlaces := int32(2)
	if baseCurrency != "" {
		currency, err := ct.DB.GetCurrencyByCurrency(baseCurrency)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if currency != nil {
			decimalPlaces = int32(currency.DecimalPlaces)
		}
	}

	depotIDs := make([]int64, 0, len(depots))
	for _, depot := range depots {
		depotIDs = append(depotIDs, depot.ID)
	}

	sourceRows, err := ct.DB.ListDividendAnalysisRowsByDepotIDs(depotIDs)
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

	rows, ok := buildDividendsByYearRows(sourceRows, decimalPlaces, locale)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ANALYSIS_INVALID_DECIMAL_VALUE"})
		return
	}

	c.JSON(http.StatusOK, AnalysisResponse{
		Success: true,
		Message: "ANALYSIS_OK",
		Data: AnalysisData{
			ID:       "dividends_by_year",
			TitleKey: "analyses.dividends_by_year.title",
			Type:     "table",
			Columns:  dividendsByYearColumns(baseCurrency),
			Rows:     rows,
		},
	})
}

// GetDividendsByYearMonth handles GET /api/analyses/dividends-by-year-month.
func (ct AnalysesController) GetDividendsByYearMonth(c *gin.Context) {
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

	allowed, err := ct.G.CanDoAny(userID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	scope, err := ct.G.ScopeForAction(userID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	depots, err := ct.DB.ListDepotsForActionScope(userID, scope.All, scope.Roles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	baseCurrency, ok, currencies := uniqueDepotBaseCurrency(depots)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "ANALYSIS_MULTIPLE_BASE_CURRENCIES",
			"currencies": currencies,
		})
		return
	}

	decimalPlaces := int32(2)
	if baseCurrency != "" {
		currency, err := ct.DB.GetCurrencyByCurrency(baseCurrency)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if currency != nil {
			decimalPlaces = int32(currency.DecimalPlaces)
		}
	}

	depotIDs := make([]int64, 0, len(depots))
	for _, depot := range depots {
		depotIDs = append(depotIDs, depot.ID)
	}

	sourceRows, err := ct.DB.ListDividendAnalysisMonthRowsByDepotIDs(depotIDs)
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

	rows, ok := buildDividendsByYearMonthRows(sourceRows, decimalPlaces, locale)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ANALYSIS_INVALID_DECIMAL_VALUE"})
		return
	}

	c.JSON(http.StatusOK, AnalysisResponse{
		Success: true,
		Message: "ANALYSIS_OK",
		Data: AnalysisData{
			ID:       "dividends_by_year_month",
			TitleKey: "analyses.dividends_by_year_month.title",
			Type:     "table",
			Columns:  dividendsByYearMonthColumns(baseCurrency),
			Rows:     rows,
		},
	})
}

func uniqueDepotBaseCurrency(depots []db.Depot) (string, bool, []string) {
	set := make(map[string]struct{})
	for _, depot := range depots {
		baseCurrency := strings.TrimSpace(depot.BaseCurrency)
		if baseCurrency == "" {
			continue
		}
		set[baseCurrency] = struct{}{}
	}

	currencies := make([]string, 0, len(set))
	for currency := range set {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)

	if len(currencies) > 1 {
		return "", false, currencies
	}
	if len(currencies) == 0 {
		return "", true, currencies
	}
	return currencies[0], true, currencies
}

func buildDividendsByYearRows(sourceRows []db.DividendsByYearSourceRow, decimalPlaces int32, locale string) ([]AnalysisRow, bool) {
	totalsByYear := make(map[string]dividendsByYearTotals)
	for _, sourceRow := range sourceRows {
		gross, err := decimal.NewFromString(sourceRow.Gross)
		if err != nil {
			return nil, false
		}
		afterWithholding, err := decimal.NewFromString(sourceRow.AfterWithholding)
		if err != nil {
			return nil, false
		}
		net, err := decimal.NewFromString(sourceRow.Net)
		if err != nil {
			return nil, false
		}

		totals := totalsByYear[sourceRow.Year]
		totals.Gross = totals.Gross.Add(gross)
		totals.AfterWithholding = totals.AfterWithholding.Add(afterWithholding)
		totals.Net = totals.Net.Add(net)
		totalsByYear[sourceRow.Year] = totals
	}

	years := make([]string, 0, len(totalsByYear))
	for year := range totalsByYear {
		years = append(years, year)
	}
	sort.Strings(years)

	rows := make([]AnalysisRow, 0, len(years))
	for _, year := range years {
		totals := totalsByYear[year]
		gross := totals.Gross.StringFixed(decimalPlaces)
		afterWithholding := totals.AfterWithholding.StringFixed(decimalPlaces)
		net := totals.Net.StringFixed(decimalPlaces)
		rows = append(rows, AnalysisRow{
			"year":              year,
			"gross":             displayformat.DecimalForLocale(gross, locale),
			"after_withholding": displayformat.DecimalForLocale(afterWithholding, locale),
			"net":               displayformat.DecimalForLocale(net, locale),
		})
	}

	return rows, true
}

func buildDividendsByYearMonthRows(sourceRows []db.DividendsByYearMonthSourceRow, decimalPlaces int32, locale string) ([]AnalysisRow, bool) {
	yearTotals := make(map[string]dividendsByYearMonthTotals)
	monthTotals := make(map[string]map[string]dividendsByYearMonthTotals)

	for _, sourceRow := range sourceRows {
		gross, err := decimal.NewFromString(sourceRow.Gross)
		if err != nil {
			return nil, false
		}
		afterWithholding, err := decimal.NewFromString(sourceRow.AfterWithholding)
		if err != nil {
			return nil, false
		}
		net, err := decimal.NewFromString(sourceRow.Net)
		if err != nil {
			return nil, false
		}

		yearTotal := yearTotals[sourceRow.Year]
		yearTotal.Gross = yearTotal.Gross.Add(gross)
		yearTotal.AfterWithholding = yearTotal.AfterWithholding.Add(afterWithholding)
		yearTotal.Net = yearTotal.Net.Add(net)
		yearTotals[sourceRow.Year] = yearTotal

		if monthTotals[sourceRow.Year] == nil {
			monthTotals[sourceRow.Year] = make(map[string]dividendsByYearMonthTotals)
		}
		monthTotal := monthTotals[sourceRow.Year][sourceRow.Month]
		monthTotal.Gross = monthTotal.Gross.Add(gross)
		monthTotal.AfterWithholding = monthTotal.AfterWithholding.Add(afterWithholding)
		monthTotal.Net = monthTotal.Net.Add(net)
		monthTotals[sourceRow.Year][sourceRow.Month] = monthTotal
	}

	years := make([]string, 0, len(yearTotals))
	for year := range yearTotals {
		years = append(years, year)
	}
	sort.Strings(years)

	rows := make([]AnalysisRow, 0, len(years)*13)
	for _, year := range years {
		rows = append(rows, buildDividendsByYearMonthAnalysisRow("year", year, "", year, yearTotals[year], decimalPlaces, locale))
		for month := 1; month <= 12; month++ {
			monthKey := twoDigitMonth(month)
			rows = append(rows, buildDividendsByYearMonthAnalysisRow("month", year, monthKey, monthKey, monthTotals[year][monthKey], decimalPlaces, locale))
		}
	}

	return rows, true
}

func buildDividendsByYearMonthAnalysisRow(level, year, month, period string, totals dividendsByYearMonthTotals, decimalPlaces int32, locale string) AnalysisRow {
	gross := totals.Gross.StringFixed(decimalPlaces)
	afterWithholding := totals.AfterWithholding.StringFixed(decimalPlaces)
	net := totals.Net.StringFixed(decimalPlaces)

	return AnalysisRow{
		"level":             level,
		"year":              year,
		"month":             month,
		"period":            period,
		"gross":             displayformat.DecimalForLocale(gross, locale),
		"after_withholding": displayformat.DecimalForLocale(afterWithholding, locale),
		"net":               displayformat.DecimalForLocale(net, locale),
	}
}

func twoDigitMonth(month int) string {
	return fmt.Sprintf("%02d", month)
}

func dividendsByYearColumns(currency string) []AnalysisColumn {
	return []AnalysisColumn{
		{
			Key:      "year",
			LabelKey: "analyses.common.year",
			Datatype: "string",
			Align:    "left",
		},
		{
			Key:      "gross",
			LabelKey: "analyses.dividends_by_year.columns.gross",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
		{
			Key:      "after_withholding",
			LabelKey: "analyses.dividends_by_year.columns.after_withholding",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
		{
			Key:      "net",
			LabelKey: "analyses.dividends_by_year.columns.net",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
	}
}

func dividendsByYearMonthColumns(currency string) []AnalysisColumn {
	return []AnalysisColumn{
		{
			Key:      "period",
			LabelKey: "analyses.common.period",
			Datatype: "string",
			Align:    "left",
		},
		{
			Key:      "gross",
			LabelKey: "analyses.dividends_by_year_month.columns.gross",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
		{
			Key:      "after_withholding",
			LabelKey: "analyses.dividends_by_year_month.columns.after_withholding",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
		{
			Key:      "net",
			LabelKey: "analyses.dividends_by_year_month.columns.net",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
	}
}
