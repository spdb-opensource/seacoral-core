package router

import (
	"context"
	"net/http"
)

// Router defines an interface to specify a group of routes to add to the docker server.
type Router interface {
	// Routes returns the list of routes to add to the docker server.
	Routes() []Route
}

// Route defines an individual API route in the docker server.
type Route interface {
	// Handler returns the raw function to create the http handler.
	Handler() HandlerFunc
	// Method returns the http method that the route responds to.
	Method() string
	// Path returns the subpath where the route responds to.
	Path() string
}

type HandlerFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (code int, out interface{}, err error)

// RouteWrapper wraps a route with extra functionality.
// It is passed in when creating a new route.
type RouteWrapper func(r Route) Route

type Adder interface {
	AddRouter(r Router)
}

// localRoute defines an individual API route to connect
// with the docker daemon. It implements Route.
type localRoute struct {
	method  string
	path    string
	handler HandlerFunc
}

// Handler returns the HandlerFunc to let the server wrap it in middlewares.
func (l localRoute) Handler() HandlerFunc {
	return l.handler
}

// Method returns the http method that the route responds to.
func (l localRoute) Method() string {
	return l.method
}

// Path returns the subpath where the route responds to.
func (l localRoute) Path() string {
	return l.path
}

// NewRoute initializes a new local route for the router.
func NewRoute(method, path string, handler HandlerFunc, opts ...RouteWrapper) Route {
	var r Route = localRoute{method, path, handler}
	for _, o := range opts {
		r = o(r)
	}
	return r
}

// NewGetRoute initializes a new route with the http method GET.
func NewGetRoute(path string, handler HandlerFunc, opts ...RouteWrapper) Route {
	return NewRoute("GET", path, handler, opts...)
}

// NewPostRoute initializes a new route with the http method POST.
func NewPostRoute(path string, handler HandlerFunc, opts ...RouteWrapper) Route {
	return NewRoute("POST", path, handler, opts...)
}

// NewPutRoute initializes a new route with the http method PUT.
func NewPutRoute(path string, handler HandlerFunc, opts ...RouteWrapper) Route {
	return NewRoute("PUT", path, handler, opts...)
}

// NewDeleteRoute initializes a new route with the http method DELETE.
func NewDeleteRoute(path string, handler HandlerFunc, opts ...RouteWrapper) Route {
	return NewRoute("DELETE", path, handler, opts...)
}

// NewOptionsRoute initializes a new route with the http method OPTIONS.
func NewOptionsRoute(path string, handler HandlerFunc, opts ...RouteWrapper) Route {
	return NewRoute("OPTIONS", path, handler, opts...)
}

// NewHeadRoute initializes a new route with the http method HEAD.
func NewHeadRoute(path string, handler HandlerFunc, opts ...RouteWrapper) Route {
	return NewRoute("HEAD", path, handler, opts...)
}
