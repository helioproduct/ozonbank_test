package service

import (
	"fmt"
	"myreddit/internal/adapter/out/storage"
	"myreddit/pkg/pagination"
)

type CreatePostRequest struct {
	UserID          int64  `validate:"required,gt=0"`
	Title           string `validate:"required"`
	Text            string `validate:"required"`
	CommentsEnabled bool
}

type CreateCommentRequest struct {
	PostID   int64 `validate:"required,gt=0"`
	ParentID *int64
	UserID   int64  `validate:"required,gt=0"`
	Body     string `validate:"required"`
}

func validatePagination(in pagination.PageRequest) error {
	beforeCursorProvided := in.BeforeCursor != nil && *in.BeforeCursor != ""
	afterCursorProvided := in.AfterCursor != nil && *in.AfterCursor != ""

	if beforeCursorProvided && afterCursorProvided {
		return fmt.Errorf("both cursors provided: %w", ErrInvalidRequest)
	}
	return nil
}

func toGetPostsParams(in pagination.PageRequest) (storage.GetPostsParams, error) {
	if err := validatePagination(in); err != nil {
		return storage.GetPostsParams{}, err
	}

	if in.Limit <= 0 {
		in.Limit = DefaultPostsLimit
	}
	in.Limit = min(in.Limit, MaxPostsLimit)

	before, err := pagination.Decode(in.BeforeCursor)
	if err != nil {
		return storage.GetPostsParams{}, fmt.Errorf("error decoding before-cursor: %w", err)
	}

	after, err := pagination.Decode(in.AfterCursor)
	if err != nil {
		return storage.GetPostsParams{}, fmt.Errorf("error decoding after-cursor: %w", err)
	}

	if before == nil && after == nil {
		return storage.GetPostsParams{}, fmt.Errorf("cursor is required: %w", ErrInvalidRequest)
	}

	var params storage.GetPostsParams
	params.Limit = in.Limit

	if before != nil {
		params.Cursor = *before
		params.Direction = storage.DirectionBefore
	} else {
		params.Cursor = *after
		params.Direction = storage.DirectionAfter
	}
	return params, nil
}

func toGetCommentsRequest(postID int64, in pagination.PageRequest) (storage.GetCommentsParams, error) {
	if err := validatePagination(in); err != nil {
		return storage.GetCommentsParams{}, err
	}

	if postID <= 0 {
		return storage.GetCommentsParams{}, fmt.Errorf("postID must be > 0: %w", ErrInvalidRequest)
	}

	if in.Limit <= 0 {
		in.Limit = DefaultCommentsLimit
	}
	in.Limit = min(in.Limit, MaxCommentsLimit)

	before, err := pagination.Decode(in.BeforeCursor)
	if err != nil {
		return storage.GetCommentsParams{}, fmt.Errorf("error decoding before-cursor: %w", err)
	}

	after, err := pagination.Decode(in.AfterCursor)
	if err != nil {
		return storage.GetCommentsParams{}, fmt.Errorf("decoding after-cursor: %w", err)
	}

	if before == nil && after == nil {
		return storage.GetCommentsParams{}, fmt.Errorf("cursor is required: %w", ErrInvalidRequest)
	}

	var params storage.GetCommentsParams
	if before != nil {
		params.Cursor = *before
		params.Direction = storage.DirectionBefore
	} else {
		params.Cursor = *after
		params.Direction = storage.DirectionAfter
	}

	return params, nil
}

func toGetRepliesParams(postID, parentID int64, in pagination.PageRequest) (storage.GetRepliesParams, error) {
	if err := validatePagination(in); err != nil {
		return storage.GetRepliesParams{}, err
	}

	if postID <= 0 {
		return storage.GetRepliesParams{}, fmt.Errorf("postID must be > 0: %w", ErrInvalidRequest)
	}

	if parentID <= 0 {
		return storage.GetRepliesParams{}, fmt.Errorf("parentID must be > 0: %w", ErrInvalidRequest)
	}

	if in.Limit <= 0 {
		in.Limit = DefaultCommentsLimit
	}
	in.Limit = min(in.Limit, MaxCommentsLimit)

	before, err := pagination.Decode(in.BeforeCursor)
	if err != nil {
		return storage.GetRepliesParams{}, fmt.Errorf("error decoding before-cursor: %w", err)
	}

	after, err := pagination.Decode(in.AfterCursor)
	if err != nil {
		return storage.GetRepliesParams{}, fmt.Errorf("error decoding after-cursor: %w", err)
	}

	if before == nil && after == nil {
		return storage.GetRepliesParams{}, fmt.Errorf("cursor is required: %w", ErrInvalidRequest)
	}

	var params storage.GetRepliesParams
	params.Limit = in.Limit

	if before != nil {
		params.Cursor = *before
		params.Direction = storage.DirectionBefore
	} else {
		params.Cursor = *after
		params.Direction = storage.DirectionAfter
	}

	return params, nil
}
