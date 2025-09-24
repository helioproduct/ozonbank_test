// cmd/app/app.go
package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"myreddit/config"
	gqlin "myreddit/internal/adapter/in/graphql"
	inmemorybus "myreddit/internal/adapter/out/commentbus/inmemory"
	memstore "myreddit/internal/adapter/out/storage/inmemory"
	pgstore "myreddit/internal/adapter/out/storage/postgres"
	"myreddit/internal/service"
	"myreddit/pkg/logger"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	cfg  config.Config
	srv  *http.Server
	pool *pgxpool.Pool
}

func NewApp(ctx context.Context, cfg config.Config) (*App, error) {
	log := logger.FromContext(ctx)

	var (
		postStorage    service.PostStorage
		commentStorage service.CommentStorage
		pool           *pgxpool.Pool
	)

	switch cfg.StorageType {
	case "postgres":
		var err error
		pool, err = pgxpool.New(ctx, cfg.Postgres.GetDSN())
		if err != nil {
			return nil, fmt.Errorf("pgxpool: %w", err)
		}
		postStorage = pgstore.NewPostStorage(pool, trmpgx.DefaultCtxGetter)
		commentStorage = pgstore.NewCommentStorage(pool, trmpgx.DefaultCtxGetter)

	default:
		postStorage = memstore.NewPostStorage()
		commentStorage = memstore.NewCommentStorage()
	}

	bus := inmemorybus.New()

	postSvc := service.NewPostService(postStorage)
	commentSvc := service.NewCommentService(commentStorage, bus, postStorage)

	resolver := gqlin.NewResolver(postSvc, commentSvc)
	es := gqlin.NewExecutableSchema(gqlin.Config{Resolvers: resolver})
	gqlSrv := handler.New(es)

	gqlSrv.AddTransport(transport.POST{})
	gqlSrv.AddTransport(&transport.Websocket{
		KeepAlivePingInterval: time.Duration(cfg.WS.KeepAliveSeconds) * time.Second,
	})
	gqlSrv.Use(extension.Introspection{})

	mux := http.NewServeMux()
	mux.Handle("/query", gqlSrv)
	mux.Handle("/", playground.Handler("GraphQL Playground", "/query"))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := ":" + cfg.HTTP.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Info("app initialized", "addr", addr, "storage", cfg.StorageType)
	return &App{cfg: cfg, srv: srv, pool: pool}, nil
}

func (a *App) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", a.srv.Addr)
		errCh <- a.srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown requested")
		shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = a.srv.Shutdown(shCtx)
		if a.pool != nil {
			a.pool.Close()
		}
		return nil

	case err := <-errCh:
		if a.pool != nil {
			a.pool.Close()
		}
		return err
	}
}
