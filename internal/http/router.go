package http

import (
	"net/http"
)

func Router(h Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST api/v1/upload", h.Upload)
	return mux
}
