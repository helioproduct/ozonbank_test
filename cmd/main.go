package main

import (
	"context"
	"fmt"
	"log"
	"myreddit/internal/adapter/out/pubsub/inmemory"
	"myreddit/internal/adapter/out/storage/postgres"
	"myreddit/internal/service"
	"myreddit/pkg/logger"
	"net/http"
	"time"

	gql "myreddit/internal/adapter/in/graphql"

	// "github.com/99designs/gqlgen/codegen/testserver/benchmark/generated"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	logger := logger.FromContext(ctx)

	uri := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		"postgresuser", "password", "localhost", 5432, "myreddit",
	)

	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		logger.Error("error creating pool", "error", err)
	}

	// trManager := manager.Must(trmpgx.NewDefaultFactory(pool))
	postStorage := postgres.NewPostStorage(pool, trmpgx.DefaultCtxGetter)
	commentStorage := postgres.NewCommentStorage(pool, trmpgx.DefaultCtxGetter)

	bus := inmemory.New(100)

	postSvc := service.NewPostService(postStorage)
	commentSvc := service.NewCommentService(commentStorage, bus)

	resolver := gql.NewResolver(postSvc, commentSvc) // см. конструктор ниже
	es := gql.NewExecutableSchema(gql.Config{Resolvers: resolver})
	srv := handler.New(es)

	// поддержка POST и WebSocket для subscriptions
	srv.AddTransport(transport.POST{})
	srv.AddTransport(&transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
	})

	// включаем интроспекцию только в dev
	srv.Use(extension.Introspection{})

	http.Handle("/query", srv)
	http.Handle("/", playground.Handler("GraphQL Playground", "/query"))

	log.Println("GraphQL server running at http://localhost:8080/")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
