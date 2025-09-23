package service

import (
	"context"
	"fmt"
	"myreddit/internal/model"
	"myreddit/pkg/pagination"

	"github.com/go-playground/validator/v10"
)

const (
	DefaultCommentsLimit = 50
	MaxCommentsLimit     = 250
)

type CommentService struct {
	commentStorage CommentStorage
}

type CommentStorage interface {
	CreateComment(ctx context.Context, req CreateCommentRequest) (model.Comment, error)
	GetCommentByID(ctx context.Context, commentID int64) (model.Comment, error)
	GetCommentsByPost(ctx context.Context, postID int64, limit int) ([]model.Comment, error)
	GetReplies(ctx context.Context, postID, parentID int64, limit int) ([]model.Comment, error)
	GetCommentsByPostWithCursor(ctx context.Context, req GetCommentsRequest) ([]model.Comment, error)
	GetRepliesWithCursor(ctx context.Context, req GetRepliesRequest) ([]model.Comment, error)
}

func NewCommentService(commentsStorage CommentStorage) *CommentService {
	return &CommentService{
		commentStorage: commentsStorage,
	}
}

func (s *CommentService) CreateComment(ctx context.Context, req CreateCommentRequest) (model.Comment, error) {
	if err := validator.New().Struct(req); err != nil {
		return model.Comment{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	return s.commentStorage.CreateComment(ctx, req)
}

func (s *CommentService) GetCommentByID(ctx context.Context, commentID int64) (model.Comment, error) {
	if commentID <= 0 {
		return model.Comment{}, ErrInvalidRequest
	}
	return s.commentStorage.GetCommentByID(ctx, commentID)
}

func (s *CommentService) GetCommentsByPost(ctx context.Context, in pagination.PageRequest, postID int64) (pagination.Page[model.Comment], error) {
	var (
		items []model.Comment
		err   error
		page  pagination.Page[model.Comment]
	)

	if err := validatePagination(in); err != nil {
		return page, err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = DefaultCommentsLimit
	}
	if limit > MaxCommentsLimit {
		limit = MaxCommentsLimit
	}
	peek := limit + 1

	afterProvided := in.AfterCursor != nil && *in.AfterCursor != ""
	beforeProvided := in.BeforeCursor != nil && *in.BeforeCursor != ""

	switch {
	case !afterProvided && !beforeProvided:
		items, err = s.commentStorage.GetCommentsByPost(ctx, postID, peek)
		if err != nil {
			return page, err
		}

	default:
		req, err := toGetCommentsRequest(postID, in)
		if err != nil {
			return page, err
		}
		req.Limit = peek

		items, err = s.commentStorage.GetCommentsByPostWithCursor(ctx, req)
		if err != nil {
			return page, err
		}
	}

	if len(items) == 0 {
		page.Items = nil
		page.Count = 0
		page.HasNextPage = false
		page.StartCursor = nil
		page.EndCursor = nil
		return page, nil
	}

	if len(items) > limit {
		page.HasNextPage = true
		items = items[:limit]
	}

	page.Items = items
	page.Count = len(items)

	startCursor := pagination.Cursor{
		CreatedAt: items[0].CreatedAt,
		ID:        items[0].ID,
	}
	endCursor := pagination.Cursor{
		CreatedAt: items[len(items)-1].CreatedAt,
		ID:        items[len(items)-1].ID,
	}

	page.StartCursor, page.EndCursor = startCursor.Encode(), endCursor.Encode()
	return page, nil
}

func (s *CommentService) GetReplies(ctx context.Context, in pagination.PageRequest, postID, parentID int64) (pagination.Page[model.Comment], error) {
	var (
		items []model.Comment
		err   error
		page  pagination.Page[model.Comment]
	)

	if err := validatePagination(in); err != nil {
		return page, err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = DefaultCommentsLimit
	}
	if limit > MaxCommentsLimit {
		limit = MaxCommentsLimit
	}
	peek := limit + 1

	afterProvided := in.AfterCursor != nil && *in.AfterCursor != ""
	beforeProvided := in.BeforeCursor != nil && *in.BeforeCursor != ""

	switch {
	case !afterProvided && !beforeProvided:
		items, err = s.commentStorage.GetReplies(ctx, postID, parentID, peek)
		if err != nil {
			return page, err
		}

	default:
		req, err := toGetRepliesRequest(postID, parentID, in)
		if err != nil {
			return page, err
		}
		req.Limit = peek

		items, err = s.commentStorage.GetRepliesWithCursor(ctx, req)
		if err != nil {
			return page, err
		}
	}

	if len(items) == 0 {
		page.Items = nil
		page.Count = 0
		page.HasNextPage = false
		page.StartCursor = nil
		page.EndCursor = nil
		return page, nil
	}

	if len(items) > limit {
		page.HasNextPage = true
		items = items[:limit]
	}

	page.Items = items
	page.Count = len(items)

	startCursor := pagination.Cursor{
		CreatedAt: items[0].CreatedAt,
		ID:        items[0].ID,
	}
	endCursor := pagination.Cursor{
		CreatedAt: items[len(items)-1].CreatedAt,
		ID:        items[len(items)-1].ID,
	}

	page.StartCursor, page.EndCursor = startCursor.Encode(), endCursor.Encode()
	return page, nil
}
