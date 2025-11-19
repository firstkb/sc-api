package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/pingsvc"
	"github.com/firstkb/sc-api/internal/utils/httpext"
)

func (srv *Server) addRoutes() http.Handler {
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

	router.Handle("GET /ping", httpext.HandleJson(func(ctx context.Context, _ *http.Request, _ any) (*pingsvc.Ping, error) {
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
