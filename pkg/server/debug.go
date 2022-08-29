package server

import (
	"context"
	"expvar"
	"net/http"
	"net/http/pprof"

	"github.com/upmio/dbscale-kube/pkg/server/router"
)

func registerDebugRouter(adder router.Adder) {
	debug := NewDebugRouter()

	adder.AddRouter(debug)
}

// NewDebugRouter creates a new debug router
// The debug router holds endpoints for debug the daemon, such as those for pprof.
func NewDebugRouter() router.Router {
	r := &debugRouter{}
	r.initRoutes()
	return r
}

type debugRouter struct {
	routes []router.Route
}

func (r *debugRouter) initRoutes() {
	r.routes = []router.Route{
		router.NewGetRoute("/debug/vars", frameworkAdaptHandler(expvar.Handler())),
		router.NewGetRoute("/debug/pprof/", frameworkAdaptHandlerFunc(pprof.Index)),
		router.NewGetRoute("/debug/pprof/cmdline", frameworkAdaptHandlerFunc(pprof.Cmdline)),
		router.NewGetRoute("/debug/pprof/profile", frameworkAdaptHandlerFunc(pprof.Profile)),
		router.NewGetRoute("/debug/pprof/symbol", frameworkAdaptHandlerFunc(pprof.Symbol)),
		router.NewGetRoute("/debug/pprof/trace", frameworkAdaptHandlerFunc(pprof.Trace)),
		router.NewGetRoute("/debug/pprof/{name}", handlePprof),
	}
}

func (r *debugRouter) Routes() []router.Route {
	return r.routes
}

func frameworkAdaptHandler(handler http.Handler) router.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
		handler.ServeHTTP(w, r)
		return 0, nil, nil
	}
}

func frameworkAdaptHandlerFunc(handler http.HandlerFunc) router.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
		handler(w, r)
		return 0, nil, nil
	}
}

func handlePprof(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	pprof.Handler(vars["name"]).ServeHTTP(w, r)
	return 0, nil, nil
}
