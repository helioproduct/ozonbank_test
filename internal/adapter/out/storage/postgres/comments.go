package postgres

import (
	"context"
	"errors"
	"fmt"
	"myreddit/internal/adapter/out/storage"
	"myreddit/internal/model"
	"myreddit/internal/service"
	"slices"

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

func (s *CommentStorage) CreateComment(ctx context.Context, req service.CreateCommentRequest) (model.Comment, error) {
	var out model.Comment

	if err := validator.New().Struct(req); err != nil {
		return out, fmt.Errorf("%w: %v", service.ErrInvalidRequest, err)
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
			return out, service.ErrNotFound
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

func (s *CommentStorage) GetCommentsByPostWithCursor(ctx context.Context, params storage.GetCommentsParams) ([]model.Comment, error) {
	qb, err := getCommentsQueryBuilder(params)
	if err != nil {
		return nil, err
	}

	query, args, err := qb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBuildingQuery, err)
	}

	tr := s.getter.DefaultTrOrDB(ctx, s.pool)
	rows, err := tr.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec select comments: %w", err)
	}
	defer rows.Close()

	out := make([]model.Comment, 0, params.Limit)
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

	if params.Direction == storage.DirectionBefore {
		slices.Reverse(out)
	}
	return out, nil
}

func (s *CommentStorage) GetReplies(ctx context.Context, postID, parentID int64, limit int) ([]model.Comment, error) {
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
		Where(sq.Eq{
			tableinfo.CommentPostIDColumn:   postID,
			tableinfo.CommentParentIDColumn: parentID,
		}).
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
		return nil, fmt.Errorf("exec select replies: %w", err)
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
			return nil, fmt.Errorf("scan replies: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows replies: %w", err)
	}
	return out, nil
}

func (s *CommentStorage) GetRepliesWithCursor(ctx context.Context, params storage.GetRepliesParams) ([]model.Comment, error) {
	qb, err := getRepliesQueryBuilder(params)
	if err != nil {
		return nil, err
	}

	query, args, err := qb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBuildingQuery, err)
	}

	tr := s.getter.DefaultTrOrDB(ctx, s.pool)
	rows, err := tr.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec select replies after/before: %w", err)
	}
	defer rows.Close()

	out := make([]model.Comment, 0, params.Limit)
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
			return nil, fmt.Errorf("scan replies after/before: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows replies after/before: %w", err)
	}

	if params.Direction == storage.DirectionBefore {
		slices.Reverse(out)
	}

	return out, nil
}

func getCommentsQueryBuilder(params storage.GetCommentsParams) (sq.SelectBuilder, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = DefaultCommentsLimit
	}

	base := sq.
		Select(
			tableinfo.CommentIDColumn,
			tableinfo.CommentPostIDColumn,
			tableinfo.CommentParentIDColumn,
			tableinfo.CommentUserIDColumn,
			tableinfo.CommentBodyColumn,
			tableinfo.CommentCreatedAtColumn,
		).
		From(tableinfo.CommentsTableName).
		Where(sq.Eq{tableinfo.CommentPostIDColumn: params.PostID}).
		PlaceholderFormat(sq.Dollar)

	createdAt := tableinfo.CommentCreatedAtColumn
	idCol := tableinfo.CommentIDColumn

	if params.Direction == storage.DirectionAfter {
		sb := base.
			Where(sq.Or{
				sq.Lt{createdAt: params.Cursor.CreatedAt},
				sq.And{
					sq.Eq{createdAt: params.Cursor.CreatedAt},
					sq.Lt{idCol: params.Cursor.ID},
				},
			}).
			OrderBy(createdAt+" DESC", idCol+" DESC").
			Limit(uint64(limit))
		return sb, nil
	}

	if params.Direction == storage.DirectionBefore {
		sb := base.
			Where(sq.Or{
				sq.Gt{createdAt: params.Cursor.CreatedAt},
				sq.And{
					sq.Eq{createdAt: params.Cursor.CreatedAt},
					sq.Gt{idCol: params.Cursor.ID},
				},
			}).
			OrderBy(createdAt+" ASC", idCol+" ASC").
			Limit(uint64(limit))
		return sb, nil
	}

	return sq.SelectBuilder{}, fmt.Errorf("invalid keyset: direction must be set: %w", service.ErrInvalidRequest)
}
func getRepliesQueryBuilder(params storage.GetRepliesParams) (sq.SelectBuilder, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = DefaultCommentsLimit
	}

	base := sq.
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
			sq.Eq{tableinfo.CommentPostIDColumn: params.PostID},
			sq.Eq{tableinfo.CommentParentIDColumn: params.ParentID},
		}).
		PlaceholderFormat(sq.Dollar)

	createdAt := tableinfo.CommentCreatedAtColumn
	idCol := tableinfo.CommentIDColumn

	if params.Direction == storage.DirectionAfter {
		// (created_at, id) < (cursor.CreatedAt, cursor.ID)
		sb := base.
			Where(sq.Or{
				sq.Lt{createdAt: params.Cursor.CreatedAt},
				sq.And{
					sq.Eq{createdAt: params.Cursor.CreatedAt},
					sq.Lt{idCol: params.Cursor.ID},
				},
			}).
			OrderBy(createdAt+" DESC", idCol+" DESC").
			Limit(uint64(limit))
		return sb, nil
	}

	if params.Direction == storage.DirectionBefore {
		// (created_at, id) > (cursor.CreatedAt, cursor.ID)
		sb := base.
			Where(sq.Or{
				sq.Gt{createdAt: params.Cursor.CreatedAt},
				sq.And{
					sq.Eq{createdAt: params.Cursor.CreatedAt},
					sq.Gt{idCol: params.Cursor.ID},
				},
			}).
			OrderBy(createdAt+" ASC", idCol+" ASC").
			Limit(uint64(limit))
		return sb, nil
	}

	return sq.SelectBuilder{}, fmt.Errorf("invalid keyset: direction must be set: %w", service.ErrInvalidRequest)
}
