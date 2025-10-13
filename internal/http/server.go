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
	"github.com/CZERTAINLY/CBOM-Repository/internal/log"
	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
)

type Server struct {
	service service.Service
}

// TODO: abstract an interface for unit test mock
func New(svc service.Service) Server {
	return Server{
		service: svc,
	}
}

func (h Server) Upload(w http.ResponseWriter, r *http.Request) {
	// Assert http POST
	if r.Method != http.MethodPost {
		details.MethodNotAllowed(w,
			fmt.Sprintf("Method %s not allowed for %s.", r.Method, r.URL.Path),
			[]string{http.MethodPost})
		return
	}

	// Assert content type and optional version
	ok, version := CheckContentType(r.Header.Get("content-type"))
	if !ok {
		details.UnsupportedMediaType(w,
			fmt.Sprintf("Content type %s not allowed for %s.", r.Method, r.URL.Path),
			[]string{"application/vnd.cyclonedx+json"})
		return
	}

	if !h.service.VersionSupported(version) {
		details.BadRequest(w,
			fmt.Sprintf("Version %s not supported.", version),
			map[string]any{"supported-versions": h.service.SupportedVersion()},
		)
		return
	}
	ctx := log.ContextAttrs(r.Context(), slog.Group(
		"http-handler",
		slog.String("path", r.URL.Path),
		slog.String("method", r.Method),
		slog.String("content-type", r.Header.Get("content-type")),
		slog.Int64("content-length", r.ContentLength),
	))

	slog.InfoContext(ctx, "Start.")

	resp, err := h.service.UploadSBOM(ctx, r.Body, version)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAlreadyExists):
			details.Conflict(w,
				"Conflict with existing SBOM",
				map[string]any{
					"conflict-details": map[string]any{
						"serial-number": resp.SerialNumber,
						"version":       resp.Version,
					},
				})
			return
		case errors.Is(err, service.ErrValidation):
			details.BadRequest(w,
				"Validation of SBOM failed.",
				map[string]any{"error": err.Error()},
			)
			return
		}
		details.Internal(w,
			"Upload of SBOM failed.",
			map[string]any{
				"error": err.Error(),
			})
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
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
	prefix := "/api/v1/sbom/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		// this is deliberate and MUST be fixed in internal/http/handler.go
		panic("bad router mapping")
	}

	urn := strings.TrimPrefix(r.URL.Path, prefix)
	if urn == "" {
		details.BadRequest(w,
			"Missing `{urn}` path variable.",
			map[string]any{"example": "GET /api/v1/sbom/urn:uuid:<uuid>"},
		)
		return
	}

	version := r.URL.Query().Get("version")

	ctx := log.ContextAttrs(r.Context(), slog.Group(
		"http-handler",
		slog.String("path", r.URL.Path),
		slog.String("method", r.Method),
		slog.String("content-type", r.Header.Get("content-type")),
		slog.String("requested-urn", urn),
		slog.String("requested-version", version),
	))

	slog.InfoContext(ctx, "Start.")

	resp, err := h.service.GetSBOMByUrn(ctx, urn, version)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			details.NotFound(w, "Requested SBOM not found.")
			return
		}
		details.Internal(w,
			"Failed to get the requested SBOM.",
			map[string]any{
				"error": err.Error(),
			})
		return
	}

	w.Header().Set("Content-Type", "application/vnd.cyclonedx+json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
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
		slog.String("content-type", r.Header.Get("content-type")),
		slog.String("requested-after", after),
	))

	slog.InfoContext(ctx, "Start.")

	resp, err := h.service.Search(ctx, i)
	if err != nil {
		details.Internal(w,
			"Failed to get the requested SBOM.",
			map[string]any{
				"error": err.Error(),
			})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
	slog.InfoContext(ctx, "Finished.", slog.Int("response-count", len(resp)))
}
