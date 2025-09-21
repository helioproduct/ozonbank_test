package model

import "time"

type Comment struct {
	ID        int64
	PostID    int64
	ParentID  *int64
	UserID    int64
	Body      string
	CreatedAt time.Time
}
