package main

import (
	"context"
	"fmt"
	"myreddit/internal/adapter/out/storage/postgres"
	"myreddit/pkg/logger"
	"myreddit/pkg/pagination"

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

	post, err := postStorage.GetPostByID(ctx, -1)

	if err != nil {
		logger.Error("error", "error", err)
	}

	fmt.Println(post)

	return

	// for i := 0; i < 1000*1000; i++ {

	// 	req := postgres.CreatePostRequest{
	// 		UserID:          100,
	// 		Title:           "new post",
	// 		Text:            "post text",
	// 		CommentsEnabled: true,
	// 	}
	// 	_, err := postStorage.CreatePost(ctx, req)
	// 	if err != nil {
	// 		logger.Error("error creating post", slog.Any("error", err))
	// 		return
	// 	}
	// }
	// return

	id, err := postStorage.GetPostAuthorID(ctx, 1000000000)
	if err != nil {
		logger.Error("error getting post", "error", err)
	}

	fmt.Println(id)

	return

	posts, err := postStorage.GetPosts(ctx, 1)

	fmt.Println(posts)

	fmt.Println("nexts")

	// fmt.Println(posts[0].ID)

	postsAfter, err := postStorage.GetPostsAfter(ctx, postgres.GetPostsAfterRequest{
		AfterCreatedAt: posts[0].CreatedAt,
		AfterID:        posts[0].ID,
		Limit:          1000,
	})
	if err != nil {
		logger.Error("error getting posts", "error", err)
	}

	for _, post := range postsAfter {

		cursor := pagination.Cursor{
			CreatedAt: post.CreatedAt,
			ID:        post.ID,
		}

		encoded := pagination.Encode(cursor)

		newCursor, err := pagination.Decode(encoded)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(cursor, encoded, newCursor)

	}

}
