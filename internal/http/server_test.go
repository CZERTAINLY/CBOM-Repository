package http_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CZERTAINLY/CBOM-Repository/internal/health"
	httpserver "github.com/CZERTAINLY/CBOM-Repository/internal/http"
	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"
	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChecker is a mock implementation of the health.Checker interface used by health.NewService
type mockChecker struct {
	name    string
	status  health.Status
	details map[string]any
}

func (m mockChecker) Name() string { return m.name }
func (m mockChecker) Check(ctx context.Context) health.Component {
	return health.Component{Status: m.status, Details: m.details}
}

func buildBOMReader(t *testing.T, withSerial bool, serial string, version int) io.ReadCloser {
	t.Helper()
	bom := cdx.BOM{BOMFormat: cdx.BOMFormat, SpecVersion: cdx.SpecVersion1_6}
	if withSerial {
		bom.SerialNumber = serial
	}
	if version > 0 {
		bom.Version = version
	}
	var sb strings.Builder
	enc := cdx.NewBOMEncoder(&sb, cdx.BOMFileFormatJSON)
	if err := enc.Encode(&bom); err != nil {
		require.NoError(t, err)
	}
	return io.NopCloser(strings.NewReader(sb.String()))
}

func TestHealthHandlers(t *testing.T) {
	storageChecker := mockChecker{name: "storage", status: health.StatusUp, details: map[string]any{"latencyMs": 1}}
	healthSvc := health.NewService(storageChecker)

	svc := service.Service{}
	srv := httpserver.New(svc, healthSvc)

	t.Run("health_ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		w := httptest.NewRecorder()
		srv.HealthHandler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	})

	t.Run("liveness_ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health/liveness", nil)
		w := httptest.NewRecorder()
		srv.LivenessHandler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("readiness_ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health/readiness", nil)
		w := httptest.NewRecorder()
		srv.ReadinessHandler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestBomHandler_MethodNotAllowed(t *testing.T) {
	srv := httpserver.New(service.Service{}, health.Service{})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/bom", nil)
	w := httptest.NewRecorder()
	srv.BomHandler(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestUploadHandler_Validation(t *testing.T) {
	// unsupported media type
	srv := httpserver.New(service.Service{}, health.Service{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bom", nil)
	req.Header.Set("content-type", "text/plain")
	w := httptest.NewRecorder()
	srv.Upload(w, req)
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)

	// version unsupported - create service via service.New using a minimal store (no S3 clients required for this test)
	st := store.New(store.Config{Bucket: "bucket"}, nil, nil)
	svc, err := service.New(st)
	require.NoError(t, err)
	srv = httpserver.New(svc, health.Service{})

	// request uses version=1.4 in media type
	req = httptest.NewRequest(http.MethodPost, "/api/v1/bom", buildBOMReader(t, true, "urn:uuid:550e8400-e29b-11d4-a716-446655440000", 1))
	req.Header.Set("content-type", "application/vnd.cyclonedx+json;version=1.4")
	w = httptest.NewRecorder()
	srv.Upload(w, req)
	// expects BadRequest because version not supported
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetByURNHandler_MissingURN(t *testing.T) {
	srv := httpserver.New(service.Service{}, health.Service{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bom/", nil)
	w := httptest.NewRecorder()
	srv.GetByURN(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchHandler_Validation(t *testing.T) {
	srv := httpserver.New(service.Service{}, health.Service{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bom/search", nil)
	w := httptest.NewRecorder()
	srv.Search(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// after not a number
	req = httptest.NewRequest(http.MethodGet, "/api/v1/bom/search?after=notanint", nil)
	w = httptest.NewRecorder()
	srv.Search(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Additional small smoke test to ensure Handler wiring returns a mux
func TestHandler_Wiring(t *testing.T) {
	svc := service.Service{}
	healthSvc := health.NewService()
	srv := httpserver.New(svc, healthSvc)
	mux := httpserver.Handler(srv)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}
