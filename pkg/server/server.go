package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/didip/tollbooth/v6"
	"github.com/upmio/dbscale-kube/pkg/server/handlerrouter"
	"net"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/gorilla/mux"

	"github.com/upmio/dbscale-kube/pkg/server/middleware"
	"github.com/upmio/dbscale-kube/pkg/server/router"
	"github.com/upmio/dbscale-kube/pkg/vars"
)

// versionMatcher defines a variable matcher to be parsed by the router
// when a request is about to be served.
const versionMatcher = "/v{version:[0-9.]+}"

type Addr struct {
	Proto string // "tcp", "tcp4", "tcp6", "unix" or "unixpacket"
	Addr  string
}

type encoder interface {
	Encode(w http.ResponseWriter, code int, v interface{}) error
}

type jsonEncoder struct{}

func (e jsonEncoder) Encode(w http.ResponseWriter, code int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	return json.NewEncoder(w).Encode(v)
}

type Server struct {
	hosts       []Addr
	servers     []*_HTTPServer
	routers     []router.Router
	rawRouters  []handlerrouter.HandlerRouter
	middlewares []middleware.Middleware
	handler     http.Handler
	encoder     encoder
	tlsConfig   *tls.Config
}

func NewServer(hosts []Addr, tlsConfig *tls.Config) *Server {
	return &Server{
		hosts:     hosts,
		servers:   make([]*_HTTPServer, len(hosts)),
		tlsConfig: tlsConfig,
		encoder:   jsonEncoder{},
	}
}

func (srv *Server) SetEncoder(er encoder) {
	srv.encoder = er
}

func (srv *Server) AddMiddleware(mw middleware.Middleware) {
	srv.middlewares = append(srv.middlewares, mw)
}

func (srv *Server) AddRouter(r router.Router) {
	srv.routers = append(srv.routers, r)
}

func (srv *Server) AddRawRouter(r handlerrouter.HandlerRouter) {
	srv.rawRouters = append(srv.rawRouters, r)
}

func (srv *Server) Shutdown() {
	for i := range srv.servers {
		if srv.servers[i] != nil {
			srv.servers[i].Shutdown()
		}
	}
}

func (srv *Server) createMux() {
	m := mux.NewRouter()

	registerDebugRouter(srv)

	lmt := tollbooth.NewLimiter(20, nil)
	lmt.SetTokenBucketExpirationTTL(time.Minute * 30)

	for _, apiRouter := range srv.routers {
		for _, r := range apiRouter.Routes() {
			f := srv.makeHTTPHandler(r.Handler())

			m.Path(versionMatcher + r.Path()).Methods(r.Method()).Handler(tollbooth.LimitFuncHandler(lmt, f))
			m.Path(r.Path()).Methods(r.Method()).Handler(tollbooth.LimitFuncHandler(lmt, f))

			klog.Infof("Router %s %s", r.Method(), versionMatcher+r.Path())
		}
	}

	for _, wsRouter := range srv.rawRouters {
		for _, r := range wsRouter.HandlerRoutes() {
			m.PathPrefix(r.Path()).Handler(r.Handler())
			klog.Infof("Raw (WS or Restful) Router %s", r.Path())
		}
	}

	srv.handler = m
}

func (srv *Server) makeHTTPHandler(handler router.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The 'context' will be used for global data that should
		// apply to all requests. Data that is specific to the
		// immediate function being called should still be passed
		// as 'args' on the function call.

		ctx := r.Context()
		handlerFunc := srv.handlerWithGlobalMiddlewares(handler)

		vars := mux.Vars(r)
		if vars == nil {
			vars = make(map[string]string)
		}

		code, out, err := handlerFunc(ctx, w, r, vars)
		if err != nil {
			srv.encoder.Encode(w, code, ErrorResponse{
				Code:  code,
				Error: err.Error(),
			})
			return
		}

		if out != nil {
			srv.encoder.Encode(w, code, out)
			return
		}

		w.WriteHeader(code)
	}
}

type ErrorResponse struct {
	Code  int    `json:"code"`
	Error string `json:"msg"`
}

// handlerWithGlobalMiddlewares wraps the handler function for a request with
// the server's global middlewares. The order of the middlewares is backwards,
// meaning that the first in the list will be evaluated last.
func (srv *Server) handlerWithGlobalMiddlewares(handler router.HandlerFunc) router.HandlerFunc {
	next := handler

	for _, m := range srv.middlewares {
		next = m.WrapHandler(next)
	}

	return next
}

// ListenAndServe starts an HTTP server on each host to listen on its
// TCP or Unix network address and calls Serve on each host's server
// to handle requests on incoming connections.
//
// The expected format for a host string is [protocol://]address. The protocol
// must be either "tcp" or "unix", with "tcp" used by default if not specified.
func (srv *Server) ListenAndServe() error {
	// init handler
	srv.createMux()

	chErrors := make(chan error, len(srv.hosts))

	for i, host := range srv.hosts {
		if host.Proto == "" {
			srv.hosts[i].Proto = "tcp"
		}

		var (
			l      net.Listener
			err    error
			server = &http.Server{
				Addr:    host.Addr,
				Handler: srv.handler,
			}
		)

		switch host.Proto {
		case "unix":
			l, err = newUnixListener(host.Addr, srv.tlsConfig)
		case "tcp":
			l, err = newListener("tcp", host.Addr, srv.tlsConfig)
		default:
			err = fmt.Errorf("unsupported protocol: %q", host.Proto)
		}
		if err != nil {
			return err
		}

		srv.servers[i] = &_HTTPServer{
			server:   server,
			listener: l,
		}
	}

	for i := range srv.servers {
		go func(server *_HTTPServer) {
			chErrors <- server.Serve()
		}(srv.servers[i])

		klog.Info(vars.Banner)
		klog.Infof("Server %s start listen and server...", srv.servers[i].server.Addr)
	}

	timeout := time.Second * 10 * time.Duration(len(srv.servers))
	ch := time.After(timeout)

	for {
		select {
		case err := <-chErrors:
			if err != nil {
				return err
			}
		case <-ch:
			return nil
		}
	}
}

func newListener(proto, addr string, tlsConfig *tls.Config) (net.Listener, error) {
	l, err := net.Listen(proto, addr)
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			return nil, fmt.Errorf("%s: Try using a different port", err)
		}
		return nil, err
	}
	if tlsConfig != nil {
		tlsConfig.NextProtos = []string{"http/1.1"}
		l = tls.NewListener(l, tlsConfig)
	}
	return l, nil
}

// _HTTPServer contains an instance of http server and the listener.
// srv *http.Server, contains configuration to create an http server and a mux router with all api end points.
// l   net.Listener, is a TCP or Socket listener that dispatches incoming request to the router.
type _HTTPServer struct {
	server   *http.Server
	listener net.Listener
}

// Serve starts listening for inbound requests.
func (s *_HTTPServer) Serve() error {
	return s.server.Serve(s.listener)
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
// Shutdown closes the _HTTPServer from listening for the inbound requests.
func (s *_HTTPServer) Shutdown() error {
	return s.server.Shutdown(context.Background())
}
