package middleware

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/upmio/dbscale-kube/pkg/utils"
	log "k8s.io/klog/v2"
)

type DebugRequestMiddleware struct{}

// DebugRequestMiddleware dumps the request to logger
func (DebugRequestMiddleware) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error)) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {

		start := time.Now()

		code, out, err := func() (int, interface{}, error) {

			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
				return handler(ctx, w, r, vars)
			}
			if err := checkForJSON(r); err != nil {
				// lijj32: when http header is not application/json, the call will fail, thus we should log this error.
				log.Error("check for json failed. %s", err.Error())
				return handler(ctx, w, r, vars)
			}

			const maxBodySize = 4096 // 4KB

			if r.ContentLength > int64(maxBodySize) {
				return handler(ctx, w, r, vars)
			}

			body := r.Body
			bufReader := bufio.NewReaderSize(body, maxBodySize)
			r.Body = newReadCloserWrapper(bufReader, func() error { return body.Close() })

			b, err := bufReader.Peek(maxBodySize)
			if err != io.EOF {
				// either there was an error reading, or the buffer is full (in which case the request is too large)
				return handler(ctx, w, r, vars)
			}

			formStr, errMarshal := utils.MaskJsonSecret(b)
			if errMarshal == nil {
				log.Infof("form data: %s", string(formStr))
			} else {
				log.Infof("form data: %s", string(b))
			}

			return handler(ctx, w, r, vars)
		}()

		log.Infof("Calling %s %s [%s] %s", r.Method, r.RequestURI, r.RemoteAddr, time.Since(start))

		return code, out, err
	}
}

// checkForJSON makes sure that the request's Content-Type is application/json.
func checkForJSON(r *http.Request) error {
	ct := r.Header.Get("Content-Type")

	// No Content-Type header is ok as long as there's no Body
	if ct == "" {
		if r.Body == nil || r.ContentLength == 0 {
			return nil
		}
	}

	// Otherwise it better be json
	if matchesContentType(ct, "application/json") {
		return nil
	}
	return fmt.Errorf("Content-Type specified (%s) must be 'application/json'", ct)
}

// matchesContentType validates the content type against the expected one
func matchesContentType(contentType, expectedType string) bool {
	mimetype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		log.Errorf("Error parsing media type: %s error: %v", contentType, err)
	}
	return err == nil && mimetype == expectedType
}

type readCloserWrapper struct {
	io.Reader
	closer func() error
}

func (r *readCloserWrapper) Close() error {
	return r.closer()
}

// newReadCloserWrapper returns a new io.ReadCloser.
func newReadCloserWrapper(r io.Reader, closer func() error) io.ReadCloser {
	return &readCloserWrapper{
		Reader: r,
		closer: closer,
	}
}
