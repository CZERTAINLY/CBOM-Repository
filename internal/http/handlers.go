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
	"github.com/CZERTAINLY/CBOM-Repository/internal/service"

	"github.com/gorilla/mux"
)

func (h Server) Upload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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

func (s Server) GetByURN(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	urn := vars["urn"]

	if urn == "" {
		details.BadRequest(w,
			"Missing `{urn}` path variable.",
			map[string]any{"example": fmt.Sprintf("GET %s%s", s.cfg.Prefix, RouteBOMByURN)},
		)
		return
	}
	if !service.URNValid(urn) {
		details.BadRequest(w,
			"Invalid `{urn}`.",
			map[string]any{"example": "urn:uuid:<uuid>"},
		)
		return
	}

	version := r.URL.Query().Get("version")

	slog.InfoContext(ctx, "Start.", slog.String("urn", urn), slog.String("version", version))

	resp, err := s.service.GetBOMByUrn(ctx, urn, version)
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
	ctx := r.Context()
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

	slog.InfoContext(ctx, "Start.", slog.String("after", after))

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
