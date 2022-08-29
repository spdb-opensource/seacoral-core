package handlerrouter

import "net/http"

// Router defines an interface to specify a group of routes to add to the docker server.
type HandlerRouter interface {
	// Routes returns the list of routes to add to the docker server.
	HandlerRoutes() []HandlerRoute
}

// Route defines an individual API route in the docker server.
type HandlerRoute interface {
	// Handler returns the raw function to create the http handler.
	Handler() http.Handler
	// Path returns the subpath where the route responds to.
	Path() string
}

type Adder interface {
	AddRouter(r HandlerRoute)
}

type localRoute struct {
	path    string
	handler http.Handler
}

func (l localRoute) Handler() http.Handler {
	return l.handler
}

func (l localRoute) Path() string {
	return l.path
}

// NewPostRoute initializes a new route with the http method POST.
func NewHandlerRoute(path string, handler http.Handler) HandlerRoute {
	return localRoute{path, handler}
}
