package controller

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/totpcrypto"
	"github.com/geschke/schrevind/pkg/users"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/pquerna/otp/totp"
)

type AuthController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
}

// NewAuthController constructs and returns a new instance.
func NewAuthController(database *db.DB, store sessions.Store, sessionName string) *AuthController {
	return &AuthController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
	}
}

type loginRequest struct {
	Email             string `json:"email"`
	Password          string `json:"password"`
	ReturnSecureToken bool   `json:"returnSecureToken"`
}

type twoFAConfirmRequest struct {
	Code        string   `json:"Code"`
	Secret      string   `json:"Secret"`
	BackupCodes []string `json:"BackupCodes"`
}

type twoFAVerifyRequest struct {
	Code       string `json:"Code"`
	BackupCode string `json:"BackupCode"`
}

type twoFADisableRequest struct {
	Password string `json:"Password"`
}

func settingsFromUser(u db.User) db.UserSettings {
	if u.Settings == nil {
		return db.UserSettings{}
	}
	return *u.Settings
}

func (ct AuthController) ensureConfigured(c *gin.Context) bool {
	if ct.DB == nil || ct.DB.SQL == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_NOT_INITIALIZED"})
		return false
	}
	if ct.Store == nil || strings.TrimSpace(ct.SessionName) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "AUTH_NOT_CONFIGURED"})
		return false
	}
	return true
}

func (ct AuthController) currentSessionUserID(c *gin.Context) (int64, bool) {
	sess, _ := ct.Store.Get(c.Request, ct.SessionName)
	if sess == nil || sess.IsNew {
		return 0, false
	}
	rawID, ok := sess.Values["id"]
	if !ok {
		return 0, false
	}
	userID, ok := rawID.(int64)
	if !ok || userID <= 0 {
		return 0, false
	}
	return userID, true
}

func applySessionOptions(sess *sessions.Session, maxAge int) {
	sess.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   config.Cfg.WebUI.CookieSecure,
		SameSite: parseSameSite(config.Cfg.WebUI.CookieSameSite),
	}
}

func normalSessionMaxAge() int {
	maxAgeDays := config.Cfg.WebUI.CookieMaxAgeDays
	if maxAgeDays <= 0 {
		maxAgeDays = 30
	}
	return maxAgeDays * 24 * 60 * 60
}

func setAuthenticatedSession(sess *sessions.Session, u db.User) {
	delete(sess.Values, "2fa_pending_user_id")
	delete(sess.Values, "2fa_pending_expires")
	sess.Values["id"] = u.ID
	sess.Values["email"] = u.Email
	sess.Values["firstname"] = u.FirstName
	sess.Values["lastname"] = u.LastName
	sess.Values["locale"] = u.Locale
	applySessionOptions(sess, normalSessionMaxAge())
}

func setPending2FASession(sess *sessions.Session, u db.User) {
	for k := range sess.Values {
		delete(sess.Values, k)
	}
	sess.Values["2fa_pending_user_id"] = u.ID
	sess.Values["2fa_pending_expires"] = time.Now().Add(5 * time.Minute).Unix()
	applySessionOptions(sess, 5*60)
}

func writeLoginSuccess(c *gin.Context, database *db.DB, u db.User) {
	groups, err := database.ListGroupsWithRoleByUserID(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"id":        strconv.FormatInt(u.ID, 10),
		"email":     u.Email,
		"firstname": u.FirstName,
		"lastname":  u.LastName,
		"locale":    u.Locale,
		"session":   "cookie",
		"groups":    groups,
		"Settings":  sanitizeUserSettingsForResponse(settingsFromUser(u)),
	})
}

// OptionsLogin handles the CORS preflight request.
func (ct AuthController) OptionsLogin(c *gin.Context) {
	// Allow preflight for browser-based clients.
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
}

// OptionsLogout handles the CORS preflight request.
func (ct AuthController) OptionsLogout(c *gin.Context) {
	// Allow preflight for browser-based clients.
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
}

// OptionsMe handles the CORS preflight request.
func (ct AuthController) OptionsMe(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
}

func (ct AuthController) Options2FASetup(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
}

func (ct AuthController) Options2FAConfirm(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
}

func (ct AuthController) Options2FAVerify(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
}

func (ct AuthController) Options2FADisable(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
}

// PostLogin performs its package-specific operation.
func (ct AuthController) PostLogin(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}

	if !ct.ensureConfigured(c) {
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password
	if email == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_CREDENTIALS"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	u, found, err := ct.DB.GetUserByEmail(ctx, email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	// Avoid leaking whether the email exists.
	if !found {
		// Do a tiny constant-time op to keep timing closer.
		_ = subtle.ConstantTimeCompare([]byte("a"), []byte("b"))
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "INVALID_CREDENTIALS"})
		return
	}

	ok, err := users.VerifyPassword(password, u.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "INVALID_CREDENTIALS"})
		return
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "INVALID_CREDENTIALS"})
		return
	}

	settings := settingsFromUser(u)
	sess, _ := ct.Store.Get(c.Request, ct.SessionName)
	if settings.TOTPEnabled {
		setPending2FASession(sess, u)
		if err := sess.Save(c.Request, c.Writer); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "SESSION_SAVE_FAILED"})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"success": true,
			"message": "TWO_FACTOR_REQUIRED",
			"session": "2fa_pending",
		})
		return
	}

	setAuthenticatedSession(sess, u)
	if err := sess.Save(c.Request, c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "SESSION_SAVE_FAILED"})
		return
	}

	writeLoginSuccess(c, ct.DB, u)
}

// Post2FASetup generates a new TOTP secret and backup codes for the authenticated user.
func (ct AuthController) Post2FASetup(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureConfigured(c) {
		return
	}

	userID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	u, found, err := ct.DB.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}
	settings := settingsFromUser(u)
	if settings.TOTPEnabled {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "TWO_FACTOR_ALREADY_ENABLED"})
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Schrevind",
		AccountName: u.Email,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	backupCodes, _, err := totpcrypto.GenerateBackupCodes(8)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"OTPAuthURL":  key.URL(),
		"Secret":      key.Secret(),
		"BackupCodes": backupCodes,
	})
}

// Post2FAConfirm confirms a TOTP setup and stores encrypted 2FA settings.
func (ct AuthController) Post2FAConfirm(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureConfigured(c) {
		return
	}

	userID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	var req twoFAConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	code := strings.TrimSpace(req.Code)
	secret := strings.TrimSpace(req.Secret)
	if code == "" || secret == "" || len(req.BackupCodes) != 8 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_2FA_CODE"})
		return
	}
	if !totp.Validate(code, secret) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_2FA_CODE"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	u, found, err := ct.DB.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}
	settings := settingsFromUser(u)
	if settings.TOTPEnabled {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "TWO_FACTOR_ALREADY_ENABLED"})
		return
	}

	encryptionKey, err := config.TOTPEncryptionKeyBytes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "TOTP_SECRET_ENCRYPTION_FAILED"})
		return
	}
	encryptedSecret, err := totpcrypto.EncryptTOTPSecret(secret, encryptionKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "TOTP_SECRET_ENCRYPTION_FAILED"})
		return
	}

	hashes := make([]string, 0, len(req.BackupCodes))
	for _, code := range req.BackupCodes {
		code = strings.TrimSpace(code)
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_BACKUP_CODE"})
			return
		}
		hash, err := totpcrypto.HashBackupCode(code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		hashes = append(hashes, hash)
	}

	settings.TOTPEnabled = true
	settings.TOTPSecret = encryptedSecret
	settings.TOTPBackupCodes = hashes
	updated, err := ct.DB.UpdateUserSettings(ctx, userID, settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !updated {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "TWO_FACTOR_ENABLED"})
}

func (ct AuthController) pending2FAUserID(c *gin.Context) (int64, bool) {
	sess, _ := ct.Store.Get(c.Request, ct.SessionName)
	if sess == nil || sess.IsNew {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return 0, false
	}

	rawID, ok := sess.Values["2fa_pending_user_id"]
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return 0, false
	}
	userID, ok := rawID.(int64)
	if !ok || userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return 0, false
	}

	rawExpires, ok := sess.Values["2fa_pending_expires"]
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return 0, false
	}
	expires, ok := rawExpires.(int64)
	if !ok || time.Now().Unix() > expires {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "TWO_FACTOR_PENDING_EXPIRED"})
		return 0, false
	}

	return userID, true
}

// Post2FAVerify verifies a pending second factor and upgrades the session.
func (ct AuthController) Post2FAVerify(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureConfigured(c) {
		return
	}

	userID, ok := ct.pending2FAUserID(c)
	if !ok {
		return
	}

	var req twoFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	u, found, err := ct.DB.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}
	settings := settingsFromUser(u)
	if !settings.TOTPEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "TWO_FACTOR_NOT_ENABLED"})
		return
	}

	verified := false
	code := strings.TrimSpace(req.Code)
	backupCode := strings.TrimSpace(req.BackupCode)
	if code != "" {
		encryptionKey, err := config.TOTPEncryptionKeyBytes()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "TOTP_SECRET_DECRYPTION_FAILED"})
			return
		}
		secret, err := totpcrypto.DecryptTOTPSecret(settings.TOTPSecret, encryptionKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "TOTP_SECRET_DECRYPTION_FAILED"})
			return
		}
		verified = totp.Validate(code, secret)
		if !verified {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_2FA_CODE"})
			return
		}
	} else if backupCode != "" {
		match, index, err := totpcrypto.ValidateBackupCode(backupCode, settings.TOTPBackupCodes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if !match {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_BACKUP_CODE"})
			return
		}
		settings.TOTPBackupCodes = append(settings.TOTPBackupCodes[:index], settings.TOTPBackupCodes[index+1:]...)
		updated, err := ct.DB.UpdateUserSettings(ctx, userID, settings)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if !updated {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
			return
		}
		u.Settings = &settings
		verified = true
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_2FA_CODE"})
		return
	}

	if !verified {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_2FA_CODE"})
		return
	}

	sess, _ := ct.Store.Get(c.Request, ct.SessionName)
	setAuthenticatedSession(sess, u)
	if err := sess.Save(c.Request, c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "SESSION_SAVE_FAILED"})
		return
	}

	writeLoginSuccess(c, ct.DB, u)
}

// Post2FADisable disables TOTP for the authenticated user after password confirmation.
func (ct AuthController) Post2FADisable(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureConfigured(c) {
		return
	}

	userID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	var req twoFADisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	password := req.Password
	if password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_CREDENTIALS"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	u, found, err := ct.DB.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	ok, err = users.VerifyPassword(password, u.Password)
	if err != nil || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "INVALID_CREDENTIALS"})
		return
	}

	settings := settingsFromUser(u)
	settings.TOTPEnabled = false
	settings.TOTPSecret = ""
	settings.TOTPBackupCodes = nil
	updated, err := ct.DB.UpdateUserSettings(ctx, userID, settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !updated {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "TWO_FACTOR_DISABLED"})
}

// PostLogout performs its package-specific operation.
func (ct AuthController) PostLogout(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}

	if ct.Store == nil || strings.TrimSpace(ct.SessionName) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "AUTH_NOT_CONFIGURED"})
		return
	}

	sess, _ := ct.Store.Get(c.Request, ct.SessionName)
	if sess == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "LOGGED_OUT"})
		return
	}

	for k := range sess.Values {
		delete(sess.Values, k)
	}

	sess.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   config.Cfg.WebUI.CookieSecure,
		SameSite: parseSameSite(config.Cfg.WebUI.CookieSameSite),
	}

	if err := sess.Save(c.Request, c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "SESSION_SAVE_FAILED"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "LOGGED_OUT"})
}

// GetMe returns the current authenticated user for a valid session.
func (ct AuthController) GetMe(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}

	if ct.DB == nil || ct.DB.SQL == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_NOT_INITIALIZED"})
		return
	}
	if ct.Store == nil || strings.TrimSpace(ct.SessionName) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "AUTH_NOT_CONFIGURED"})
		return
	}

	sess, _ := ct.Store.Get(c.Request, ct.SessionName)
	if sess == nil || sess.IsNew {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	rawID, ok := sess.Values["id"]
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}
	userID, ok := rawID.(int64)
	if !ok || userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	u, found, err := ct.DB.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		// Session points to a deleted user -> expire cookie.
		for k := range sess.Values {
			delete(sess.Values, k)
		}
		sess.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   config.Cfg.WebUI.CookieSecure,
			SameSite: parseSameSite(config.Cfg.WebUI.CookieSameSite),
		}
		_ = sess.Save(c.Request, c.Writer)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	u = sanitizeUserForResponse(u)

	groups, err := ct.DB.ListGroupsWithRoleByUserID(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    u,
		"groups":  groups,
	})
}

// parseSameSite performs its package-specific operation.
func parseSameSite(v string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax", "":
		return http.SameSiteLaxMode
	default:
		// Default to Lax, but still make the config error visible in logs later.
		return http.SameSiteLaxMode
	}
}
