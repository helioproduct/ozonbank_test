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
	GetCommentsByPostAfter(ctx context.Context, req GetCommentsAfterRequest) ([]model.Comment, error)
	GetReplies(ctx context.Context, postID, parentID int64, limit int) ([]model.Comment, error)
	GetRepliesAfter(ctx context.Context, req GetRepliesAfterRequest) ([]model.Comment, error)
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

func (s *CommentService) GetCommentsByPost(ctx context.Context, req pagination.PageRequest, postID int64) (pagination.Page[model.Comment], error) {
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultCommentsLimit
	}
	peek := limit + 1

	var comments []model.Comment
	var err error

	if req.AfterCursor == nil || *req.AfterCursor == "" {
		comments, err = s.commentStorage.GetCommentsByPost(ctx, postID, peek)
		if err != nil {
			return pagination.Page[model.Comment]{}, err
		}
	} else {
		cur, err := pagination.Decode(*req.AfterCursor)
		if err != nil {
			return pagination.Page[model.Comment]{}, err
		}
		comments, err = s.commentStorage.GetCommentsByPostAfter(ctx, GetCommentsRequest{
			PostID:    postID,
			CreatedAt: cur.CreatedAt,
			CommentID: cur.ID,
			Limit:     peek,
		})
		if err != nil {
			return pagination.Page[model.Comment]{}, err
		}
	}

	page := pagination.Page[model.Comment]{}
	if len(comments) == 0 {
		return page, nil
	}

	if len(comments) > limit {
		page.HasNextPage = true
		comments = comments[:limit]
	}
	page.Items = comments
	page.Count = len(comments)

	last := comments[len(comments)-1]
	end := pagination.Encode(pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	page.EndCursor = &end

	return page, nil
}

func (s *CommentService) GetReplies(ctx context.Context, req pagination.PageRequest, postID, parentID int64) (pagination.Page[model.Comment], error) {
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultCommentsLimit
	}
	peek := limit + 1

	var replies []model.Comment
	var err error

	if req.AfterCursor == nil || *req.AfterCursor == "" {
		replies, err = s.commentStorage.GetReplies(ctx, postID, parentID, peek)
		if err != nil {
			return pagination.Page[model.Comment]{}, err
		}
	} else {
		cur, err := pagination.Decode(*req.AfterCursor)
		if err != nil {
			return pagination.Page[model.Comment]{}, err
		}
		replies, err = s.commentStorage.GetRepliesAfter(ctx, GetRepliesAfterRequest{
			PostID:    postID,
			ParentID:  parentID,
			CreatedAt: cur.CreatedAt,
			CommentID: cur.ID,
			Limit:     peek,
		})
		if err != nil {
			return pagination.Page[model.Comment]{}, err
		}
	}

	page := pagination.Page[model.Comment]{}
	if len(replies) == 0 {
		return page, nil
	}

	if len(replies) > limit {
		page.HasNextPage = true
		replies = replies[:limit]
	}
	page.Items = replies
	page.Count = len(replies)

	last := replies[len(replies)-1]
	end := pagination.Encode(pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	page.EndCursor = &end

	return page, nil
}
