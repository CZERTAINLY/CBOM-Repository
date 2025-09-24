package http

import (
	"fmt"
	"net/http"

	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
)

type Handler struct {
	service service.Service
}

func New(svc service.Service) Handler {
	return Handler{
		service: svc,
	}
}

func (h Handler) Upload(w http.ResponseWriter, r *http.Request) {
	// HTTP Method assertion
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write(fmt.Appendf(nil, "Allowed HTTP methods: %q", http.MethodPost))
		return
	}

	err := h.service.Upload(r.Context(), r.Body)
	defer r.Body.Close()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(fmt.Appendf(nil, "Upload failed: %q", err))
		return
	}

	w.WriteHeader(http.StatusOK)
}
