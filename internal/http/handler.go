package http

import (
	"net/http"
)

func Handler(srv Server) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/sbom/upload", srv.Upload)
	mux.HandleFunc("/api/v1/sbom/{urn}", srv.GetByURN)
	mux.HandleFunc("/api/v1/sbom", srv.Search)

	return mux
}
