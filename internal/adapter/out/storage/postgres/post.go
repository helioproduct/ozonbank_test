package postgres

import (
	"context"
	"errors"
	"fmt"
	"myreddit/internal/model"
	"myreddit/pkg/tableinfo"
	"time"

	sq "github.com/Masterminds/squirrel"
	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultPostsLimit = 50
)

var (
	ErrBuildingQuery  = errors.New("error building sql-query")
	ErrInvalidRequest = errors.New("invalid request")
	ErrNotFound       = errors.New("not found")
)

type PostStorage struct {
	pool   *pgxpool.Pool
	getter *trmpgx.CtxGetter
}

func NewPostStorage(pool *pgxpool.Pool, getter *trmpgx.CtxGetter) *PostStorage {
	return &PostStorage{
		pool:   pool,
		getter: getter,
	}
}

type CreatePostRequest struct {
	UserID          int64  `validate:"required,gt=0"`
	Title           string `validate:"required"`
	Text            string `validate:"required"`
	CommentsEnabled bool
}

func (s *PostStorage) CreatePost(ctx context.Context, req CreatePostRequest) (model.Post, error) {
	var out model.Post

	if err := validator.New().Struct(req); err != nil {
		return model.Post{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	query, args, err := sq.
		Insert(tableinfo.PostsTableName).
		Columns(
			tableinfo.PostTitleColumn,
			tableinfo.PostBodyColumn,
			tableinfo.PostUserIDColumn,
			tableinfo.PostCommentsEnabledColumn,
		).
		Values(req.Title, req.Text, req.UserID, req.CommentsEnabled).
		Suffix(fmt.Sprintf("RETURNING %s, %s, %s, %s, %s, %s",
			tableinfo.PostIDColumn,
			tableinfo.PostTitleColumn,
			tableinfo.PostBodyColumn,
			tableinfo.PostUserIDColumn,
			tableinfo.PostCommentsEnabledColumn,
			tableinfo.PostCreatedAtColumn,
		)).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return out, fmt.Errorf("%w: %v", ErrBuildingQuery, err)
	}

	tr := s.getter.DefaultTrOrDB(ctx, s.pool)
	if err := tr.QueryRow(ctx, query, args...).Scan(
		&out.ID,
		&out.Title,
		&out.Body,
		&out.UserID,
		&out.CommentsEnabled,
		&out.CreatedAt,
	); err != nil {
		return out, fmt.Errorf("exec error creating post: %w", err)
	}

	return out, nil
}

func (s *PostStorage) GetPostByID(ctx context.Context, postID int64) (model.Post, error) {
	var out model.Post

	query, args, err := sq.
		Select(
			tableinfo.PostIDColumn,
			tableinfo.PostTitleColumn,
			tableinfo.PostBodyColumn,
			tableinfo.PostUserIDColumn,
			tableinfo.PostCommentsEnabledColumn,
			tableinfo.PostCreatedAtColumn,
		).
		From(tableinfo.PostsTableName).
		Where(sq.Eq{tableinfo.PostIDColumn: postID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return out, fmt.Errorf("%w: %v", ErrBuildingQuery, err)
	}

	tr := s.getter.DefaultTrOrDB(ctx, s.pool)

	if err := tr.QueryRow(ctx, query, args...).Scan(
		&out.ID,
		&out.Title,
		&out.Body,
		&out.UserID,
		&out.CommentsEnabled,
		&out.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, ErrNotFound
		}
		return out, fmt.Errorf("exec select post by id: %w", err)
	}

	return out, nil
}

func (s *PostStorage) GetPosts(ctx context.Context, limit int) ([]model.Post, error) {
	if limit <= 0 {
		limit = DefaultPostsLimit
	}
	query, args, err := sq.
		Select(
			tableinfo.PostIDColumn,
			tableinfo.PostTitleColumn,
			tableinfo.PostBodyColumn,
			tableinfo.PostUserIDColumn,
			tableinfo.PostCommentsEnabledColumn,
			tableinfo.PostCreatedAtColumn,
		).
		From(tableinfo.PostsTableName).
		OrderBy(
			fmt.Sprintf("%s DESC", tableinfo.PostCreatedAtColumn),
			fmt.Sprintf("%s DESC", tableinfo.PostIDColumn),
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
		return nil, fmt.Errorf("exec error selecting posts: %w", err)
	}
	defer rows.Close()

	out := make([]model.Post, 0, limit)
	for rows.Next() {
		var p model.Post
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.Body,
			&p.UserID,
			&p.CommentsEnabled,
			&p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return out, nil
}

type GetPostsAfterRequest struct {
	AfterCreatedAt time.Time `validate:"required"`
	AfterID        int64     `validate:"gte=0"`
	Limit          int       `validate:"gt=0"`
}

func (s *PostStorage) GetPostsAfter(ctx context.Context, req GetPostsAfterRequest) ([]model.Post, error) {
	if req.Limit <= 0 {
		req.Limit = DefaultPostsLimit
	}

	if err := validator.New().Struct(req); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	query, args, err := sq.
		Select(
			tableinfo.PostIDColumn,
			tableinfo.PostTitleColumn,
			tableinfo.PostBodyColumn,
			tableinfo.PostUserIDColumn,
			tableinfo.PostCommentsEnabledColumn,
			tableinfo.PostCreatedAtColumn,
		).
		From(tableinfo.PostsTableName).
		Where(
			sq.Expr(
				fmt.Sprintf("(%s, %s) < (?, ?)", tableinfo.PostCreatedAtColumn, tableinfo.PostIDColumn),
				req.AfterCreatedAt, req.AfterID,
			),
		).
		OrderBy(
			tableinfo.PostCreatedAtColumn+" DESC",
			tableinfo.PostIDColumn+" DESC",
		).
		Limit(uint64(req.Limit)).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select: %w", err)
	}

	tr := s.getter.DefaultTrOrDB(ctx, s.pool)
	rows, err := tr.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec select: %w", err)
	}
	defer rows.Close()

	out := make([]model.Post, 0, req.Limit)
	for rows.Next() {
		var p model.Post
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.Body,
			&p.UserID,
			&p.CommentsEnabled,
			&p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	return out, nil
}

func (s *PostStorage) GetPostAuthorID(ctx context.Context, postID int64) (int64, error) {
	query, args, err := sq.
		Select(tableinfo.PostUserIDColumn).
		From(tableinfo.PostsTableName).
		Where(sq.Eq{tableinfo.PostIDColumn: postID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrBuildingQuery, err)
	}

	tr := s.getter.DefaultTrOrDB(ctx, s.pool)

	var authorID int64
	if err := tr.QueryRow(ctx, query, args...).Scan(&authorID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("exec select author_id: %w", err)
	}
	return authorID, nil
}

func (s *PostStorage) SetCommentsEnabled(ctx context.Context, postID int64, enabled bool) error {
	query, args, err := sq.
		Update(tableinfo.PostsTableName).
		Set(tableinfo.PostCommentsEnabledColumn, enabled).
		Where(sq.Eq{tableinfo.PostIDColumn: postID}).
		Suffix(fmt.Sprintf("RETURNING %s", tableinfo.PostIDColumn)).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBuildingQuery, err)
	}

	tr := s.getter.DefaultTrOrDB(ctx, s.pool)

	var dummy int64
	if err := tr.QueryRow(ctx, query, args...).Scan(&dummy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("exec update comments_enabled: %w", err)
	}
	return nil
}
