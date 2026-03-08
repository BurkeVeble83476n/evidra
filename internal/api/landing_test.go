package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLandingPage_ServesHTML(t *testing.T) {
	t.Parallel()

	uiFS := fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<html><body>Evidra</body></html>"),
		},
	}

	cfg := RouterConfig{
		APIKey:        "test-key",
		DefaultTenant: "default",
		UIFS:          uiFS,
	}
	handler := NewRouter(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "Evidra") {
		t.Error("landing page should contain 'Evidra'")
	}
}

func TestOpenAPISpec_Served(t *testing.T) {
	t.Parallel()

	uiFS := fstest.MapFS{
		"openapi.yaml": &fstest.MapFile{
			Data: []byte("openapi: 3.0.3\ninfo:\n  title: Evidra API"),
		},
	}

	cfg := RouterConfig{
		APIKey:        "test-key",
		DefaultTenant: "default",
		UIFS:          uiFS,
	}
	handler := NewRouter(cfg)

	req := httptest.NewRequest("GET", "/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "openapi: 3.0.3") {
		t.Error("openapi.yaml should be served")
	}
}

func TestSwaggerUI_Served(t *testing.T) {
	t.Parallel()

	uiFS := fstest.MapFS{
		"docs/api/index.html": &fstest.MapFile{
			Data: []byte("<html>swagger-ui</html>"),
		},
	}

	cfg := RouterConfig{
		APIKey:        "test-key",
		DefaultTenant: "default",
		UIFS:          uiFS,
	}
	handler := NewRouter(cfg)

	req := httptest.NewRequest("GET", "/docs/api/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "swagger-ui") {
		t.Error("swagger UI page should be served")
	}
}
