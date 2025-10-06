package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/ifnotnil/daemon"
)

func main() {
	d := daemon.Start(context.Background())
	ctx := d.CTX()

	db := dbConn{}

	httpServer := NewHTTPModule(ctx)
	httpServer.Start(d.FatalErrorsChannel()) // starts its own go routine

	d.Defer(
		db.Close,            // finally, close db
		daemon.CancelCTX,    // then cancel the ctx
		httpServer.ShutDown, // first shutdown http server
	)

	d.Wait()
}

type httpModule struct {
	server *http.Server
}

func (s *httpModule) Start(fatalErrors chan<- error) {
	go func() {
		err := s.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatalErrors <- err
		}
	}()
}

func (s *httpModule) ShutDown(ctx context.Context) {
	_ = s.server.Shutdown(ctx)
}

func NewHTTPModule(ctx context.Context) *httpModule {
	return &httpModule{
		server: &http.Server{
			Addr:              "0.0.0.0:3030",
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: 3 * time.Second,
			BaseContext: func(_ net.Listener) context.Context {
				return ctx
			},
		},
	}
}

type dbConn struct{}

func (dbConn) Close(ctx context.Context) {}
