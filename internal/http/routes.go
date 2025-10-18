package http

const (
	APIPrefix          = "/api/v1"
	RouteBOM           = APIPrefix + "/bom"
	RouteBOMByURN      = RouteBOM + "/{urn}"
	RouteHealth        = APIPrefix + "/health"
	RouteHealthLive    = RouteHealth + "/liveness"
	RouteHealthReady   = RouteHealth + "/readiness"
)
