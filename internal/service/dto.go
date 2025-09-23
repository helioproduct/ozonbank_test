package service

import "time"

type CreatePostRequest struct {
	UserID          int64  `validate:"required,gt=0"`
	Title           string `validate:"required"`
	Text            string `validate:"required"`
	CommentsEnabled bool
}

type GetPostsAfterRequest struct {
	AfterCreatedAt time.Time `validate:"required"`
	AfterID        int64     `validate:"gte=0"`
	Limit          int       `validate:"gt=0"`
}

type CreateCommentRequest struct {
	PostID   int64 `validate:"required,gt=0"`
	ParentID *int64
	UserID   int64  `validate:"required,gt=0"`
	Body     string `validate:"required"`
}

type GetCommentsAfterRequest struct {
	PostID         int64     `validate:"required,gt=0"`
	AfterCreatedAt time.Time `validate:"required"`
	AfterID        int64     `validate:"gte=0"`
	Limit          int       `validate:"gt=0"`
}

type GetRepliesAfterRequest struct {
	PostID         int64     `validate:"required,gt=0"`
	ParentID       int64     `validate:"required,gt=0"`
	AfterCreatedAt time.Time `validate:"required"`
	AfterID        int64     `validate:"gte=0"`
	Limit          int       `validate:"gt=0"`
}
