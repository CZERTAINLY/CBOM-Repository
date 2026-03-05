package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/CZERTAINLY/CBOM-Repository/internal/details"
	"github.com/CZERTAINLY/CBOM-Repository/internal/health"
	"github.com/CZERTAINLY/CBOM-Repository/internal/log"
	"github.com/CZERTAINLY/CBOM-Repository/internal/service"

	"github.com/gorilla/mux"
)

const (
	V1Prefix         = "/v1"
	RouteBOM         = V1Prefix + "/bom"
	RouteBOMByURN    = RouteBOM + "/{urn}"
	RouteBOMVersions = RouteBOMByURN + "/versions"
	RouteHealth      = V1Prefix + "/health"
	RouteHealthLive  = RouteHealth + "/liveness"
	RouteHealthReady = RouteHealth + "/readiness"
)

type Config struct {
	Port        int    `envconfig:"APP_HTTP_PORT" default:"8080"`
	Prefix      string `envconfig:"APP_HTTP_PREFIX" default:"/api"`
	MaxBodySize int64  `envconfig:"APP_HTTP_MAX_BODY_SIZE" default:"10485760"` // default to 10MB
}

type Server struct {
	cfg           Config
	service       service.Service
	healthService health.Service
}

func New(cfg Config, svc service.Service, healthSvc health.Service) Server {
	cfg.Prefix = strings.TrimSuffix(cfg.Prefix, "/")
	if len(cfg.Prefix) != 0 && cfg.Prefix[0] != '/' {
		cfg.Prefix = fmt.Sprintf("/%s", cfg.Prefix)
	}

	return Server{
		cfg:           cfg,
		service:       svc,
		healthService: healthSvc,
	}
}

func (s *Server) Handler() *mux.Router {
	r := mux.NewRouter()

	r.Use(httpInfoContext)
	r.Use(MaxBodySizeMiddleware(s.cfg.MaxBodySize))

	uploadRouter := r.Methods(http.MethodPost).Subrouter()
	uploadRouter.Use(s.BOMValidationMiddleware)
	uploadRouter.HandleFunc(fmt.Sprintf("%s%s", s.cfg.Prefix, RouteBOM), s.Upload)

	r.HandleFunc(fmt.Sprintf("%s%s", s.cfg.Prefix, RouteBOM), s.Search).Methods(http.MethodGet)
	r.HandleFunc(fmt.Sprintf("%s%s", s.cfg.Prefix, RouteBOMByURN), s.GetByURN).Methods(http.MethodGet)
	r.HandleFunc(fmt.Sprintf("%s%s", s.cfg.Prefix, RouteBOMVersions), s.URNVersions).Methods(http.MethodGet)
	r.HandleFunc(fmt.Sprintf("%s%s", s.cfg.Prefix, RouteHealth), s.HealthHandler).Methods(http.MethodGet)
	r.HandleFunc(fmt.Sprintf("%s%s", s.cfg.Prefix, RouteHealthLive), s.LivenessHandler).Methods(http.MethodGet)
	r.HandleFunc(fmt.Sprintf("%s%s", s.cfg.Prefix, RouteHealthReady), s.ReadinessHandler).Methods(http.MethodGet)

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("Received an HTTP request for an unmapped path and method.",
			slog.String("path", r.URL.Path), slog.String("method", r.Method))
		details.NotFound(w,
			fmt.Sprintf("There is no handler registered for path: %s, method: %s",
				r.URL.Path, r.Method,
			))
	})

	return r
}

// HealthHandler handles requests to the /api/v1/health endpoint.
// It returns the overall health status of the service and its components.
// Returns 200 OK if status is UP or DEGRADED, 503 Service Unavailable otherwise.
func (h Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
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

func httpInfoContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Add structured HTTP attributes to context
		ctx := log.ContextAttrs(r.Context(), slog.Group("http-info",
			slog.String("method", r.Method),
			slog.String("url-path", r.URL.Path),
		))

		// Pass updated request into chain
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// MaxBodySizeMiddleware limits the size of the request body to maxBytes.
func MaxBodySizeMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wrap the request body with a MaxBytesReader only when maxBytes is positive.
			// Non-positive values are treated as "no limit" to avoid failing all reads.
			if maxBytes > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}

			// Continue to the next handler
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) BOMValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert content type and optional version
		contentType := r.Header.Get(HeaderContentType)
		ok, version := CheckContentType(contentType)
		if !ok {
			details.UnsupportedMediaType(w,
				fmt.Sprintf("Content type %s not allowed for %s method %s", contentType, r.URL.Path, r.Method),
				[]string{"application/vnd.cyclonedx+json"})
			return
		}

		if !s.service.VersionSupported(version) {
			details.BadRequest(w,
				fmt.Sprintf("Version %s not supported", version),
				map[string]any{"supported-versions": s.service.SupportedVersion()},
			)
			return
		}

		r.Header.Set("X-BOM-Version", version)
		next.ServeHTTP(w, r)
	})
}
