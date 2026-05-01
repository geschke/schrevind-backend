package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/geschke/schrevind/pkg/db"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

type staticSessionStore struct {
	session *sessions.Session
}

func (s staticSessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return s.session, nil
}

func (s staticSessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	return s.session, nil
}

func (s staticSessionStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	return nil
}

func newInlandTaxTemplatesTestController(t *testing.T) (*InlandTaxTemplatesController, sessions.Store, string) {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	sessionName := "schrevind_test_session"
	store := staticSessionStore{
		session: &sessions.Session{
			Values: map[interface{}]interface{}{"id": int64(1)},
			IsNew:  false,
		},
	}
	return NewInlandTaxTemplatesController(database, store, sessionName), store, sessionName
}

func performInlandTaxTemplatesRequest(t *testing.T, ctl *InlandTaxTemplatesController, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/inland-tax-templates", ctl.GetList)
	router.GET("/api/inland-tax-templates/:template", ctl.GetByTemplate)

	req := httptest.NewRequest(method, path, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func TestGetInlandTaxTemplatesReturnsSummariesWithoutFields(t *testing.T) {
	ctl, _, _ := newInlandTaxTemplatesTestController(t)

	rec := performInlandTaxTemplatesRequest(t, ctl, http.MethodGet, "/api/inland-tax-templates")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    []struct {
			Template string `json:"Template"`
			Label    string `json:"Label"`
			Currency string `json:"Currency"`
			Fields   any    `json:"Fields,omitempty"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Message != "INLAND_TAX_TEMPLATES_LOADED" {
		t.Fatalf("response = %+v", body)
	}
	if len(body.Data) != 1 {
		t.Fatalf("data len = %d, want 1", len(body.Data))
	}
	if body.Data[0].Template != "DE" || body.Data[0].Label != "Deutschland" || body.Data[0].Currency != "EUR" {
		t.Fatalf("summary = %+v, want DE Deutschland EUR", body.Data[0])
	}
	if body.Data[0].Fields != nil {
		t.Fatalf("summary Fields = %+v, want nil", body.Data[0].Fields)
	}
}

func TestGetInlandTaxTemplateReturnsFields(t *testing.T) {
	ctl, _, _ := newInlandTaxTemplatesTestController(t)

	rec := performInlandTaxTemplatesRequest(t, ctl, http.MethodGet, "/api/inland-tax-templates/de")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			Template string `json:"Template"`
			Label    string `json:"Label"`
			Currency string `json:"Currency"`
			Fields   []struct {
				Code      string `json:"Code"`
				Label     string `json:"Label"`
				Currency  string `json:"Currency"`
				SortOrder int    `json:"SortOrder"`
			} `json:"Fields"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Message != "INLAND_TAX_TEMPLATE_LOADED" {
		t.Fatalf("response = %+v", body)
	}
	if body.Data.Template != "DE" || len(body.Data.Fields) != 3 {
		t.Fatalf("data = %+v, want DE with 3 fields", body.Data)
	}
	if body.Data.Fields[0].Code != "capital_gains_tax" || body.Data.Fields[0].SortOrder != 10 {
		t.Fatalf("first field = %+v, want sorted capital_gains_tax", body.Data.Fields[0])
	}
}

func TestGetInlandTaxTemplateUnknownReturnsNotFound(t *testing.T) {
	ctl, _, _ := newInlandTaxTemplatesTestController(t)

	rec := performInlandTaxTemplatesRequest(t, ctl, http.MethodGet, "/api/inland-tax-templates/XX")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Success || body.Message != "INLAND_TAX_TEMPLATE_NOT_FOUND" {
		t.Fatalf("response = %+v", body)
	}
}
