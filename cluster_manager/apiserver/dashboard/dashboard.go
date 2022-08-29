package dashboard

import (
	"github.com/upmio/dbscale-kube/dashboard_backend/args"
	"github.com/upmio/dbscale-kube/dashboard_backend/auth"
	authApi "github.com/upmio/dbscale-kube/dashboard_backend/auth/api"
	"github.com/upmio/dbscale-kube/dashboard_backend/auth/jwe"
	"github.com/upmio/dbscale-kube/dashboard_backend/client"
	clientapi "github.com/upmio/dbscale-kube/dashboard_backend/client/api"
	"github.com/upmio/dbscale-kube/dashboard_backend/handler"
	"github.com/upmio/dbscale-kube/dashboard_backend/integration"
	"github.com/upmio/dbscale-kube/dashboard_backend/settings"
	"github.com/upmio/dbscale-kube/dashboard_backend/sync"
	"github.com/upmio/dbscale-kube/dashboard_backend/systembanner"
	"k8s.io/klog/v2"
	"net/http"
)

var (
	kubeConfigRoot = "/root/"
	//apiServerHost = "localhost:8080"
)

//func init() {
//	flag.StringVar(&TheKubeConfig, "TheKubeConfig", TheKubeConfig, "TheKubeConfig path")
//	flag.StringVar(&apiServerHost, "apiServerHost", apiServerHost, "apiServerHost and port")
//}

func NewDashboardAPIHandler(path, domain, port string) (http.Handler, error) {
	clientManager := client.NewClientManager(path, "https://"+domain+":"+port)

	versionInfo, err := clientManager.InsecureClient().Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	klog.Infof("Successful initial request to the apiserver, version: %s", versionInfo.String())

	// Init auth manager
	authManager := initAuthManager(clientManager)

	// Init settings manager
	settingsManager := settings.NewSettingsManager()

	// Init system banner manager
	systemBannerManager := systembanner.NewSystemBannerManager(args.Holder.GetSystemBanner(),
		args.Holder.GetSystemBannerSeverity())

	// Init integrations
	integrationManager := integration.NewIntegrationManager(clientManager)

	apiHandler, err := handler.CreateHTTPAPIHandler(
		domain,
		integrationManager,
		clientManager,
		authManager,
		settingsManager,
		systemBannerManager)
	if err != nil {
		klog.Fatal(err)
	}

	return apiHandler, nil
}

func initAuthManager(clientManager clientapi.ClientManager) authApi.AuthManager {
	insecureClient := clientManager.InsecureClient()

	// Init default encryption key synchronizer
	synchronizerManager := sync.NewSynchronizerManager(insecureClient)
	keySynchronizer := synchronizerManager.Secret("kube-system", authApi.EncryptionKeyHolderName)

	// Register synchronizer. Overwatch will be responsible for restarting it in case of error.
	sync.Overwatch.RegisterSynchronizer(keySynchronizer, sync.AlwaysRestart)

	// Init encryption key holder and token manager
	keyHolder := jwe.NewRSAKeyHolder(keySynchronizer)
	tokenManager := jwe.NewJWETokenManager(keyHolder)
	//tokenTTL := time.Duration(args.Holder.GetTokenTTL())
	//if tokenTTL != authApi.DefaultTokenTTL {
	//	tokenManager.SetTokenTTL(tokenTTL)
	//}

	//authModes.Add(authApi.Token)
	// Set token manager for client manager.
	//clientManager.SetTokenManager(tokenManager)
	authModes := authApi.AuthenticationModes{}
	authModes.Add(authApi.Token)

	// UI logic dictates this should be the inverse of the cli option
	authenticationSkippable := true

	return auth.NewAuthManager(clientManager, tokenManager, authModes, authenticationSkippable)
}
