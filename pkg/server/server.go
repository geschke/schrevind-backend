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

		depotsCtl := controller.NewDepotsController(database, store, sessionName)
		router.GET("/api/depots/list", depotsCtl.GetList)
		router.OPTIONS("/api/depots/list", depotsCtl.Options)
		router.GET("/api/depots/:id", depotsCtl.GetByID)
		router.OPTIONS("/api/depots/:id", depotsCtl.Options)
		router.POST("/api/depots/add", depotsCtl.PostAdd)
		router.OPTIONS("/api/depots/add", depotsCtl.Options)
		router.POST("/api/depots/update/:id", depotsCtl.PostUpdate)
		router.OPTIONS("/api/depots/update/:id", depotsCtl.Options)
		router.POST("/api/depots/delete/:id", depotsCtl.PostDelete)
		router.OPTIONS("/api/depots/delete/:id", depotsCtl.Options)

		securitiesCtl := controller.NewSecuritiesController(database, store, sessionName)
		router.GET("/api/securities/list", securitiesCtl.GetList)
		router.OPTIONS("/api/securities/list", securitiesCtl.Options)
		router.GET("/api/securities/:id", securitiesCtl.GetByID)
		router.OPTIONS("/api/securities/:id", securitiesCtl.Options)
		router.POST("/api/securities/add", securitiesCtl.PostAdd)
		router.OPTIONS("/api/securities/add", securitiesCtl.Options)
		router.POST("/api/securities/update/:id", securitiesCtl.PostUpdate)
		router.OPTIONS("/api/securities/update/:id", securitiesCtl.Options)
		router.POST("/api/securities/delete/:id", securitiesCtl.PostDelete)
		router.OPTIONS("/api/securities/delete/:id", securitiesCtl.Options)

		currenciesCtl := controller.NewCurrenciesController(database, store, sessionName)
		router.GET("/api/currencies/list", currenciesCtl.GetList)
		router.OPTIONS("/api/currencies/list", currenciesCtl.Options)
		router.GET("/api/currencies/:id", currenciesCtl.GetByID)
		router.OPTIONS("/api/currencies/:id", currenciesCtl.Options)
		router.POST("/api/currencies/add", currenciesCtl.PostAdd)
		router.OPTIONS("/api/currencies/add", currenciesCtl.Options)
		router.POST("/api/currencies/update/:id", currenciesCtl.PostUpdate)
		router.OPTIONS("/api/currencies/update/:id", currenciesCtl.Options)
		router.POST("/api/currencies/delete/:id", currenciesCtl.PostDelete)
		router.OPTIONS("/api/currencies/delete/:id", currenciesCtl.Options)

		withholdingTaxDefaultsCtl := controller.NewWithholdingTaxDefaultsController(database, store, sessionName)
		router.GET("/api/withholding-tax-defaults/list", withholdingTaxDefaultsCtl.GetList)
		router.OPTIONS("/api/withholding-tax-defaults/list", withholdingTaxDefaultsCtl.Options)
		router.GET("/api/withholding-tax-defaults/by-depot/:depot_id", withholdingTaxDefaultsCtl.GetListByDepot)
		router.OPTIONS("/api/withholding-tax-defaults/by-depot/:depot_id", withholdingTaxDefaultsCtl.Options)
		router.GET("/api/withholding-tax-defaults/effective", withholdingTaxDefaultsCtl.GetEffective)
		router.OPTIONS("/api/withholding-tax-defaults/effective", withholdingTaxDefaultsCtl.Options)
		router.GET("/api/withholding-tax-defaults/:id", withholdingTaxDefaultsCtl.GetByID)
		router.OPTIONS("/api/withholding-tax-defaults/:id", withholdingTaxDefaultsCtl.Options)
		router.POST("/api/withholding-tax-defaults/add", withholdingTaxDefaultsCtl.PostAdd)
		router.OPTIONS("/api/withholding-tax-defaults/add", withholdingTaxDefaultsCtl.Options)
		router.POST("/api/withholding-tax-defaults/update/:id", withholdingTaxDefaultsCtl.PostUpdate)
		router.OPTIONS("/api/withholding-tax-defaults/update/:id", withholdingTaxDefaultsCtl.Options)
		router.POST("/api/withholding-tax-defaults/delete/:id", withholdingTaxDefaultsCtl.PostDelete)
		router.OPTIONS("/api/withholding-tax-defaults/delete/:id", withholdingTaxDefaultsCtl.Options)

		dividendEntriesCtl := controller.NewDividendEntriesController(database, store, sessionName)
		router.GET("/api/dividend-entries/by-user/:user_id/range", dividendEntriesCtl.GetListByUserAndRange)
		router.OPTIONS("/api/dividend-entries/by-user/:user_id/range", dividendEntriesCtl.Options)
		router.GET("/api/dividend-entries/by-depot/:depot_id/range", dividendEntriesCtl.GetListByDepotAndRange)
		router.OPTIONS("/api/dividend-entries/by-depot/:depot_id/range", dividendEntriesCtl.Options)
		router.GET("/api/dividend-entries/by-security/:security_id/range", dividendEntriesCtl.GetListBySecurityAndRange)
		router.OPTIONS("/api/dividend-entries/by-security/:security_id/range", dividendEntriesCtl.Options)
		router.GET("/api/dividend-entries/by-user/:user_id", dividendEntriesCtl.GetListByUser)
		router.OPTIONS("/api/dividend-entries/by-user/:user_id", dividendEntriesCtl.Options)
		router.GET("/api/dividend-entries/by-depot/:depot_id", dividendEntriesCtl.GetListByDepot)
		router.OPTIONS("/api/dividend-entries/by-depot/:depot_id", dividendEntriesCtl.Options)
		router.GET("/api/dividend-entries/by-security/:security_id", dividendEntriesCtl.GetListBySecurity)
		router.OPTIONS("/api/dividend-entries/by-security/:security_id", dividendEntriesCtl.Options)
		router.GET("/api/dividend-entries/:id", dividendEntriesCtl.GetByID)
		router.OPTIONS("/api/dividend-entries/:id", dividendEntriesCtl.Options)
		router.POST("/api/dividend-entries/add", dividendEntriesCtl.PostAdd)
		router.OPTIONS("/api/dividend-entries/add", dividendEntriesCtl.Options)
		router.POST("/api/dividend-entries/update/:id", dividendEntriesCtl.PostUpdate)
		router.OPTIONS("/api/dividend-entries/update/:id", dividendEntriesCtl.Options)
		router.POST("/api/dividend-entries/delete/:id", dividendEntriesCtl.PostDelete)
		router.OPTIONS("/api/dividend-entries/delete/:id", dividendEntriesCtl.Options)
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
