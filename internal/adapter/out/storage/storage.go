package storage

import (
	"errors"
	"myreddit/pkg/pagination"
)

type Direction int

const (
	DirectionUnspecified Direction = iota
	DirectionAfter
	DirectionBefore
)

var (
	ErrDirectionUnset = errors.New("direction must be set")
)

type GetPostsParams struct {
	Cursor    pagination.Cursor
	Direction Direction
	Limit     int
}

type GetCommentsParams struct {
	PostID    int64
	Cursor    pagination.Cursor
	Direction Direction
	Limit     int
}

type GetRepliesParams struct {
	PostID    int64
	ParentID  int64
	Cursor    pagination.Cursor
	Direction Direction
	Limit     int
}
