package model

import "time"

type Post struct {
	ID              int64
	Title           string
	Text            string
	UserID          int64
	CommentsEnabled bool
	CreatedAt       time.Time
}
