package service

import (
	"fmt"
	"myreddit/pkg/pagination"
)

type CreatePostRequest struct {
	UserID          int64  `validate:"required,gt=0"`
	Title           string `validate:"required"`
	Text            string `validate:"required"`
	CommentsEnabled bool
}

type GetPostsRequest struct {
	Before *pagination.Cursor
	After  *pagination.Cursor
	Limit  int
}

type CreateCommentRequest struct {
	PostID   int64 `validate:"required,gt=0"`
	ParentID *int64
	UserID   int64  `validate:"required,gt=0"`
	Body     string `validate:"required"`
}

type GetCommentsRequest struct {
	PostID int64
	Before *pagination.Cursor
	After  *pagination.Cursor
	Limit  int
}

type GetRepliesRequest struct {
	PostID   int64
	ParentID int64
	Before   *pagination.Cursor
	After    *pagination.Cursor
	Limit    int
}

func validatePagination(in pagination.PageRequest) error {
	beforeCursorProvided := in.BeforeCursor != nil && *in.BeforeCursor != ""
	afterCursorProvided := in.AfterCursor != nil && *in.AfterCursor != ""

	if beforeCursorProvided && afterCursorProvided {
		return fmt.Errorf("both cursors provided: %w", ErrInvalidRequest)
	}
	return nil
}

func toGetPostsRequest(in pagination.PageRequest) (GetPostsRequest, error) {
	if err := validatePagination(in); err != nil {
		return GetPostsRequest{}, err
	}

	if in.Limit <= 0 {
		in.Limit = DefaultPostsLimit
	}
	in.Limit = min(in.Limit, MaxPostsLimit)

	before, err := pagination.Decode(in.BeforeCursor)
	if err != nil {
		return GetPostsRequest{}, fmt.Errorf("error decoding before-cursor: %w", err)
	}

	after, err := pagination.Decode(in.AfterCursor)
	if err != nil {
		return GetPostsRequest{}, fmt.Errorf("error decoding after-cursor: %w", err)
	}

	return GetPostsRequest{
		Before: before,
		After:  after,
		Limit:  in.Limit,
	}, nil
}

func toGetCommentsRequest(postID int64, in pagination.PageRequest) (GetCommentsRequest, error) {
	if err := validatePagination(in); err != nil {
		return GetCommentsRequest{}, err
	}

	if postID <= 0 {
		return GetCommentsRequest{}, fmt.Errorf("postID must be > 0: %w", ErrInvalidRequest)
	}

	if in.Limit <= 0 {
		in.Limit = DefaultCommentsLimit
	}
	in.Limit = min(in.Limit, MaxCommentsLimit)

	before, err := pagination.Decode(in.BeforeCursor)
	if err != nil {
		return GetCommentsRequest{}, fmt.Errorf("error decoding before-cursor: %w", err)
	}

	after, err := pagination.Decode(in.AfterCursor)
	if err != nil {
		return GetCommentsRequest{}, fmt.Errorf("decoding after-cursor: %w", err)
	}

	return GetCommentsRequest{
		PostID: postID,
		Before: before,
		After:  after,
		Limit:  in.Limit,
	}, nil
}

func toGetRepliesRequest(postID, parentID int64, in pagination.PageRequest) (GetRepliesRequest, error) {
	if err := validatePagination(in); err != nil {
		return GetRepliesRequest{}, err
	}

	if postID <= 0 {
		return GetRepliesRequest{}, fmt.Errorf("postID must be > 0: %w", ErrInvalidRequest)
	}

	if parentID <= 0 {
		return GetRepliesRequest{}, fmt.Errorf("parentID must be > 0: %w", ErrInvalidRequest)
	}

	if in.Limit <= 0 {
		in.Limit = DefaultCommentsLimit
	}
	in.Limit = min(in.Limit, MaxCommentsLimit)

	before, err := pagination.Decode(in.BeforeCursor)
	if err != nil {
		return GetRepliesRequest{}, fmt.Errorf("error decoding before-cursor: %w", err)
	}

	after, err := pagination.Decode(in.AfterCursor)
	if err != nil {
		return GetRepliesRequest{}, fmt.Errorf("error decoding after-cursor: %w", err)
	}

	return GetRepliesRequest{
		PostID:   postID,
		ParentID: parentID,
		Before:   before,
		After:    after,
		Limit:    in.Limit,
	}, nil
}
