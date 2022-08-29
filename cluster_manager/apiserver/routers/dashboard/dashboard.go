package dashboard

import (
	dashboardHandler "github.com/upmio/dbscale-kube/dashboard_backend/handler"
	"github.com/upmio/dbscale-kube/pkg/server"
	"github.com/upmio/dbscale-kube/pkg/server/handlerrouter"
	"net/http"
)

func RegisterDashboardRoute(siteDomain string, apiHandler http.Handler, srv *server.Server) {
	r := &dashboardRoute{}

	r.handlerRoutes = []handlerrouter.HandlerRoute{
		handlerrouter.NewHandlerRoute("/"+siteDomain+"/api/sockjs", dashboardHandler.CreateAttachHandler("/"+siteDomain+"/api/sockjs")),
		handlerrouter.NewHandlerRoute("/"+siteDomain+"/api/", apiHandler),
	}

	srv.AddRawRouter(r)
}

type dashboardRoute struct {
	handlerRoutes []handlerrouter.HandlerRoute
}

func (dr dashboardRoute) HandlerRoutes() []handlerrouter.HandlerRoute {
	return dr.handlerRoutes
}
