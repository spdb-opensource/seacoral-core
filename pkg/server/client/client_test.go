package client

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func setupTestServer(addr string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
	})

	srv := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go srv.ListenAndServe()

	return srv
}

func testClientGet(url string, client Client, t *testing.T) {
	for i := 0; i < 10; i++ {

		resp, err := client.Get(context.Background(), url)
		if err != nil {
			t.Errorf("%d %+v", i, err)
		} else {
			t.Log(i, resp.StatusCode)
		}
	}
}

func TestNewClient(t *testing.T) {
	addr := "127.0.0.1:2233"
	url := "http://" + addr + "/ping"

	srv := setupTestServer(addr)

	client := NewClient(addr, 10*time.Second, nil)

	testClientGet(url, client, t)

	err := srv.Shutdown(context.Background())
	if err != nil {
		t.Errorf("%+v", err)
	}

	for i := 0; i < 10; i++ {

		_, err := client.Get(context.Background(), url)
		if err == nil {
			t.Error(i, "Unexpected!")
		} else {
			t.Log(i, err)
		}
	}

	setupTestServer(addr)

	testClientGet(url, client, t)
}
