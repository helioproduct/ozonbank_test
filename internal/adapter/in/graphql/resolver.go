package graphql

import (
	"context"
	"myreddit/internal/model"
	"myreddit/internal/service"
	"myreddit/pkg/pagination"
)

type PostService interface {
	CreatePost(ctx context.Context, req service.CreatePostRequest) (model.Post, error)
	GetPostByID(ctx context.Context, postID int64) (model.Post, error)
	GetPosts(ctx context.Context, in pagination.PageRequest) (pagination.Page[model.Post], error)
	ChangePostCommentPermission(ctx context.Context, postID, userID int64, enabled bool) error
}

type CommentService interface {
	CreateComment(ctx context.Context, req service.CreateCommentRequest) (model.Comment, error)
	GetCommentByID(ctx context.Context, commentID int64) (model.Comment, error)
	GetCommentsByPost(ctx context.Context, in pagination.PageRequest, postID int64) (pagination.Page[model.Comment], error)
	GetReplies(ctx context.Context, in pagination.PageRequest, postID, parentID int64) (pagination.Page[model.Comment], error)
	Listen(ctx context.Context, postID int64) (<-chan model.Comment, error)
}

type Resolver struct {
	postsService   PostService
	commentService CommentService
}
