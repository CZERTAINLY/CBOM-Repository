package http

import (
	"net/http"
)

func Handler(srv Server) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc(RouteBOM, srv.BomHandler)
	mux.HandleFunc(RouteBOMByURN, srv.GetByURN)
	mux.HandleFunc(RouteHealth, srv.HealthHandler)
	mux.HandleFunc(RouteHealthLive, srv.LivenessHandler)
	mux.HandleFunc(RouteHealthReady, srv.ReadinessHandler)

	return mux
}
