package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/grrt"
	"github.com/geschke/schrevind/pkg/users"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

type UsersController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
	G           *grrt.Grrt
}

// NewUsersController constructs and returns a new instance.
func NewUsersController(database *db.DB, store sessions.Store, sessionName string, g *grrt.Grrt) *UsersController {
	return &UsersController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
		G:           g,
	}
}

// Options handles the CORS preflight request.
func (ct UsersController) Options(c *gin.Context) {
	// Allow preflight for browser-based clients.
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type updateUserRequest struct {
	GroupID   int64   `json:"GroupID"`
	Email     *string `json:"Email"`
	FirstName *string `json:"FirstName"`
	LastName  *string `json:"LastName"`
}

type addUserRequest struct {
	GroupID         int64  `json:"GroupID"`
	Email           string `json:"Email"`
	Password        string `json:"Password"`
	PasswordConfirm string `json:"PasswordConfirm"`
	FirstName       string `json:"FirstName"`
	LastName        string `json:"LastName"`
}

type updatePasswordRequest struct {
	GroupID           int64  `json:"GroupID"`
	Password          string `json:"Password"`
	PasswordDuplicate string `json:"PasswordDuplicate"`
}

type deleteUserRequest struct {
	GroupID int64 `json:"GroupID"`
}

// ensureAuthorized performs its package-specific operation.
func (ct UsersController) ensureAuthorized(c *gin.Context) bool {
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
func (ct UsersController) currentSessionUserID(c *gin.Context) (int64, bool) {
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

// canManageUser checks whether sessionUserID may perform action on users in groupID.
// System-admin is checked first; if not, group-admin of the given group is checked.
// Returns (allowed, error).
func (ct UsersController) canManageUser(sessionUserID int64, action string, groupID int64) (bool, error) {
	ok, err := ct.G.Can(sessionUserID, action)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	if groupID <= 0 {
		return false, nil
	}
	return ct.G.CanDo(sessionUserID, db.EntityTypeGroup, action, groupID)
}

// parseUserID performs its package-specific operation.
func parseUserID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_USER_ID"})
		return 0, false
	}
	return id, true
}

// GET /api/users/list
func (ct UsersController) GetList(c *gin.Context) {
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

	allowed, err := ct.G.Can(sessionUserID, "user:list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	items, err := ct.DB.ListUsers(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	for i := range items {
		items[i].Password = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   int64(len(items)),
		"items":   items,
	})
}

// GET /api/users/:id
func (ct UsersController) GetByID(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	// A user may always fetch their own profile.
	if id != sessionUserID {
		allowed, err := ct.G.Can(sessionUserID, "user:list")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	item, found, err := ct.DB.GetUserByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "USER_NOT_FOUND"})
		return
	}

	item.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/users/update/:id
func (ct UsersController) PostUpdate(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	// A user may always edit their own profile; otherwise requires user:edit permission.
	if id != sessionUserID {
		allowed, err := ct.canManageUser(sessionUserID, "user:edit", req.GroupID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
			return
		}
	}

	if req.Email == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_EMAIL"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	current, found, err := ct.DB.GetUserByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "USER_NOT_FOUND"})
		return
	}
	currentEmail := strings.ToLower(strings.TrimSpace(current.Email))
	nextEmail := strings.ToLower(strings.TrimSpace(*req.Email))
	if nextEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_EMAIL"})
		return
	}

	other, otherFound, err := ct.DB.GetUserByEmail(ctx, nextEmail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if otherFound && other.ID != id {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "EMAIL_ALREADY_IN_USE"})
		return
	}

	upd := db.User{
		ID:        id,
		FirstName: current.FirstName,
		LastName:  current.LastName,
	}

	if req.FirstName != nil {
		upd.FirstName = strings.TrimSpace(*req.FirstName)
	}
	if req.LastName != nil {
		upd.LastName = strings.TrimSpace(*req.LastName)
	}
	if nextEmail != currentEmail {
		upd.Email = nextEmail
	}

	updated, err := ct.DB.UpdateUser(ctx, upd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "USER_NOT_FOUND"})
		return
	}

	item, found, err := ct.DB.GetUserByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "USER_NOT_FOUND"})
		return
	}

	item.Password = ""
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/users/update-password/:id
func (ct UsersController) PostUpdatePassword(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	var req updatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	// A user may always change their own password; otherwise requires user:edit permission.
	if id != sessionUserID {
		allowed, err := ct.canManageUser(sessionUserID, "user:edit", req.GroupID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
			return
		}
	}

	password := strings.TrimSpace(req.Password)
	passwordDup := strings.TrimSpace(req.PasswordDuplicate)
	if passwordDup == "" || password != passwordDup {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "PASSWORD_MISMATCH"})
		return
	}
	if err := users.ValidatePassword(password); err != nil {
		switch {
		case errors.Is(err, users.ErrPasswordRequired):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_PASSWORD"})
		case errors.Is(err, users.ErrPasswordTooShort):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "PASSWORD_TOO_SHORT"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_PASSWORD"})
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	current, found, err := ct.DB.GetUserByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "USER_NOT_FOUND"})
		return
	}

	hash, err := users.HashPassword(password, users.DefaultArgon2idParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "PASSWORD_HASH_FAILED"})
		return
	}

	updated, err := ct.DB.UpdateUser(ctx, db.User{
		ID:        id,
		Password:  hash,
		FirstName: current.FirstName,
		LastName:  current.LastName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "USER_NOT_FOUND"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "PASSWORD_UPDATED",
	})
}

// POST /api/users/add
func (ct UsersController) PostAdd(c *gin.Context) {
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

	var req addUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	if req.GroupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return
	}

	allowed, err := ct.canManageUser(sessionUserID, "user:create", req.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)
	passwordConfirm := strings.TrimSpace(req.PasswordConfirm)
	firstName := strings.TrimSpace(req.FirstName)
	lastName := strings.TrimSpace(req.LastName)

	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_EMAIL"})
		return
	}
	if passwordConfirm == "" || password != passwordConfirm {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "PASSWORD_MISMATCH"})
		return
	}
	if err := users.ValidatePassword(password); err != nil {
		switch {
		case errors.Is(err, users.ErrPasswordRequired):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_PASSWORD"})
		case errors.Is(err, users.ErrPasswordTooShort):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "PASSWORD_TOO_SHORT"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_PASSWORD"})
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if existing, found, err := ct.DB.GetUserByEmail(ctx, email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	} else if found && existing.ID > 0 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "EMAIL_ALREADY_IN_USE"})
		return
	}

	hash, err := users.HashPassword(password, users.DefaultArgon2idParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "PASSWORD_HASH_FAILED"})
		return
	}

	newID, err := ct.DB.CreateUser(ctx, db.User{
		Email:     email,
		Password:  hash,
		FirstName: firstName,
		LastName:  lastName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	// Do not auto-assign to the system group — that is a privilege context,
	// not an organisational one. The system admin must add the user explicitly.
	if req.GroupID != db.SystemGroupID {
		if _, err := ct.DB.AddUserToGroup(req.GroupID, newID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
	}

	item, found, err := ct.DB.GetUserByID(ctx, newID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	item.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/users/delete/:id
func (ct UsersController) PostDelete(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	if sessionUserID == id {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "CANNOT_DELETE_OWN_ACCOUNT"})
		return
	}

	var req deleteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	allowed, err := ct.canManageUser(sessionUserID, "user:delete", req.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	deleted, err := ct.DB.DeleteUser(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "USER_NOT_FOUND"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "USER_DELETED",
	})
}
