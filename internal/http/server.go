package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/CZERTAINLY/CBOM-Repository/internal/details"
	"github.com/CZERTAINLY/CBOM-Repository/internal/health"
	"github.com/CZERTAINLY/CBOM-Repository/internal/log"
	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
)

type Server struct {
	service       service.Service
	healthService health.Service
}

// TODO: abstract an interface for unit test mock
func New(svc service.Service, healthSvc health.Service) Server {
	return Server{
		service:       svc,
		healthService: healthSvc,
	}
}

func (h Server) BomHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.Upload(w, r)
	case http.MethodGet:
		h.Search(w, r)
	default:
		// Return Problem Document for unsupported methods
		details.MethodNotAllowed(w,
			fmt.Sprintf("Method %s not allowed for %s.", r.Method, r.URL.Path),
			[]string{http.MethodGet, http.MethodPost})
		return
	}
}

func (h Server) Upload(w http.ResponseWriter, r *http.Request) {
	// Assert http POST
	if r.Method != http.MethodPost {
		details.MethodNotAllowed(w,
			fmt.Sprintf("Method %s not allowed for %s", r.Method, r.URL.Path),
			[]string{http.MethodPost})
		return
	}

	// Assert content type and optional version
	ok, version := CheckContentType(r.Header.Get(HeaderContentType))
	if !ok {
		details.UnsupportedMediaType(w,
			fmt.Sprintf("Content type %s not allowed for %s method %s", r.Header.Get(HeaderContentType), r.URL.Path, r.Method),
			[]string{"application/vnd.cyclonedx+json"})
		return
	}

	if !h.service.VersionSupported(version) {
		details.BadRequest(w,
			fmt.Sprintf("Version %s not supported", version),
			map[string]any{"supported-versions": h.service.SupportedVersion()},
		)
		return
	}
	ctx := log.ContextAttrs(r.Context(), slog.Group(
		"http-handler",
		slog.String("path", r.URL.Path),
		slog.String("method", r.Method),
		slog.String(HeaderContentType, r.Header.Get(HeaderContentType)),
		slog.Int64("content-length", r.ContentLength),
	))

	slog.InfoContext(ctx, "Start.")

	resp, err := h.service.UploadBOM(ctx, r.Body, version)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAlreadyExists):
			details.Conflict(w,
				"Conflict with existing BOM",
				map[string]any{
					"conflict-details": map[string]any{
						"serial-number": resp.SerialNumber,
						"version":       resp.Version,
					},
				})
			return
		case errors.Is(err, service.ErrValidation):
			details.BadRequest(w,
				"Validation of BOM failed.",
				map[string]any{"error": err.Error()},
			)
			return
		}
		details.Internal(w,
			"Upload of BOM failed.",
			map[string]any{
				"error": err.Error(),
			})
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		slog.ErrorContext(ctx, "`json.NewEncoder()` failed", slog.String("error", err.Error()))
		return
	}
	slog.InfoContext(ctx, "Finished.", slog.Group(
		"response",
		slog.String("serialNumber", resp.SerialNumber),
		slog.Int("version", resp.Version),
	))
}

func (h Server) GetByURN(w http.ResponseWriter, r *http.Request) {
	// Assert http GET
	if r.Method != http.MethodGet {
		details.MethodNotAllowed(w,
			fmt.Sprintf("Method %s not allowed for %s.", r.Method, r.URL.Path),
			[]string{http.MethodGet})
		return
	}

	// Extract params
	prefix := RouteBOM + "/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		// this is deliberate and MUST be fixed in internal/http/handler.go
		panic("bad router mapping")
	}

	urn := strings.TrimPrefix(r.URL.Path, prefix)
	if urn == "" {
		details.BadRequest(w,
			"Missing `{urn}` path variable.",
			map[string]any{"example": "GET /api/v1/bom/urn:uuid:<uuid>"},
		)
		return
	}

	version := r.URL.Query().Get("version")

	ctx := log.ContextAttrs(r.Context(), slog.Group(
		"http-handler",
		slog.String("path", r.URL.Path),
		slog.String("method", r.Method),
		slog.String(HeaderContentType, r.Header.Get(HeaderContentType)),
		slog.String("requested-urn", urn),
		slog.String("requested-version", version),
	))

	slog.InfoContext(ctx, "Start.")

	resp, err := h.service.GetBOMByUrn(ctx, urn, version)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			details.NotFound(w, "Requested BOM not found.")
			return
		}
		details.Internal(w,
			"Failed to get the requested BOM.",
			map[string]any{
				"error": err.Error(),
			})
		return
	}

	w.Header().Set("Content-Type", "application/vnd.cyclonedx+json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		slog.ErrorContext(ctx, "`json.NewEncoder()` failed", slog.String("error", err.Error()))
		return
	}
	slog.InfoContext(ctx, "Finished.")
}

func (h Server) Search(w http.ResponseWriter, r *http.Request) {
	// Assert http GET
	if r.Method != http.MethodGet {
		details.MethodNotAllowed(w,
			fmt.Sprintf("Method %s not allowed for %s.", r.Method, r.URL.Path),
			[]string{http.MethodGet})
		return
	}
	after := r.URL.Query().Get("after")

	if strings.TrimSpace(after) == "" {
		details.BadRequest(w,
			"Request validation failed.",
			map[string]any{"errors": []struct {
				Detail string `json:"detail"`
				Param  string `json:"parameter"`
			}{
				{
					Detail: "Query parameter must not be empty.",
					Param:  "after",
				},
			},
			},
		)
		return
	}

	i, err := strconv.ParseInt(after, 10, 64)
	if err != nil || i < 0 {
		details.BadRequest(w,
			"Request validation failed.",
			map[string]any{"errors": []struct {
				Detail string `json:"detail"`
				Param  string `json:"parameter"`
			}{
				{
					Detail: "Query parameter must be a positive integer (unixtime).",
					Param:  "after",
				},
			},
			},
		)
		return
	}

	ctx := log.ContextAttrs(r.Context(), slog.Group(
		"http-handler",
		slog.String("path", r.URL.Path),
		slog.String("method", r.Method),
		slog.String(HeaderContentType, r.Header.Get(HeaderContentType)),
		slog.String("requested-after", after),
	))

	slog.InfoContext(ctx, "Start.")

	resp, err := h.service.Search(ctx, i)
	if err != nil {
		details.Internal(w,
			"Failed to get the requested BOM.",
			map[string]any{
				"error": err.Error(),
			})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		slog.ErrorContext(ctx, "`json.NewEncoder()` failed", slog.String("error", err.Error()))
		return
	}
	slog.InfoContext(ctx, "Finished.", slog.Int("response-count", len(resp)))
}

// HealthHandler handles requests to the /api/v1/health endpoint.
// It returns the overall health status of the service and its components.
// Returns 200 OK if status is UP or DEGRADED, 503 Service Unavailable otherwise.
func (h Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		details.MethodNotAllowed(w,
			fmt.Sprintf("Method %s not allowed for %s.", r.Method, r.URL.Path),
			[]string{http.MethodGet})
		return
	}

	healthStatus := h.healthService.CheckHealth(r.Context())

	statusCode := http.StatusOK
	if healthStatus.Status == health.StatusDown || healthStatus.Status == health.StatusOutOfService {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(healthStatus); err != nil {
		slog.ErrorContext(r.Context(), "`json.NewEncoder()` failed", slog.String("error", err.Error()))
		return
	}
}

// LivenessHandler handles requests to the /api/v1/health/liveness endpoint.
// It returns the liveness status used by Kubernetes to determine if the pod should be restarted.
// Always returns 200 OK with status UP unless the application process is in a failed state.
func (h Server) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		details.MethodNotAllowed(w,
			fmt.Sprintf("Method %s not allowed for %s.", r.Method, r.URL.Path),
			[]string{http.MethodGet})
		return
	}

	healthStatus := h.healthService.CheckLiveness(r.Context())

	statusCode := http.StatusOK
	if healthStatus.Status != health.StatusUp {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(healthStatus); err != nil {
		slog.ErrorContext(r.Context(), "`json.NewEncoder()` failed", slog.String("error", err.Error()))
		return
	}
}

// ReadinessHandler handles requests to the /api/v1/health/readiness endpoint.
// It returns the readiness status used by Kubernetes to determine if the pod can accept traffic.
// Returns 200 OK if all critical components are available, 503 Service Unavailable otherwise.
func (h Server) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		details.MethodNotAllowed(w,
			fmt.Sprintf("Method %s not allowed for %s.", r.Method, r.URL.Path),
			[]string{http.MethodGet})
		return
	}

	healthStatus := h.healthService.CheckReadiness(r.Context())

	statusCode := http.StatusOK
	if healthStatus.Status != health.StatusUp {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(healthStatus); err != nil {
		slog.ErrorContext(r.Context(), "`json.NewEncoder()` failed", slog.String("error", err.Error()))
		return
	}
}
