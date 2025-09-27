package http

import (
	"fmt"
	"io"
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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			_ = fmt.Errorf("failed to close request body: %v", err)
		}
	}(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(fmt.Appendf(nil, "Upload failed: %q", err))
		return
	}

	w.WriteHeader(http.StatusOK)
}
