package service

import (
	"context"
	"fmt"
	"myreddit/internal/model"
	"myreddit/pkg/pagination"

	"github.com/go-playground/validator/v10"
)

const (
	DefaultPostsLimit = 50
	MaxPostsLimit     = 250
)

type PostStorage interface {
	CreatePost(ctx context.Context, req CreatePostRequest) (model.Post, error)
	GetPostByID(ctx context.Context, postID int64) (model.Post, error)
	GetPosts(ctx context.Context, limit int) ([]model.Post, error)
	GetPostsAfter(ctx context.Context, req GetPostsAfterRequest) ([]model.Post, error)

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
	return s.postStorage.CreatePost(ctx, req)
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

func (s *PostService) GetPosts(ctx context.Context, req pagination.PageRequest) (pagination.Page[model.Post], error) {
	var (
		posts []model.Post
		err   error
	)

	limit := req.Limit
	if limit <= 0 {
		limit = DefaultPostsLimit
	}
	peek := limit + 1

	// если курсора еще  нет
	if req.AfterCursor == nil || *req.AfterCursor == "" {
		posts, err = s.postStorage.GetPosts(ctx, peek)
		if err != nil {
			return pagination.Page[model.Post]{}, err
		}
	} else {
		cur, err := pagination.Decode(*req.AfterCursor)
		if err != nil {
			return pagination.Page[model.Post]{}, err
		}
		posts, err = s.postStorage.GetPostsAfter(ctx, GetPostsAfterRequest{
			CreatedAt: cur.CreatedAt,
			PostID:    cur.ID,
			Limit:     peek,
		})
		if err != nil {
			return pagination.Page[model.Post]{}, err
		}
	}

	page := pagination.Page[model.Post]{}

	if len(posts) == 0 {
		page.Items = nil
		page.Count = 0
		page.HasNextPage = false
		page.EndCursor = nil
		return page, nil
	}

	if len(posts) > limit {
		page.HasNextPage = true
		posts = posts[:limit]
	}

	page.Items = posts
	page.Count = len(posts)

	last := posts[len(posts)-1]
	end := pagination.Encode(pagination.Cursor{
		CreatedAt: last.CreatedAt,
		ID:        last.ID,
	})
	page.EndCursor = &end

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
