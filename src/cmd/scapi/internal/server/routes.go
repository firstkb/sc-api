package server

import (
	"context"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/pingsvc"
	"github.com/firstkb/sc-api/internal/httpx/apperr"
	"github.com/firstkb/sc-api/internal/httpx/handler"
	"github.com/firstkb/sc-api/internal/httpx/router"
)

func (srv *Server) buildRoutes() (*http.ServeMux, *router.Classifier) {
	b := router.NewBuilder()

	b.Handle("HEALTH_STATUS", "GET", "/status", router.TierHealth,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("OK\n"))
		}))

	b.Handle("HEALTH_LIVE", "GET", "/healthz", router.TierHealth,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("ok\n"))
		}))

	b.Handle("HEALTH_READY", "GET", "/readyz", router.TierHealth,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("ready\n"))
		}))

	b.Handle("PING_GET", "GET", "/ping", router.TierSecure,
		handler.HandleJson(func(ctx context.Context, _ *http.Request, _ any) (*pingsvc.Ping, error) {
			info, err := srv.pingsvc.GetPing(ctx)
			if err != nil {
				return nil, apperr.WrapAndLog(srv.logger, ctx, "PING_GET",
					http.StatusInternalServerError, "cannot get ping", err, srv.ClaimForLog(ctx)...)
			}
			return info, nil
		}, srv.logger))

	return b.Mux(), b.Classifier()
}

/*func (srv *Server) addRoutes() http.Handler {
router := http.NewServeMux()

router.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		fmt.Fprintln(w, "OK")
	} else {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
})

/*router.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		fmt.Fprintln(w, "OK")
	} else {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
})*/

/*router.Handle("GET /profile", httpext.HandleJson(func(ctx context.Context, _ *http.Request, _ any) (*profilesvc.Profile, error) {
	info, err := srv.profilesvc.GetProfile(ctx)
	if err != nil {
		return nil, srv.wrapErr(ctx, "GET_PROFILE",
			http.StatusInternalServerError, "cannot get profile", err)
	}
	return info, nil
}, srv.logger))*/

/*router.Handle("GET /ping", httpext.HandleJson(func(ctx context.Context, _ *http.Request, _ any) (*pingsvc.Ping, error) {
		info, err := srv.pingsvc.GetPing(ctx)
		if err != nil {
			return nil, srv.wrapErr(ctx, "GET_PING",
				http.StatusInternalServerError, "cannot get ping", err)
		}
		return info, nil
	}, srv.logger))

	var handler http.Handler = router
	return handler
}
*/
