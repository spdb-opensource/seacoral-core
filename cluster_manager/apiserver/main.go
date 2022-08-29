package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/upmio/dbscale-kube/pkg/server"
	"github.com/upmio/dbscale-kube/pkg/server/middleware"
	"github.com/upmio/dbscale-kube/pkg/signals"
	"github.com/upmio/dbscale-kube/pkg/vars"
)

var (
	fakeDB      bool
	versionFlag bool
	addr        = "localhost:8080"

	tlsconfig *tls.Config
)

func init() {
	initDBConfig()
	flag.BoolVar(&versionFlag, "version", false, "show the version ")
	flag.StringVar(&addr, "addr", addr, "apiserver addr of server")
	flag.StringVar(&execServicePort, "exec-port", execServicePort, "exec server port")
}

func main() {
	flag.Parse()

	if versionFlag {
		fmt.Println("version:", vars.GITCOMMIT, vars.BUILDTIME)
		return
	}

	rand.Seed(time.Now().UTC().UnixNano())
	logs.InitLogs()
	defer logs.FlushLogs()

	klog.Info("VERSION: ", vars.GITCOMMIT, " ", vars.BUILDTIME)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	addrs := []server.Addr{
		{
			Proto: "tcp",
			Addr:  addr,
		},
	}

	srv := server.NewServer(addrs, tlsconfig)
	srv.AddMiddleware(middleware.DebugRequestMiddleware{})
	srv.AddMiddleware(middleware.ErrorRequestMiddleware{})

	err := initRouter(srv)
	if err != nil {
		klog.Fatal("Init routers:", err)
		return
	}

	if err := srv.ListenAndServe(); err != nil {
		srv.Shutdown()

		klog.Fatalf("HTTP server shut down! %v", err)

		os.Exit(1)
	}

	<-stopCh

	// We received an interrupt signal, shut down.
	srv.Shutdown()

	klog.Info("Shut down!")
}
