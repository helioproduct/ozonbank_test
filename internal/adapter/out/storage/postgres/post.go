package postgres

import (
	"context"
	"errors"
	"fmt"
	"myreddit/internal/adapter/out/storage"
	"myreddit/internal/model"
	"myreddit/internal/service"
	"myreddit/pkg/tableinfo"
	"slices"

	sq "github.com/Masterminds/squirrel"
	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBuildingQuery = errors.New("error building sql-query")
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

func (s *PostStorage) CreatePost(ctx context.Context, post model.Post) (model.Post, error) {
	var out model.Post

	query, args, err := sq.
		Insert(tableinfo.PostsTableName).
		Columns(
			tableinfo.PostTitleColumn,
			tableinfo.PostBodyColumn,
			tableinfo.PostUserIDColumn,
			tableinfo.PostCommentsEnabledColumn,
		).
		Values(post.Title, post.Text, post.UserID, post.CommentsEnabled).
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
		&out.Text,
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
		&out.Text,
		&out.UserID,
		&out.CommentsEnabled,
		&out.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, service.ErrNotFound
		}
		return out, fmt.Errorf("exec select post by id: %w", err)
	}

	return out, nil
}

func (s *PostStorage) GetPosts(ctx context.Context, limit int) ([]model.Post, error) {
	if limit <= 0 {
		limit = service.DefaultPostsLimit
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
			&p.Text,
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

func (s *PostStorage) GetPostsWithCursor(ctx context.Context, params storage.GetPostsParams) ([]model.Post, error) {
	qb, err := getPostsQueryBuilder(params)
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
		return nil, fmt.Errorf("exec select: %w", err)
	}
	defer rows.Close()

	limit := params.Limit
	if limit <= 0 {
		limit = service.DefaultPostsLimit
	}

	out := make([]model.Post, 0, limit)
	for rows.Next() {
		var p model.Post
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.Text,
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

	if params.Direction == storage.DirectionBefore {
		slices.Reverse(out)
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
			return 0, service.ErrNotFound
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
			return service.ErrNotFound
		}
		return fmt.Errorf("exec update comments_enabled: %w", err)
	}
	return nil
}

func getPostsQueryBuilder(params storage.GetPostsParams) (sq.SelectBuilder, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = service.DefaultPostsLimit
	}

	base := sq.
		Select(
			tableinfo.PostIDColumn,
			tableinfo.PostTitleColumn,
			tableinfo.PostBodyColumn,
			tableinfo.PostUserIDColumn,
			tableinfo.PostCommentsEnabledColumn,
			tableinfo.PostCreatedAtColumn,
		).
		From(tableinfo.PostsTableName).
		PlaceholderFormat(sq.Dollar)

	createdAt := tableinfo.PostCreatedAtColumn
	idCol := tableinfo.PostIDColumn

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
		// (created_at, id)> (cursor.CreatedAt, cursor.ID)
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

	return sq.SelectBuilder{}, fmt.Errorf("invalid keyset: direction must be set")
}
