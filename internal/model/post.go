package model

import "time"

type Post struct {
	ID               int64
	Title            string
	Body             string
	UserID           int64
	CommentsDisabled bool
	CreatedAt        time.Time
}
