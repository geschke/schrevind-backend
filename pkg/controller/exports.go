package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/export"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

// ExportsController handles HTTP endpoints for the export feature.
type ExportsController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
}

// NewExportsController constructs and returns a new instance.
func NewExportsController(database *db.DB, store sessions.Store, sessionName string) *ExportsController {
	return &ExportsController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
	}
}

// Options handles the CORS preflight request.
func (ct ExportsController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

// ensureAuthorized performs its package-specific operation.
func (ct ExportsController) ensureAuthorized(c *gin.Context) bool {
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

// exportFileInfo is the JSON representation of a single export file.
type exportFileInfo struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// POST /api/exports/start
func (ct ExportsController) PostStart(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	ts := time.Now().UTC().Format("2006-01-02T15-04-05")
	filename := fmt.Sprintf("schrevind-backup-%s.json", ts)
	filePath := filepath.Join(config.Cfg.Export.Dir, filename)

	if err := export.Run(ct.DB, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "EXPORT_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "EXPORT_STARTED",
	})
}

// GET /api/exports/list
func (ct ExportsController) GetList(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	entries, err := os.ReadDir(config.Cfg.Export.Dir)
	if err != nil {
		// Return an empty list if the export directory does not exist yet.
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []exportFileInfo{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "EXPORT_LIST_ERROR",
		})
		return
	}

	files := make([]exportFileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, exportFileInfo{
			Filename: entry.Name(),
			Size:     info.Size(),
		})
	}

	// Sort by filename descending so that newer timestamp-named files appear first.
	sort.Slice(files, func(i, j int) bool {
		return files[i].Filename > files[j].Filename
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    files,
	})
}

// GET /api/exports/get/:filename
func (ct ExportsController) GetFile(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	rawName := c.Param("filename")

	// Reject any path traversal: filepath.Base strips directory components,
	// so if the result differs from the raw param, a traversal was attempted.
	safeName := filepath.Base(rawName)
	if safeName != rawName || safeName == "." || safeName == ".." || safeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_FILENAME"})
		return
	}

	filePath := filepath.Join(config.Cfg.Export.Dir, safeName)

	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "EXPORT_NOT_FOUND",
		})
		return
	}

	c.FileAttachment(filePath, safeName)
}
