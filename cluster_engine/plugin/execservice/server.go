package execserver

import (
	"crypto/tls"
	"os"

	exec "github.com/upmio/dbscale-kube/cluster_engine/plugin/execservice/routers"
	"github.com/upmio/dbscale-kube/pkg/server"
	"github.com/upmio/dbscale-kube/pkg/server/middleware"
	"github.com/upmio/dbscale-kube/pkg/server/router"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func SetupExecServer(addr string, config *restclient.Config, tlsconfig *tls.Config, stopCh <-chan struct{}) {

	addrs := []server.Addr{
		{
			Proto: "tcp",
			Addr:  addr,
		},
	}

	srv := server.NewServer(addrs, tlsconfig)
	srv.AddMiddleware(middleware.DebugRequestMiddleware{})
	srv.AddMiddleware(middleware.ErrorRequestMiddleware{})

	err := initRouter(config, srv)
	if err != nil {
		klog.Error("Init routers:", err)
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

	klog.Info("HTTP Server Shut down!")
}

func initRouter(config *restclient.Config, adder router.Adder) error {
	exec.RegisterRouter(config, adder)

	return nil
}
