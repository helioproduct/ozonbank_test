package postgres

import (
	"context"
	"errors"
	"fmt"
	"myreddit/internal/model"
	"time"

	"myreddit/pkg/tableinfo"

	sq "github.com/Masterminds/squirrel"
	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
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

func (s *CommentStorage) GetCommentByID(ctx context.Context, commentID int64) (model.Comment, error) {
	var out model.Comment

	query, args, err := sq.
		Select(
			tableinfo.CommentIDColumn,
			tableinfo.CommentPostIDColumn,
			tableinfo.CommentParentIDColumn,
			tableinfo.CommentUserIDColumn,
			tableinfo.CommentBodyColumn,
			tableinfo.CommentCreatedAtColumn,
		).
		From(tableinfo.CommentsTableName).
		Where(sq.Eq{tableinfo.CommentIDColumn: commentID}).
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
		if errors.Is(err, pgx.ErrNoRows) {
			return out, ErrNotFound
		}
		return out, fmt.Errorf("exec select comment by id: %w", err)
	}

	return out, nil
}

func (s *CommentStorage) GetCommentsByPost(ctx context.Context, postID int64, limit int) ([]model.Comment, error) {
	if limit <= 0 {
		limit = DefaultCommentsLimit
	}

	query, args, err := sq.
		Select(
			tableinfo.CommentIDColumn,
			tableinfo.CommentPostIDColumn,
			tableinfo.CommentParentIDColumn,
			tableinfo.CommentUserIDColumn,
			tableinfo.CommentBodyColumn,
			tableinfo.CommentCreatedAtColumn,
		).
		From(tableinfo.CommentsTableName).
		Where(sq.Eq{tableinfo.CommentPostIDColumn: postID}).
		OrderBy(
			fmt.Sprintf("%s DESC", tableinfo.CommentCreatedAtColumn),
			fmt.Sprintf("%s DESC", tableinfo.CommentIDColumn),
		).
		Limit(uint64(limit)).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBuildingQuery, err)
	}

	tr := s.getter.DefaultTrOrDB(ctx, s.pool)
	rows, err := tr.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec select comments: %w", err)
	}
	defer rows.Close()

	out := make([]model.Comment, 0, limit)
	for rows.Next() {
		var c model.Comment
		if err := rows.Scan(
			&c.ID,
			&c.PostID,
			&c.ParentID,
			&c.UserID,
			&c.Body,
			&c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return out, nil
}

type GetCommentsAfterRequest struct {
	PostID         int64     `validate:"required,gt=0"`
	AfterCreatedAt time.Time `validate:"required"`
	AfterID        int64     `validate:"gte=0"`
	Limit          int       `validate:"gt=0"`
}

func (s *CommentStorage) GetCommentsByPostAfter(ctx context.Context, req GetCommentsAfterRequest) ([]model.Comment, error) {
	if req.Limit <= 0 {
		req.Limit = DefaultCommentsLimit
	}

	if err := validator.New().Struct(req); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	query, args, err := sq.
		Select(
			tableinfo.CommentIDColumn,
			tableinfo.CommentPostIDColumn,
			tableinfo.CommentParentIDColumn,
			tableinfo.CommentUserIDColumn,
			tableinfo.CommentBodyColumn,
			tableinfo.CommentCreatedAtColumn,
		).
		From(tableinfo.CommentsTableName).
		Where(sq.And{
			sq.Eq{tableinfo.CommentPostIDColumn: req.PostID},
			sq.Expr(
				fmt.Sprintf("(%s, %s) < (?, ?)", tableinfo.CommentCreatedAtColumn, tableinfo.CommentIDColumn),
				req.AfterCreatedAt, req.AfterID,
			),
		}).
		OrderBy(
			fmt.Sprintf("%s DESC", tableinfo.CommentCreatedAtColumn),
			fmt.Sprintf("%s DESC", tableinfo.CommentIDColumn),
		).
		Limit(uint64(req.Limit)).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select comments after: %w", err)
	}

	tr := s.getter.DefaultTrOrDB(ctx, s.pool)
	rows, err := tr.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec select comments after: %w", err)
	}
	defer rows.Close()

	out := make([]model.Comment, 0, req.Limit)
	for rows.Next() {
		var c model.Comment
		if err := rows.Scan(
			&c.ID,
			&c.PostID,
			&c.ParentID,
			&c.UserID,
			&c.Body,
			&c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return out, nil
}
