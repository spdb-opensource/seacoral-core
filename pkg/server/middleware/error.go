package middleware

import (
	"context"
	"net/http"

	log "k8s.io/klog/v2"
)

type ErrorRequestMiddleware struct{}

// ErrorRequestMiddleware dumps the request to logger
func (ErrorRequestMiddleware) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error)) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
		code, obj, err := handler(ctx, w, r, vars)
		if err != nil {
			log.Errorf("Calling %s %s [%s] ,%d:%s", r.Method, r.RequestURI, r.RemoteAddr, code, err)
		}

		return code, obj, err
	}
}
