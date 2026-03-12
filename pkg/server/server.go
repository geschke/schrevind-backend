package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/controller"
	"github.com/geschke/schrevind/pkg/db"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

// Start starts processing.
func Start(database *db.DB) error {
	gin.SetMode(gin.ReleaseMode)

	//if config.LogLevel == "debug" {
	gin.SetMode(gin.DebugMode)
	//}

	router := gin.New()
	if config.Cfg.WebUI.Enabled {
		sessionName := config.Cfg.WebUI.SessionName
		if sessionName == "" {
			sessionName = "schrevind_session"
		}
		store := sessions.NewCookieStore([]byte(config.Cfg.WebUI.SessionKey))
		auth := controller.NewAuthController(database, store, sessionName)
		router.POST("/api/auth/login", auth.PostLogin)
		router.OPTIONS("/api/auth/login", auth.OptionsLogin)
		router.POST("/api/auth/logout", auth.PostLogout)
		router.OPTIONS("/api/auth/logout", auth.OptionsLogout)
		router.GET("/api/auth/me", auth.GetMe)
		router.OPTIONS("/api/auth/me", auth.OptionsMe)

		usersCtl := controller.NewUsersController(database, store, sessionName)
		router.GET("/api/users/list", usersCtl.GetList)
		router.OPTIONS("/api/users/list", usersCtl.Options)
		router.POST("/api/users/add", usersCtl.PostAdd)
		router.OPTIONS("/api/users/add", usersCtl.Options)
		router.GET("/api/users/:id", usersCtl.GetByID)
		router.OPTIONS("/api/users/:id", usersCtl.Options)
		router.POST("/api/users/update/:id", usersCtl.PostUpdate)
		router.OPTIONS("/api/users/update/:id", usersCtl.Options)
		router.POST("/api/users/update-password/:id", usersCtl.PostUpdatePassword)
		router.OPTIONS("/api/users/update-password/:id", usersCtl.Options)
		router.POST("/api/users/delete/:id", usersCtl.PostDelete)
		router.OPTIONS("/api/users/delete/:id", usersCtl.Options)
	}

	// public routes
	router.GET("/", getMain)

	// Basic health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	srv := &http.Server{
		Addr:    config.Cfg.Server.Listen,
		Handler: router,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown failed: %v", err)
	}

	return serveErr
}

// getMain returns data for the requested input.
func getMain(c *gin.Context) {
	c.Header("Access-Control-Allow-Methods", "PUT, POST, GET, DELETE, OPTIONS")
	c.JSON(200, gin.H{
		"message": "nothing here",
	})

}
