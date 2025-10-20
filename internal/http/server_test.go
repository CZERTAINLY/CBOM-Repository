package http_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/CZERTAINLY/CBOM-Repository/internal/health"
	httpserver "github.com/CZERTAINLY/CBOM-Repository/internal/http"
	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"
	mockS3 "github.com/CZERTAINLY/CBOM-Repository/internal/store/mock"
	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

func TestHealthHandler_ServiceUnavailable(t *testing.T) {
	// Override readiness to OutOfService to force 503
	healthSvc := health.NewService(mockChecker{name: "readiness", status: health.StatusOutOfService})
	srv := httpserver.New(service.Service{}, healthSvc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	srv.HealthHandler(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestReadinessHandler_ServiceUnavailable(t *testing.T) {
	// Any checker DOWN causes readiness status != UP => 503
	healthSvc := health.NewService(mockChecker{name: "storage", status: health.StatusDown})
	srv := httpserver.New(service.Service{}, healthSvc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/readiness", nil)
	w := httptest.NewRecorder()
	srv.ReadinessHandler(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestBomHandler_MethodNotAllowed(t *testing.T) {
	srv := httpserver.New(service.Service{}, health.Service{})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/bom", nil)
	w := httptest.NewRecorder()
	srv.BomHandler(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestBomHandler_DispatchGetSearchSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s3Mock := mockS3.NewMockS3Contract(ctrl)
	st := store.New(store.Config{Bucket: "bucket"}, s3Mock, nil)
	svc, err := service.New(st)
	require.NoError(t, err)
	srv := httpserver.New(svc, health.NewService())

	now := time.Now()
	s3Mock.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{Key: awsString("urn:uuid:1-1"), LastModified: &now},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bom?after="+strconv.FormatInt(now.Unix()-1, 10), nil)
	w := httptest.NewRecorder()
	srv.BomHandler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUploadHandler_Validation(t *testing.T) {
	// unsupported media type
	srv := httpserver.New(service.Service{}, health.Service{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bom", nil)
	req.Header.Set(httpserver.HeaderContentType, "text/plain")
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
	req.Header.Set(httpserver.HeaderContentType, "application/vnd.cyclonedx+json;version=1.4")
	w = httptest.NewRecorder()
	srv.Upload(w, req)
	// expects BadRequest because version not supported
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadHandler_ValidationFailed(t *testing.T) {
	// Build real service (schema available), but body violates schema (extra prop)
	st := store.New(store.Config{Bucket: "bucket"}, nil, nil)
	svc, err := service.New(st)
	require.NoError(t, err)
	srv := httpserver.New(svc, health.NewService())

	body := io.NopCloser(strings.NewReader("{\n  \"bomFormat\": \"CycloneDX\",\n  \"specVersion\": \"1.6\",\n  \"extra\": \"x\"\n}"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bom", body)
	req.Header.Set(httpserver.HeaderContentType, "application/vnd.cyclonedx+json")
	w := httptest.NewRecorder()
	srv.Upload(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetByURNHandler_MissingURN(t *testing.T) {
	srv := httpserver.New(service.Service{}, health.Service{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bom/", nil)
	w := httptest.NewRecorder()
	srv.GetByURN(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetByURNHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s3Mock := mockS3.NewMockS3Contract(ctrl)
	st := store.New(store.Config{Bucket: "bucket"}, s3Mock, nil)
	svc, err := service.New(st)
	require.NoError(t, err)
	srv := httpserver.New(svc, health.NewService())

	// With explicit version, service will call GetObject which returns NoSuchKey
	s3Mock.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).Return((*s3.GetObjectOutput)(nil), &types.NoSuchKey{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bom/urn:uuid:123?version=1", nil)
	w := httptest.NewRecorder()
	srv.GetByURN(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetByURNHandler_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s3Mock := mockS3.NewMockS3Contract(ctrl)
	st := store.New(store.Config{Bucket: "bucket"}, s3Mock, nil)
	svc, err := service.New(st)
	require.NoError(t, err)
	srv := httpserver.New(svc, health.NewService())

	// Return unexpected error from GetObject
	s3Mock.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).Return((*s3.GetObjectOutput)(nil), assert.AnError)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bom/urn:uuid:123?version=1", nil)
	w := httptest.NewRecorder()
	srv.GetByURN(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
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

func TestSearchHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s3Mock := mockS3.NewMockS3Contract(ctrl)
	st := store.New(store.Config{Bucket: "bucket"}, s3Mock, nil)
	svc, err := service.New(st)
	require.NoError(t, err)
	srv := httpserver.New(svc, health.NewService())

	now := time.Now()
	s3Mock.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{Key: awsString("urn:uuid:1-1"), LastModified: &now},
			{Key: awsString("urn:uuid:2-2"), LastModified: &now},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bom/search?after="+strconv.FormatInt(now.Unix()-1, 10), nil)
	w := httptest.NewRecorder()
	srv.Search(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
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

func TestUploadHandler_SuccessCreated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s3Mock := mockS3.NewMockS3Contract(ctrl)
	s3Manager := mockS3.NewMockS3Manager(ctrl)

	st := store.New(store.Config{Bucket: "bucket"}, s3Mock, s3Manager)
	svc, err := service.New(st)
	require.NoError(t, err)
	srv := httpserver.New(svc, health.NewService())

	// HeadObject returns NotFound so generated serial won't conflict
	s3Mock.EXPECT().HeadObject(gomock.Any(), gomock.Any()).Return((*s3.HeadObjectOutput)(nil), &types.NotFound{}).AnyTimes()
	// Upload called twice: original and modified
	s3Manager.EXPECT().Upload(gomock.Any(), gomock.Any()).Return(&manager.UploadOutput{}, nil).Times(2)

	// minimal BOM without serial -> upload should create and return 201
	body := io.NopCloser(strings.NewReader("{\n  \"bomFormat\": \"CycloneDX\",\n  \"specVersion\": \"1.6\"\n}"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bom", body)
	req.Header.Set(httpserver.HeaderContentType, "application/vnd.cyclonedx+json")
	w := httptest.NewRecorder()
	srv.Upload(w, req)
	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestUploadHandler_ConflictAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s3Mock := mockS3.NewMockS3Contract(ctrl)
	s3Manager := mockS3.NewMockS3Manager(ctrl)

	st := store.New(store.Config{Bucket: "bucket"}, s3Mock, s3Manager)
	svc, err := service.New(st)
	require.NoError(t, err)
	srv := httpserver.New(svc, health.NewService())

	// HeadObject returns nil -> key exists
	s3Mock.EXPECT().HeadObject(gomock.Any(), gomock.Any()).Return(&s3.HeadObjectOutput{}, nil)

	serial := "urn:uuid:550e8400-e29b-11d4-a716-446655440000"
	body := io.NopCloser(strings.NewReader("{\n  \"bomFormat\": \"CycloneDX\",\n  \"specVersion\": \"1.6\",\n  \"serialNumber\": \"" + serial + "\",\n  \"version\": 2\n}"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bom", body)
	req.Header.Set(httpserver.HeaderContentType, "application/vnd.cyclonedx+json")
	w := httptest.NewRecorder()
	srv.Upload(w, req)
	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestUpload_MethodNotAllowed(t *testing.T) {
	srv := httpserver.New(service.Service{}, health.NewService())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bom", nil)
	srv.Upload(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestGetByURN_MethodNotAllowed(t *testing.T) {
	srv := httpserver.New(service.Service{}, health.NewService())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bom/urn:uuid:123", nil)
	srv.GetByURN(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHealthEndpoints_MethodNotAllowed(t *testing.T) {
	srv := httpserver.New(service.Service{}, health.NewService())
	// Health
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
	srv.HealthHandler(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	// Liveness
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/health/liveness", nil)
	srv.LivenessHandler(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	// Readiness
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/health/readiness", nil)
	srv.ReadinessHandler(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestGetByURNHandler_Success_HTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s3Mock := mockS3.NewMockS3Contract(ctrl)
	st := store.New(store.Config{Bucket: "bucket"}, s3Mock, nil)
	svc, err := service.New(st)
	require.NoError(t, err)
	srv := httpserver.New(svc, health.NewService())

	s3Mock.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("{\"bomFormat\":\"CycloneDX\"}"))}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bom/urn:uuid:123?version=1", nil)
	w := httptest.NewRecorder()
	srv.GetByURN(w, req)
	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/vnd.cyclonedx+json", resp.Header.Get("Content-Type"))
}

// helper to create *string for aws types in this package
func awsString(s string) *string { return &s }
