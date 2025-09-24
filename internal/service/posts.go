package service

import (
	"context"
	"fmt"
	"myreddit/internal/adapter/out/storage"
	"myreddit/internal/model"
	"myreddit/pkg/pagination"

	"github.com/go-playground/validator/v10"
)

const (
	DefaultPostsLimit = 50
	MaxPostsLimit     = 250
)

//go:generate mockgen -source=posts.go -destination=./post_storage_mock.go -package=service myreddit/internal/service PostStorage
type PostStorage interface {
	CreatePost(ctx context.Context, post model.Post) (model.Post, error)
	GetPostByID(ctx context.Context, postID int64) (model.Post, error)
	GetPosts(ctx context.Context, limit int) ([]model.Post, error)
	GetPostsWithCursor(ctx context.Context, params storage.GetPostsParams) ([]model.Post, error)
	GetPostAuthorID(ctx context.Context, postID int64) (int64, error)
	SetCommentsEnabled(ctx context.Context, postID int64, enabled bool) error
}

type PostService struct {
	postStorage PostStorage
}

func NewPostService(postStorage PostStorage) *PostService {
	return &PostService{
		postStorage: postStorage,
	}
}

func (s *PostService) CreatePost(ctx context.Context, req CreatePostRequest) (model.Post, error) {
	if err := validator.New().Struct(req); err != nil {
		return model.Post{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	return s.postStorage.CreatePost(ctx, model.Post{
		UserID:          req.UserID,
		Title:           req.Title,
		Text:            req.Text,
		CommentsEnabled: req.CommentsEnabled,
	})
}

func (s *PostService) GetPostByID(ctx context.Context, postID int64) (model.Post, error) {
	if postID <= 0 {
		return model.Post{}, fmt.Errorf("postID must be > 0: %w", ErrInvalidRequest)
	}
	p, err := s.postStorage.GetPostByID(ctx, postID)
	if err != nil {
		return model.Post{}, err
	}
	return p, nil
}

func (s *PostService) GetPosts(ctx context.Context, in pagination.PageRequest) (pagination.Page[model.Post], error) {
	var (
		posts []model.Post
		err   error
		page  pagination.Page[model.Post]
	)

	if err := validatePagination(in); err != nil {
		return page, err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = DefaultPostsLimit
	}
	if limit > MaxPostsLimit {
		limit = MaxPostsLimit
	}
	peek := limit + 1

	afterProvided := in.AfterCursor != nil && *in.AfterCursor != ""
	beforeProvided := in.BeforeCursor != nil && *in.BeforeCursor != ""

	switch {
	case !afterProvided && !beforeProvided:
		posts, err = s.postStorage.GetPosts(ctx, peek)
		if err != nil {
			return page, err
		}

	default:
		params, err := toGetPostsParams(in)
		if err != nil {
			return page, err
		}
		params.Limit = peek
		posts, err = s.postStorage.GetPostsWithCursor(ctx, params)
		if err != nil {
			return page, err
		}
	}

	if len(posts) == 0 {
		page.Items = nil
		page.Count = 0
		page.HasNextPage = false
		page.StartCursor = nil
		page.EndCursor = nil
		return page, nil
	}

	if len(posts) > limit {
		page.HasNextPage = true
		posts = posts[:limit]
	}

	page.Items = posts
	page.Count = len(posts)

	startCursor := pagination.Cursor{
		CreatedAt: posts[0].CreatedAt,
		ID:        posts[0].ID,
	}
	endCursor := pagination.Cursor{
		CreatedAt: posts[len(posts)-1].CreatedAt,
		ID:        posts[len(posts)-1].ID,
	}

	page.StartCursor, page.EndCursor = startCursor.Encode(), endCursor.Encode()
	return page, nil
}

func (s *PostService) ChangePostCommentPermission(ctx context.Context, postID, userID int64, enabled bool) error {
	if postID <= 0 || userID <= 0 {
		return ErrInvalidRequest
	}
	ownerID, err := s.postStorage.GetPostAuthorID(ctx, postID)
	if err != nil {
		return err
	}
	if ownerID != userID {
		return fmt.Errorf("%w: not a post owner", ErrForbidden)
	}
	return s.postStorage.SetCommentsEnabled(ctx, postID, enabled)
}
