package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
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

type YearChartResponse struct {
	Success bool                  `json:"success"`
	Message string                `json:"message"`
	Data    YearChartResponseData `json:"data"`
}

type YearChartResponseData struct {
	Categories []string          `json:"categories"`
	Series     []YearChartSeries `json:"series"`
}

type YearChartSeries struct {
	Key      string            `json:"key"`
	Currency string            `json:"currency"`
	Values   []YearChartNumber `json:"values"`
}

type YearChartNumber string

type YearMonthChartResponse struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Data    YearMonthChartResponseData `json:"data"`
}

type YearMonthChartResponseData struct {
	Rows []YearMonthChartRow `json:"rows"`
}

type YearMonthChartRow struct {
	Year             string          `json:"year"`
	Month            string          `json:"month"`
	Gross            YearChartNumber `json:"gross"`
	AfterWithholding YearChartNumber `json:"after_withholding"`
	Net              YearChartNumber `json:"net"`
	Currency         string          `json:"currency"`
}

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

type dividendsBySecurityYearGroupKey struct {
	SecurityID   int64
	SecurityName string
	SecurityISIN string
	Year         string
	Quantity     string
}

type dividendsBySecurityKey struct {
	SecurityID   int64
	SecurityName string
	SecurityISIN string
}

type dividendsBySecurityYearGroup struct {
	dividendsBySecurityYearGroupKey
	FirstPayDate     string
	Gross            decimal.Decimal
	AfterWithholding decimal.Decimal
	Net              decimal.Decimal
}

type dividendsByYearMonthSecurityGroupKey struct {
	Year         string
	Month        string
	SecurityID   int64
	SecurityName string
	SecurityISIN string
}

type dividendsByYearMonthSecurityGroup struct {
	dividendsByYearMonthSecurityGroupKey
	Gross            decimal.Decimal
	AfterWithholding decimal.Decimal
	Net              decimal.Decimal
}

// MarshalJSON emits an already normalized decimal string as a JSON number.
func (n YearChartNumber) MarshalJSON() ([]byte, error) {
	if n == "" {
		return []byte("0"), nil
	}
	return []byte(n), nil
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

// GetDividendsBySecurityYear handles GET /api/analyses/dividends-by-security-year.
func (ct AnalysesController) GetDividendsBySecurityYear(c *gin.Context) {
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

	depots, ok, statusCode, message := filterDepotsByQuery(c, depots)
	if !ok {
		c.JSON(statusCode, gin.H{"success": false, "message": message})
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

	sourceRows, err := ct.DB.ListDividendAnalysisSecurityYearRowsByDepotIDs(depotIDs)
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

	rows, ok := buildDividendsBySecurityYearRows(sourceRows, decimalPlaces, locale)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ANALYSIS_INVALID_DECIMAL_VALUE"})
		return
	}

	c.JSON(http.StatusOK, AnalysisResponse{
		Success: true,
		Message: "ANALYSIS_OK",
		Data: AnalysisData{
			ID:       "dividends_by_security_year",
			TitleKey: "analyses.dividends_by_security_year.title",
			Type:     "table",
			Columns:  dividendsBySecurityYearColumns(baseCurrency),
			Rows:     rows,
		},
	})
}

// GetDividendsByYearMonthSecurity handles GET /api/analyses/dividends-by-year-month-security.
func (ct AnalysesController) GetDividendsByYearMonthSecurity(c *gin.Context) {
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

	depots, ok, statusCode, message := filterDepotsByQuery(c, depots)
	if !ok {
		c.JSON(statusCode, gin.H{"success": false, "message": message})
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

	sourceRows, err := ct.DB.ListDividendAnalysisYearMonthSecurityRowsByDepotIDs(depotIDs)
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

	rows, ok := buildDividendsByYearMonthSecurityRows(sourceRows, decimalPlaces, locale)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ANALYSIS_INVALID_DECIMAL_VALUE"})
		return
	}

	c.JSON(http.StatusOK, AnalysisResponse{
		Success: true,
		Message: "ANALYSIS_OK",
		Data: AnalysisData{
			ID:       "dividends_by_year_month_security",
			TitleKey: "analyses.dividends_by_year_month_security.title",
			Type:     "table",
			Columns:  dividendsByYearMonthSecurityColumns(baseCurrency),
			Rows:     rows,
		},
	})
}

// GetDividendsByYearChart handles GET /api/analyses/dividends-by-year-chart.
func (ct AnalysesController) GetDividendsByYearChart(c *gin.Context) {
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

	depots, ok, statusCode, message := filterDepotsByQuery(c, depots)
	if !ok {
		c.JSON(statusCode, gin.H{"success": false, "message": message})
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

	data, ok := buildDividendsByYearChartData(sourceRows, decimalPlaces, baseCurrency)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ANALYSIS_INVALID_DECIMAL_VALUE"})
		return
	}

	c.JSON(http.StatusOK, YearChartResponse{
		Success: true,
		Message: "ANALYSIS_OK",
		Data:    data,
	})
}

// GetDividendsByYearMonthChart handles GET /api/analyses/dividends-by-year-month-chart.
func (ct AnalysesController) GetDividendsByYearMonthChart(c *gin.Context) {
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

	depots, ok, statusCode, message := filterDepotsByQuery(c, depots)
	if !ok {
		c.JSON(statusCode, gin.H{"success": false, "message": message})
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

	data, ok := buildDividendsByYearMonthChartData(sourceRows, decimalPlaces, baseCurrency)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ANALYSIS_INVALID_DECIMAL_VALUE"})
		return
	}

	c.JSON(http.StatusOK, YearMonthChartResponse{
		Success: true,
		Message: "ANALYSIS_OK",
		Data:    data,
	})
}

func filterDepotsByQuery(c *gin.Context, depots []db.Depot) ([]db.Depot, bool, int, string) {
	return filterDepotsByRequestedIDs(c.QueryArray("depot_id"), depots)
}

func filterDepotsByRequestedIDs(rawIDs []string, depots []db.Depot) ([]db.Depot, bool, int, string) {
	if len(rawIDs) == 0 {
		return depots, true, 0, ""
	}

	requestedIDs := make(map[int64]struct{}, len(rawIDs))
	for _, rawID := range rawIDs {
		id, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
		if err != nil || id <= 0 {
			return nil, false, http.StatusBadRequest, "INVALID_DEPOT_ID"
		}
		requestedIDs[id] = struct{}{}
	}

	filtered := make([]db.Depot, 0, len(requestedIDs))
	for _, depot := range depots {
		if _, ok := requestedIDs[depot.ID]; !ok {
			continue
		}
		filtered = append(filtered, depot)
		delete(requestedIDs, depot.ID)
	}

	if len(requestedIDs) > 0 {
		return nil, false, http.StatusForbidden, "FORBIDDEN"
	}

	return filtered, true, 0, ""
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

func buildDividendsByYearChartData(sourceRows []db.DividendsByYearSourceRow, decimalPlaces int32, currency string) (YearChartResponseData, bool) {
	totalsByYear := make(map[string]dividendsByYearTotals)
	for _, sourceRow := range sourceRows {
		gross, err := decimal.NewFromString(sourceRow.Gross)
		if err != nil {
			return YearChartResponseData{}, false
		}
		afterWithholding, err := decimal.NewFromString(sourceRow.AfterWithholding)
		if err != nil {
			return YearChartResponseData{}, false
		}
		net, err := decimal.NewFromString(sourceRow.Net)
		if err != nil {
			return YearChartResponseData{}, false
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

	data := YearChartResponseData{
		Categories: years,
		Series: []YearChartSeries{
			{Key: "gross", Currency: currency, Values: make([]YearChartNumber, 0, len(years))},
			{Key: "after_withholding", Currency: currency, Values: make([]YearChartNumber, 0, len(years))},
			{Key: "net", Currency: currency, Values: make([]YearChartNumber, 0, len(years))},
		},
	}

	for _, year := range years {
		totals := totalsByYear[year]
		data.Series[0].Values = append(data.Series[0].Values, YearChartNumber(totals.Gross.StringFixed(decimalPlaces)))
		data.Series[1].Values = append(data.Series[1].Values, YearChartNumber(totals.AfterWithholding.StringFixed(decimalPlaces)))
		data.Series[2].Values = append(data.Series[2].Values, YearChartNumber(totals.Net.StringFixed(decimalPlaces)))
	}

	return data, true
}

func buildDividendsByYearMonthChartData(sourceRows []db.DividendsByYearMonthSourceRow, decimalPlaces int32, currency string) (YearMonthChartResponseData, bool) {
	yearTotals := make(map[string]dividendsByYearMonthTotals)
	monthTotals := make(map[string]map[string]dividendsByYearMonthTotals)

	for _, sourceRow := range sourceRows {
		gross, err := decimal.NewFromString(sourceRow.Gross)
		if err != nil {
			return YearMonthChartResponseData{}, false
		}
		afterWithholding, err := decimal.NewFromString(sourceRow.AfterWithholding)
		if err != nil {
			return YearMonthChartResponseData{}, false
		}
		net, err := decimal.NewFromString(sourceRow.Net)
		if err != nil {
			return YearMonthChartResponseData{}, false
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

	data := YearMonthChartResponseData{
		Rows: make([]YearMonthChartRow, 0, len(years)*12),
	}

	for _, year := range years {
		for month := 1; month <= 12; month++ {
			monthKey := twoDigitMonth(month)
			totals := monthTotals[year][monthKey]
			data.Rows = append(data.Rows, YearMonthChartRow{
				Year:             year,
				Month:            monthKey,
				Gross:            YearChartNumber(totals.Gross.StringFixed(decimalPlaces)),
				AfterWithholding: YearChartNumber(totals.AfterWithholding.StringFixed(decimalPlaces)),
				Net:              YearChartNumber(totals.Net.StringFixed(decimalPlaces)),
				Currency:         currency,
			})
		}
	}

	return data, true
}

func buildDividendsBySecurityYearRows(sourceRows []db.DividendsBySecurityYearSourceRow, decimalPlaces int32, locale string) ([]AnalysisRow, bool) {
	groupsByKey := make(map[dividendsBySecurityYearGroupKey]*dividendsBySecurityYearGroup)
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

		key := dividendsBySecurityYearGroupKey{
			SecurityID:   sourceRow.SecurityID,
			SecurityName: sourceRow.SecurityName,
			SecurityISIN: sourceRow.SecurityISIN,
			Year:         sourceRow.Year,
			Quantity:     sourceRow.Quantity,
		}
		group := groupsByKey[key]
		if group == nil {
			group = &dividendsBySecurityYearGroup{
				dividendsBySecurityYearGroupKey: key,
				FirstPayDate:                    sourceRow.PayDate,
			}
			groupsByKey[key] = group
		}
		if group.FirstPayDate == "" || sourceRow.PayDate < group.FirstPayDate {
			group.FirstPayDate = sourceRow.PayDate
		}
		group.Gross = group.Gross.Add(gross)
		group.AfterWithholding = group.AfterWithholding.Add(afterWithholding)
		group.Net = group.Net.Add(net)
	}

	groups := make([]dividendsBySecurityYearGroup, 0, len(groupsByKey))
	for _, group := range groupsByKey {
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return dividendsBySecurityYearGroupLess(groups[i], groups[j])
	})

	rows := make([]AnalysisRow, 0, len(groups))
	var currentSecurity dividendsBySecurityKey
	var currentTotals dividendsByYearTotals
	hasCurrentSecurity := false

	appendSummary := func() {
		rows = append(rows, buildDividendsBySecurityYearSummaryRow(currentSecurity, currentTotals, decimalPlaces, locale))
	}

	for _, group := range groups {
		security := dividendsBySecurityKey{
			SecurityID:   group.SecurityID,
			SecurityName: group.SecurityName,
			SecurityISIN: group.SecurityISIN,
		}
		if hasCurrentSecurity && security != currentSecurity {
			appendSummary()
			currentTotals = dividendsByYearTotals{}
		}
		if !hasCurrentSecurity || security != currentSecurity {
			currentSecurity = security
			hasCurrentSecurity = true
		}

		rows = append(rows, buildDividendsBySecurityYearDetailRow(group, decimalPlaces, locale))
		currentTotals.Gross = currentTotals.Gross.Add(group.Gross)
		currentTotals.AfterWithholding = currentTotals.AfterWithholding.Add(group.AfterWithholding)
		currentTotals.Net = currentTotals.Net.Add(group.Net)
	}

	if hasCurrentSecurity {
		appendSummary()
	}

	return rows, true
}

func dividendsBySecurityYearGroupLess(left, right dividendsBySecurityYearGroup) bool {
	leftName := strings.ToLower(left.SecurityName)
	rightName := strings.ToLower(right.SecurityName)
	if leftName != rightName {
		return leftName < rightName
	}
	if left.SecurityName != right.SecurityName {
		return left.SecurityName < right.SecurityName
	}
	if left.SecurityISIN != right.SecurityISIN {
		return left.SecurityISIN < right.SecurityISIN
	}
	if left.SecurityID != right.SecurityID {
		return left.SecurityID < right.SecurityID
	}
	if left.Year != right.Year {
		return left.Year < right.Year
	}
	if left.FirstPayDate != right.FirstPayDate {
		return left.FirstPayDate < right.FirstPayDate
	}
	return left.Quantity < right.Quantity
}

func buildDividendsBySecurityYearDetailRow(group dividendsBySecurityYearGroup, decimalPlaces int32, locale string) AnalysisRow {
	return AnalysisRow{
		"security_name":     group.SecurityName,
		"security_isin":     group.SecurityISIN,
		"year":              group.Year,
		"quantity":          displayformat.DecimalForLocale(group.Quantity, locale),
		"gross":             displayformat.DecimalForLocale(group.Gross.StringFixed(decimalPlaces), locale),
		"after_withholding": displayformat.DecimalForLocale(group.AfterWithholding.StringFixed(decimalPlaces), locale),
		"net":               displayformat.DecimalForLocale(group.Net.StringFixed(decimalPlaces), locale),
	}
}

func buildDividendsBySecurityYearSummaryRow(security dividendsBySecurityKey, totals dividendsByYearTotals, decimalPlaces int32, locale string) AnalysisRow {
	return AnalysisRow{
		"security_name":     strings.TrimSpace(security.SecurityName + " Ergebnis"),
		"security_isin":     "",
		"year":              "",
		"quantity":          "",
		"gross":             displayformat.DecimalForLocale(totals.Gross.StringFixed(decimalPlaces), locale),
		"after_withholding": displayformat.DecimalForLocale(totals.AfterWithholding.StringFixed(decimalPlaces), locale),
		"net":               displayformat.DecimalForLocale(totals.Net.StringFixed(decimalPlaces), locale),
	}
}

func buildDividendsByYearMonthSecurityRows(sourceRows []db.DividendsByYearMonthSecuritySourceRow, decimalPlaces int32, locale string) ([]AnalysisRow, bool) {
	groupsByKey := make(map[dividendsByYearMonthSecurityGroupKey]*dividendsByYearMonthSecurityGroup)
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

		key := dividendsByYearMonthSecurityGroupKey{
			Year:         sourceRow.Year,
			Month:        sourceRow.Month,
			SecurityID:   sourceRow.SecurityID,
			SecurityName: sourceRow.SecurityName,
			SecurityISIN: sourceRow.SecurityISIN,
		}
		group := groupsByKey[key]
		if group == nil {
			group = &dividendsByYearMonthSecurityGroup{
				dividendsByYearMonthSecurityGroupKey: key,
			}
			groupsByKey[key] = group
		}
		group.Gross = group.Gross.Add(gross)
		group.AfterWithholding = group.AfterWithholding.Add(afterWithholding)
		group.Net = group.Net.Add(net)
	}

	groups := make([]dividendsByYearMonthSecurityGroup, 0, len(groupsByKey))
	for _, group := range groupsByKey {
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return dividendsByYearMonthSecurityGroupLess(groups[i], groups[j])
	})

	rows := make([]AnalysisRow, 0, len(groups))
	var currentYear string
	var currentMonth string
	var currentTotals dividendsByYearTotals
	hasCurrentMonth := false

	appendMonthSummary := func() {
		rows = append(rows, buildDividendsByYearMonthSecuritySummaryRow(currentYear, currentMonth, currentTotals, decimalPlaces, locale))
	}

	for _, group := range groups {
		if hasCurrentMonth && (group.Year != currentYear || group.Month != currentMonth) {
			appendMonthSummary()
			currentTotals = dividendsByYearTotals{}
		}
		if !hasCurrentMonth || group.Year != currentYear || group.Month != currentMonth {
			currentYear = group.Year
			currentMonth = group.Month
			hasCurrentMonth = true
		}

		rows = append(rows, buildDividendsByYearMonthSecurityDetailRow(group, decimalPlaces, locale))
		currentTotals.Gross = currentTotals.Gross.Add(group.Gross)
		currentTotals.AfterWithholding = currentTotals.AfterWithholding.Add(group.AfterWithholding)
		currentTotals.Net = currentTotals.Net.Add(group.Net)
	}

	if hasCurrentMonth {
		appendMonthSummary()
	}

	return rows, true
}

func dividendsByYearMonthSecurityGroupLess(left, right dividendsByYearMonthSecurityGroup) bool {
	if left.Year != right.Year {
		return left.Year < right.Year
	}
	if left.Month != right.Month {
		return left.Month < right.Month
	}
	leftName := strings.ToLower(left.SecurityName)
	rightName := strings.ToLower(right.SecurityName)
	if leftName != rightName {
		return leftName < rightName
	}
	if left.SecurityName != right.SecurityName {
		return left.SecurityName < right.SecurityName
	}
	if left.SecurityISIN != right.SecurityISIN {
		return left.SecurityISIN < right.SecurityISIN
	}
	return left.SecurityID < right.SecurityID
}

func buildDividendsByYearMonthSecurityDetailRow(group dividendsByYearMonthSecurityGroup, decimalPlaces int32, locale string) AnalysisRow {
	return AnalysisRow{
		"year":              group.Year,
		"month":             group.Month,
		"security_name":     group.SecurityName,
		"security_isin":     group.SecurityISIN,
		"gross":             displayformat.DecimalForLocale(group.Gross.StringFixed(decimalPlaces), locale),
		"after_withholding": displayformat.DecimalForLocale(group.AfterWithholding.StringFixed(decimalPlaces), locale),
		"net":               displayformat.DecimalForLocale(group.Net.StringFixed(decimalPlaces), locale),
	}
}

func buildDividendsByYearMonthSecuritySummaryRow(year, month string, totals dividendsByYearTotals, decimalPlaces int32, locale string) AnalysisRow {
	return AnalysisRow{
		"year":              year,
		"month":             month,
		"security_name":     "Monat Ergebnis",
		"security_isin":     "",
		"gross":             displayformat.DecimalForLocale(totals.Gross.StringFixed(decimalPlaces), locale),
		"after_withholding": displayformat.DecimalForLocale(totals.AfterWithholding.StringFixed(decimalPlaces), locale),
		"net":               displayformat.DecimalForLocale(totals.Net.StringFixed(decimalPlaces), locale),
	}
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

func dividendsBySecurityYearColumns(currency string) []AnalysisColumn {
	return []AnalysisColumn{
		{
			Key:      "security_name",
			LabelKey: "analyses.dividends_by_security_year.columns.security_name",
			Datatype: "string",
			Align:    "left",
		},
		{
			Key:      "security_isin",
			LabelKey: "analyses.dividends_by_security_year.columns.security_isin",
			Datatype: "string",
			Align:    "left",
		},
		{
			Key:      "year",
			LabelKey: "analyses.common.year",
			Datatype: "string",
			Align:    "left",
		},
		{
			Key:      "quantity",
			LabelKey: "analyses.dividends_by_security_year.columns.quantity",
			Datatype: "decimal",
			Align:    "right",
		},
		{
			Key:      "gross",
			LabelKey: "analyses.dividends_by_security_year.columns.gross",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
		{
			Key:      "after_withholding",
			LabelKey: "analyses.dividends_by_security_year.columns.after_withholding",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
		{
			Key:      "net",
			LabelKey: "analyses.dividends_by_security_year.columns.net",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
	}
}

func dividendsByYearMonthSecurityColumns(currency string) []AnalysisColumn {
	return []AnalysisColumn{
		{
			Key:      "year",
			LabelKey: "analyses.common.year",
			Datatype: "string",
			Align:    "left",
		},
		{
			Key:      "month",
			LabelKey: "analyses.common.month",
			Datatype: "string",
			Align:    "left",
		},
		{
			Key:      "security_name",
			LabelKey: "analyses.dividends_by_year_month_security.columns.security_name",
			Datatype: "string",
			Align:    "left",
		},
		{
			Key:      "security_isin",
			LabelKey: "analyses.dividends_by_year_month_security.columns.security_isin",
			Datatype: "string",
			Align:    "left",
		},
		{
			Key:      "gross",
			LabelKey: "analyses.dividends_by_year_month_security.columns.gross",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
		{
			Key:      "after_withholding",
			LabelKey: "analyses.dividends_by_year_month_security.columns.after_withholding",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
		{
			Key:      "net",
			LabelKey: "analyses.dividends_by_year_month_security.columns.net",
			Datatype: "currency",
			Currency: currency,
			Align:    "right",
		},
	}
}
