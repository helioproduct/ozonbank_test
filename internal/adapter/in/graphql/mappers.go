package graphql

import (
	"strconv"
	"time"

	gqlmodel "myreddit/internal/adapter/in/graphql/model"
	"myreddit/internal/model"
	"myreddit/pkg/pagination"
)

func toPostNode(p model.Post) *gqlmodel.Post {
	return &gqlmodel.Post{
		ID:              strconv.FormatInt(p.ID, 10),
		Title:           p.Title,
		Body:            p.Body,
		UserID:          strconv.FormatInt(p.UserID, 10),
		CommentsEnabled: p.CommentsEnabled,
		CreatedAt:       p.CreatedAt,
	}
}

func toCommentNode(c model.Comment) *gqlmodel.Comment {
	var parent *string
	if c.ParentID != nil {
		s := strconv.FormatInt(*c.ParentID, 10)
		parent = &s
	}
	return &gqlmodel.Comment{
		ID:        strconv.FormatInt(c.ID, 10),
		PostID:    strconv.FormatInt(c.PostID, 10),
		ParentID:  parent,
		UserID:    strconv.FormatInt(c.UserID, 10),
		Body:      c.Body,
		CreatedAt: c.CreatedAt,
	}
}

func encodeCursor(id int64, ts time.Time) *string {
	cursor := pagination.Cursor{CreatedAt: ts, ID: id}
	return cursor.Encode()
}

func toPageRequest(in *gqlmodel.PageInput) pagination.PageRequest {
	var limit int
	var before, after *string
	if in != nil {
		if in.Limit != nil {
			limit = *in.Limit
		}
		if in.Before != nil && *in.Before != "" {
			before = in.Before
		}
		if in.After != nil && *in.After != "" {
			after = in.After
		}
	}
	return pagination.PageRequest{
		Limit:        limit,
		BeforeCursor: before,
		AfterCursor:  after,
	}
}
