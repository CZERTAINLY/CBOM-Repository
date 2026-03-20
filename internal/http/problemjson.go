package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const (
	problemJsonAboutBlankType = "about:blank"
)

func template(detail string, statusCode int) problem {
	t := time.Now()
	p := problem{
		Type:      problemJsonAboutBlankType,
		Title:     http.StatusText(statusCode),
		Status:    statusCode,
		Detail:    detail,
		Timestamp: t.Format(time.RFC3339),
	}
	return p
}

func badrequest(w http.ResponseWriter, detail string) {
	p := template(detail, http.StatusBadRequest)
	p.Json(w)
}

func internal(w http.ResponseWriter, detail string) {
	p := template(detail, http.StatusInternalServerError)
	p.Json(w)
}

func notfound(w http.ResponseWriter, detail string) {
	p := template(detail, http.StatusNotFound)
	p.Json(w)
}

func conflict(w http.ResponseWriter, detail string) {
	p := template(detail, http.StatusConflict)
	p.Json(w)
}

func unsupportedMediaType(w http.ResponseWriter, detail string) {
	p := template(detail, http.StatusUnsupportedMediaType)
	p.Json(w)
}

func requestTooLarge(w http.ResponseWriter, detail string) {
	p := template(detail, http.StatusRequestEntityTooLarge)
	p.Json(w)
}

type problem struct {
	Type      string `json:"type,omitempty"`
	Title     string `json:"title,omitempty"`
	Status    int    `json:"status,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

func (p problem) Json(w http.ResponseWriter) {
	var err error
	var b []byte
	if b, err = json.Marshal(p); err != nil {
		// this shouldn't happen as we control the annotation of problem struct
		slog.Error("Failed to marshal problem struct to json",
			slog.String("error", err.Error()), slog.Any("struct", p))
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/problem+json")
	if p.Status > 0 {
		w.WriteHeader(p.Status)
	}
	_, _ = w.Write(b)
}
