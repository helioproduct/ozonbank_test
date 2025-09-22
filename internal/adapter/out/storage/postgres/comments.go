package postgres

import (
	"context"
	"fmt"
	"myreddit/internal/model"

	"myreddit/pkg/tableinfo"

	sq "github.com/Masterminds/squirrel"
	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultCommentsLimit = 50
)

type CommentStorage struct {
	pool   *pgxpool.Pool
	getter *trmpgx.CtxGetter
}

func NewCommentStorage(pool *pgxpool.Pool, getter *trmpgx.CtxGetter) *CommentStorage {
	return &CommentStorage{pool: pool, getter: getter}
}

type CreateCommentRequest struct {
	PostID   int64 `validate:"required,gt=0"`
	ParentID *int64
	UserID   int64  `validate:"required,gt=0"`
	Body     string `validate:"required"`
}

func (s *CommentStorage) CreateComment(ctx context.Context, req CreateCommentRequest) (model.Comment, error) {
	var out model.Comment

	if err := validator.New().Struct(req); err != nil {
		return out, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	query, args, err := sq.
		Insert(tableinfo.CommentsTableName).
		Columns(
			tableinfo.CommentPostIDColumn,
			tableinfo.CommentParentIDColumn,
			tableinfo.CommentUserIDColumn,
			tableinfo.CommentBodyColumn,
		).
		Values(req.PostID, req.ParentID, req.UserID, req.Body).
		Suffix(fmt.Sprintf(
			"RETURNING %s, %s, %s, %s, %s, %s",
			tableinfo.CommentIDColumn,
			tableinfo.CommentPostIDColumn,
			tableinfo.CommentParentIDColumn,
			tableinfo.CommentUserIDColumn,
			tableinfo.CommentBodyColumn,
			tableinfo.CommentCreatedAtColumn,
		)).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return out, fmt.Errorf("%w: %v", ErrBuildingQuery, err)
	}

	tr := s.getter.DefaultTrOrDB(ctx, s.pool)
	if err := tr.QueryRow(ctx, query, args...).Scan(
		&out.ID,
		&out.PostID,
		&out.ParentID,
		&out.UserID,
		&out.Body,
		&out.CreatedAt,
	); err != nil {
		return out, fmt.Errorf("exec insert comment: %w", err)
	}

	return out, nil
}
