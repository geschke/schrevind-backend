package controller

import (
	"net/http"
	"sort"
	"strings"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

type InlandTaxTemplatesController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
}

// NewInlandTaxTemplatesController constructs and returns a new instance.
func NewInlandTaxTemplatesController(database *db.DB, store sessions.Store, sessionName string) *InlandTaxTemplatesController {
	return &InlandTaxTemplatesController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
	}
}

// Options handles the CORS preflight request.
func (ct InlandTaxTemplatesController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type inlandTaxTemplateSummary struct {
	Template string `json:"Template"`
	Label    string `json:"Label"`
	Currency string `json:"Currency"`
}

// ensureAuthorized performs its package-specific operation.
func (ct InlandTaxTemplatesController) ensureAuthorized(c *gin.Context) bool {
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

func sortedInlandTaxTemplates() []db.InlandTaxTemplate {
	items := make([]db.InlandTaxTemplate, 0, len(db.InlandTaxTemplates))
	for _, item := range db.InlandTaxTemplates {
		item.Fields = append([]db.InlandTaxTemplateField(nil), item.Fields...)
		sort.SliceStable(item.Fields, func(i, j int) bool {
			if item.Fields[i].SortOrder == item.Fields[j].SortOrder {
				return item.Fields[i].Code < item.Fields[j].Code
			}
			return item.Fields[i].SortOrder < item.Fields[j].SortOrder
		})
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Template < items[j].Template
	})
	return items
}

func findInlandTaxTemplate(template string) (db.InlandTaxTemplate, bool) {
	key := strings.ToUpper(strings.TrimSpace(template))
	item, ok := db.InlandTaxTemplates[key]
	if !ok {
		return db.InlandTaxTemplate{}, false
	}
	item.Fields = append([]db.InlandTaxTemplateField(nil), item.Fields...)
	sort.SliceStable(item.Fields, func(i, j int) bool {
		if item.Fields[i].SortOrder == item.Fields[j].SortOrder {
			return item.Fields[i].Code < item.Fields[j].Code
		}
		return item.Fields[i].SortOrder < item.Fields[j].SortOrder
	})
	return item, true
}

// GET /api/inland-tax-templates
func (ct InlandTaxTemplatesController) GetList(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	items := sortedInlandTaxTemplates()
	data := make([]inlandTaxTemplateSummary, 0, len(items))
	for _, item := range items {
		data = append(data, inlandTaxTemplateSummary{
			Template: item.Template,
			Label:    item.Label,
			Currency: item.Currency,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "INLAND_TAX_TEMPLATES_LOADED",
		"data":    data,
	})
}

// GET /api/inland-tax-templates/:template
func (ct InlandTaxTemplatesController) GetByTemplate(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	item, ok := findInlandTaxTemplate(c.Param("template"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "INLAND_TAX_TEMPLATE_NOT_FOUND"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "INLAND_TAX_TEMPLATE_LOADED",
		"data":    item,
	})
}
