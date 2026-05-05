package server_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/controller"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/grrt"
	"github.com/geschke/schrevind/pkg/totpcrypto"
	"github.com/geschke/schrevind/pkg/users"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/pquerna/otp/totp"
)

// TestAuthLoginLogoutFlow tests the expected behavior of this component.
func TestAuthLoginLogoutFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldCfg := config.Cfg
	t.Cleanup(func() { config.Cfg = oldCfg })

	config.Cfg.WebUI.Enabled = true
	config.Cfg.WebUI.SessionKey = strings.Repeat("k", 32)
	config.Cfg.WebUI.SessionName = "schrevind_session"
	config.Cfg.WebUI.CookieSecure = false
	config.Cfg.WebUI.CookieSameSite = "lax"
	config.Cfg.WebUI.CookieMaxAgeDays = 30
	config.Cfg.WebUI.CORSAllowedOrigins = []string{"http://localhost:3000"}
	config.Cfg.WebUI.TOTPEncryptionKey = strings.Repeat("0", 64)

	dbPath := filepath.Join(t.TempDir(), "auth-it.sqlite")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	_, err = users.Create(context.Background(), database, users.CreateParams{
		Email:     "admin@example.com",
		Password:  "Secret123!",
		FirstName: "Ada",
		LastName:  "Lovelace",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	store := sessions.NewCookieStore([]byte(config.Cfg.WebUI.SessionKey))
	authCtl := controller.NewAuthController(database, store, config.Cfg.WebUI.SessionName)
	usersCtl := controller.NewUsersController(database, store, config.Cfg.WebUI.SessionName, grrt.New(database))

	r := gin.New()
	r.POST("/api/auth/login", authCtl.PostLogin)
	r.POST("/api/auth/logout", authCtl.PostLogout)
	r.GET("/api/users/list", usersCtl.GetList)

	srv := httptest.NewServer(r)
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	origin := "http://localhost:3000"

	loginReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", bytes.NewBufferString(
		`{"email":"admin@example.com","password":"Secret123!"}`,
	))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("Origin", origin)

	loginRes, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer loginRes.Body.Close()

	if loginRes.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRes.StatusCode, mustReadBody(t, loginRes))
	}
	if !strings.Contains(strings.Join(loginRes.Header.Values("Set-Cookie"), ";"), config.Cfg.WebUI.SessionName+"=") {
		t.Fatalf("login should set session cookie")
	}

	listReqBefore, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/users/list", nil)
	listReqBefore.Header.Set("Origin", origin)

	listResBefore, err := client.Do(listReqBefore)
	if err != nil {
		t.Fatalf("users/list before logout request: %v", err)
	}
	defer listResBefore.Body.Close()

	if listResBefore.StatusCode != http.StatusOK {
		t.Fatalf("users/list before logout status=%d body=%s", listResBefore.StatusCode, mustReadBody(t, listResBefore))
	}

	logoutReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	logoutReq.Header.Set("Origin", origin)

	logoutRes, err := client.Do(logoutReq)
	if err != nil {
		t.Fatalf("logout request: %v", err)
	}
	defer logoutRes.Body.Close()

	if logoutRes.StatusCode != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", logoutRes.StatusCode, mustReadBody(t, logoutRes))
	}

	logoutSetCookie := strings.Join(logoutRes.Header.Values("Set-Cookie"), ";")
	if !strings.Contains(logoutSetCookie, config.Cfg.WebUI.SessionName+"=") {
		t.Fatalf("logout should return updated session cookie")
	}
	if !strings.Contains(logoutSetCookie, "Max-Age=0") && !strings.Contains(strings.ToLower(logoutSetCookie), "expires=") {
		t.Fatalf("logout cookie should expire session")
	}

	listReqAfter, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/users/list", nil)
	listReqAfter.Header.Set("Origin", origin)

	listResAfter, err := client.Do(listReqAfter)
	if err != nil {
		t.Fatalf("users/list after logout request: %v", err)
	}
	defer listResAfter.Body.Close()

	if listResAfter.StatusCode != http.StatusUnauthorized {
		t.Fatalf("users/list after logout status=%d body=%s", listResAfter.StatusCode, mustReadBody(t, listResAfter))
	}

	logoutReqAgain, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	logoutReqAgain.Header.Set("Origin", origin)

	logoutResAgain, err := client.Do(logoutReqAgain)
	if err != nil {
		t.Fatalf("second logout request: %v", err)
	}
	defer logoutResAgain.Body.Close()

	if logoutResAgain.StatusCode != http.StatusOK {
		t.Fatalf("second logout status=%d body=%s", logoutResAgain.StatusCode, mustReadBody(t, logoutResAgain))
	}
}

func TestAuth2FALoginVerifyFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldCfg := config.Cfg
	t.Cleanup(func() { config.Cfg = oldCfg })

	config.Cfg.WebUI.Enabled = true
	config.Cfg.WebUI.SessionKey = strings.Repeat("k", 32)
	config.Cfg.WebUI.SessionName = "schrevind_session"
	config.Cfg.WebUI.CookieSecure = false
	config.Cfg.WebUI.CookieSameSite = "lax"
	config.Cfg.WebUI.CookieMaxAgeDays = 30
	config.Cfg.WebUI.CORSAllowedOrigins = []string{"http://localhost:3000"}
	config.Cfg.WebUI.TOTPEncryptionKey = strings.Repeat("1", 64)

	dbPath := filepath.Join(t.TempDir(), "auth-2fa-it.sqlite")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	userID, err := users.Create(context.Background(), database, users.CreateParams{
		Email:     "totp@example.com",
		Password:  "Secret123!",
		FirstName: "Tina",
		LastName:  "Totp",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	key, err := config.TOTPEncryptionKeyBytes()
	if err != nil {
		t.Fatalf("TOTPEncryptionKeyBytes() error = %v", err)
	}
	secret := "JBSWY3DPEHPK3PXP"
	encryptedSecret, err := totpcrypto.EncryptTOTPSecret(secret, key)
	if err != nil {
		t.Fatalf("EncryptTOTPSecret() error = %v", err)
	}
	if _, err := database.UpdateUserSettings(context.Background(), userID, db.UserSettings{
		TOTPEnabled: true,
		TOTPSecret:  encryptedSecret,
	}); err != nil {
		t.Fatalf("UpdateUserSettings() error = %v", err)
	}

	store := sessions.NewCookieStore([]byte(config.Cfg.WebUI.SessionKey))
	authCtl := controller.NewAuthController(database, store, config.Cfg.WebUI.SessionName)

	r := gin.New()
	r.POST("/api/auth/login", authCtl.PostLogin)
	r.POST("/api/auth/2fa/verify", authCtl.Post2FAVerify)
	r.GET("/api/auth/me", authCtl.GetMe)

	srv := httptest.NewServer(r)
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	origin := "http://localhost:3000"

	loginReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", bytes.NewBufferString(
		`{"email":"totp@example.com","password":"Secret123!"}`,
	))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("Origin", origin)

	loginRes, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer loginRes.Body.Close()

	if loginRes.StatusCode != http.StatusAccepted {
		t.Fatalf("login status=%d body=%s", loginRes.StatusCode, mustReadBody(t, loginRes))
	}

	meBeforeReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/auth/me", nil)
	meBeforeReq.Header.Set("Origin", origin)
	meBeforeRes, err := client.Do(meBeforeReq)
	if err != nil {
		t.Fatalf("me before verify request: %v", err)
	}
	defer meBeforeRes.Body.Close()
	if meBeforeRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me before verify status=%d body=%s", meBeforeRes.StatusCode, mustReadBody(t, meBeforeRes))
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}
	verifyReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/2fa/verify", bytes.NewBufferString(
		`{"Code":"`+code+`"}`,
	))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Origin", origin)

	verifyRes, err := client.Do(verifyReq)
	if err != nil {
		t.Fatalf("verify request: %v", err)
	}
	defer verifyRes.Body.Close()
	if verifyRes.StatusCode != http.StatusOK {
		t.Fatalf("verify status=%d body=%s", verifyRes.StatusCode, mustReadBody(t, verifyRes))
	}

	meAfterReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/auth/me", nil)
	meAfterReq.Header.Set("Origin", origin)
	meAfterRes, err := client.Do(meAfterReq)
	if err != nil {
		t.Fatalf("me after verify request: %v", err)
	}
	defer meAfterRes.Body.Close()
	if meAfterRes.StatusCode != http.StatusOK {
		t.Fatalf("me after verify status=%d body=%s", meAfterRes.StatusCode, mustReadBody(t, meAfterRes))
	}
}

func TestAuth2FABackupCodeIsConsumed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldCfg := config.Cfg
	t.Cleanup(func() { config.Cfg = oldCfg })

	config.Cfg.WebUI.Enabled = true
	config.Cfg.WebUI.SessionKey = strings.Repeat("k", 32)
	config.Cfg.WebUI.SessionName = "schrevind_session"
	config.Cfg.WebUI.CookieSecure = false
	config.Cfg.WebUI.CookieSameSite = "lax"
	config.Cfg.WebUI.CookieMaxAgeDays = 30
	config.Cfg.WebUI.CORSAllowedOrigins = []string{"http://localhost:3000"}
	config.Cfg.WebUI.TOTPEncryptionKey = strings.Repeat("2", 64)

	dbPath := filepath.Join(t.TempDir(), "auth-2fa-backup-it.sqlite")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	userID, err := users.Create(context.Background(), database, users.CreateParams{
		Email:    "backup@example.com",
		Password: "Secret123!",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	key, _ := config.TOTPEncryptionKeyBytes()
	encryptedSecret, err := totpcrypto.EncryptTOTPSecret("JBSWY3DPEHPK3PXP", key)
	if err != nil {
		t.Fatalf("EncryptTOTPSecret() error = %v", err)
	}
	hash, err := totpcrypto.HashBackupCode("BACKUP1234")
	if err != nil {
		t.Fatalf("HashBackupCode() error = %v", err)
	}
	if _, err := database.UpdateUserSettings(context.Background(), userID, db.UserSettings{
		TOTPEnabled:     true,
		TOTPSecret:      encryptedSecret,
		TOTPBackupCodes: []string{hash},
	}); err != nil {
		t.Fatalf("UpdateUserSettings() error = %v", err)
	}

	store := sessions.NewCookieStore([]byte(config.Cfg.WebUI.SessionKey))
	authCtl := controller.NewAuthController(database, store, config.Cfg.WebUI.SessionName)
	r := gin.New()
	r.POST("/api/auth/login", authCtl.PostLogin)
	r.POST("/api/auth/2fa/verify", authCtl.Post2FAVerify)
	srv := httptest.NewServer(r)
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	origin := "http://localhost:3000"

	loginReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", bytes.NewBufferString(
		`{"email":"backup@example.com","password":"Secret123!"}`,
	))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("Origin", origin)
	loginRes, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer loginRes.Body.Close()
	if loginRes.StatusCode != http.StatusAccepted {
		t.Fatalf("login status=%d body=%s", loginRes.StatusCode, mustReadBody(t, loginRes))
	}

	verifyReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/2fa/verify", bytes.NewBufferString(
		`{"BackupCode":"BACKUP1234"}`,
	))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Origin", origin)
	verifyRes, err := client.Do(verifyReq)
	if err != nil {
		t.Fatalf("verify request: %v", err)
	}
	defer verifyRes.Body.Close()
	if verifyRes.StatusCode != http.StatusOK {
		t.Fatalf("verify status=%d body=%s", verifyRes.StatusCode, mustReadBody(t, verifyRes))
	}

	u, found, err := database.GetUserByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if !found || u.Settings == nil || len(u.Settings.TOTPBackupCodes) != 0 {
		t.Fatalf("backup codes after verify = %+v, want empty", u.Settings)
	}
}

func TestAuth2FADisableClearsSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldCfg := config.Cfg
	t.Cleanup(func() { config.Cfg = oldCfg })

	config.Cfg.WebUI.Enabled = true
	config.Cfg.WebUI.SessionKey = strings.Repeat("k", 32)
	config.Cfg.WebUI.SessionName = "schrevind_session"
	config.Cfg.WebUI.CookieSecure = false
	config.Cfg.WebUI.CookieSameSite = "lax"
	config.Cfg.WebUI.CookieMaxAgeDays = 30
	config.Cfg.WebUI.CORSAllowedOrigins = []string{"http://localhost:3000"}
	config.Cfg.WebUI.TOTPEncryptionKey = strings.Repeat("3", 64)

	dbPath := filepath.Join(t.TempDir(), "auth-2fa-disable-it.sqlite")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	userID, err := users.Create(context.Background(), database, users.CreateParams{
		Email:    "disable@example.com",
		Password: "Secret123!",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if _, err := database.UpdateUserSettings(context.Background(), userID, db.UserSettings{
		TOTPEnabled:     true,
		TOTPSecret:      "encrypted",
		TOTPBackupCodes: []string{"hash"},
	}); err != nil {
		t.Fatalf("UpdateUserSettings() error = %v", err)
	}

	store := sessions.NewCookieStore([]byte(config.Cfg.WebUI.SessionKey))
	authCtl := controller.NewAuthController(database, store, config.Cfg.WebUI.SessionName)
	r := gin.New()
	r.POST("/api/auth/login", authCtl.PostLogin)
	r.POST("/api/auth/2fa/disable", authCtl.Post2FADisable)
	srv := httptest.NewServer(r)
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	origin := "http://localhost:3000"

	// Login while 2FA is disabled in-memory for the session setup, then re-enable before disable.
	if _, err := database.UpdateUserSettings(context.Background(), userID, db.UserSettings{}); err != nil {
		t.Fatalf("disable setup settings: %v", err)
	}
	loginReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", bytes.NewBufferString(
		`{"email":"disable@example.com","password":"Secret123!"}`,
	))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("Origin", origin)
	loginRes, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer loginRes.Body.Close()
	if loginRes.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRes.StatusCode, mustReadBody(t, loginRes))
	}
	if _, err := database.UpdateUserSettings(context.Background(), userID, db.UserSettings{
		TOTPEnabled:     true,
		TOTPSecret:      "encrypted",
		TOTPBackupCodes: []string{"hash"},
	}); err != nil {
		t.Fatalf("re-enable setup settings: %v", err)
	}

	disableReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/2fa/disable", bytes.NewBufferString(
		`{"Password":"Secret123!"}`,
	))
	disableReq.Header.Set("Content-Type", "application/json")
	disableReq.Header.Set("Origin", origin)
	disableRes, err := client.Do(disableReq)
	if err != nil {
		t.Fatalf("disable request: %v", err)
	}
	defer disableRes.Body.Close()
	if disableRes.StatusCode != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disableRes.StatusCode, mustReadBody(t, disableRes))
	}

	u, found, err := database.GetUserByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if !found || u.Settings == nil {
		t.Fatalf("user/settings missing")
	}
	if u.Settings.TOTPEnabled || u.Settings.TOTPSecret != "" || len(u.Settings.TOTPBackupCodes) != 0 {
		t.Fatalf("settings after disable = %+v, want cleared TOTP fields", u.Settings)
	}
}

func TestAuth2FASetupRejectsAlreadyEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldCfg := config.Cfg
	t.Cleanup(func() { config.Cfg = oldCfg })

	config.Cfg.WebUI.Enabled = true
	config.Cfg.WebUI.SessionKey = strings.Repeat("k", 32)
	config.Cfg.WebUI.SessionName = "schrevind_session"
	config.Cfg.WebUI.CookieSecure = false
	config.Cfg.WebUI.CookieSameSite = "lax"
	config.Cfg.WebUI.CookieMaxAgeDays = 30
	config.Cfg.WebUI.CORSAllowedOrigins = []string{"http://localhost:3000"}
	config.Cfg.WebUI.TOTPEncryptionKey = strings.Repeat("4", 64)

	dbPath := filepath.Join(t.TempDir(), "auth-2fa-setup-it.sqlite")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	userID, err := users.Create(context.Background(), database, users.CreateParams{
		Email:    "setup@example.com",
		Password: "Secret123!",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	store := sessions.NewCookieStore([]byte(config.Cfg.WebUI.SessionKey))
	authCtl := controller.NewAuthController(database, store, config.Cfg.WebUI.SessionName)
	r := gin.New()
	r.POST("/api/auth/login", authCtl.PostLogin)
	r.POST("/api/auth/2fa/setup", authCtl.Post2FASetup)
	srv := httptest.NewServer(r)
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	origin := "http://localhost:3000"

	loginReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", bytes.NewBufferString(
		`{"email":"setup@example.com","password":"Secret123!"}`,
	))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("Origin", origin)
	loginRes, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer loginRes.Body.Close()
	if loginRes.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRes.StatusCode, mustReadBody(t, loginRes))
	}

	if _, err := database.UpdateUserSettings(context.Background(), userID, db.UserSettings{
		TOTPEnabled: true,
	}); err != nil {
		t.Fatalf("UpdateUserSettings() error = %v", err)
	}

	setupReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/2fa/setup", nil)
	setupReq.Header.Set("Origin", origin)
	setupRes, err := client.Do(setupReq)
	if err != nil {
		t.Fatalf("setup request: %v", err)
	}
	defer setupRes.Body.Close()
	if setupRes.StatusCode != http.StatusConflict {
		t.Fatalf("setup status=%d body=%s", setupRes.StatusCode, mustReadBody(t, setupRes))
	}
}

// mustReadBody performs its package-specific operation.
func mustReadBody(t *testing.T, res *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "<read error>"
	}
	return string(b)
}
