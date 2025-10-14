package http

import (
	"net/http"
)

func Handler(srv Server) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc(RouteBOM, srv.BomHandler)
	mux.HandleFunc(RouteBOMByURN, srv.GetByURN)

	return mux
}
