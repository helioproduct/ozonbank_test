package postgres

import (
	"context"
	"errors"
	"myreddit/internal/adapter/out/storage"
	"myreddit/internal/adapter/out/storage/postgres/mocks"
	"myreddit/internal/model"
	"myreddit/internal/service"
	"myreddit/pkg/pagination"
	"testing"
	"time"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_getPostsQueryBuilder(t *testing.T) {
	cursor := pagination.Cursor{
		ID:        123,
		CreatedAt: time.Date(2025, 9, 24, 12, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name      string
		params    storage.GetPostsParams
		wantOrder string
		wantWhere []string
		wantErr   bool
	}{
		{
			name: "after cursor",
			params: storage.GetPostsParams{
				Cursor:    cursor,
				Direction: storage.DirectionAfter,
				Limit:     10,
			},
			wantOrder: "ORDER BY created_at DESC, id DESC",
			wantWhere: []string{"<", "created_at", "id"},
		},
		{
			name: "before cursor",
			params: storage.GetPostsParams{
				Cursor:    cursor,
				Direction: storage.DirectionBefore,
				Limit:     5,
			},
			wantOrder: "ORDER BY created_at ASC, id ASC",
			wantWhere: []string{">", "created_at", "id"},
		},
		{
			name: "invalid direction",
			params: storage.GetPostsParams{
				Cursor: cursor,
				Limit:  3,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qb, err := getPostsQueryBuilder(tt.params)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			sql, _, err := qb.ToSql()
			require.NoError(t, err)

			require.Contains(t, sql, tt.wantOrder)
			for _, w := range tt.wantWhere {
				require.Contains(t, sql, w)
			}
		})
	}
}

func TestPostStorage_CreatePost_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mocks.NewMockDB(ctrl)

	// ctx, sql, args
	mockDB.
		EXPECT().
		QueryRow(
			gomock.Any(),
			gomock.Any(),
			"hw", "wh", int64(4), true,
		).
		Return(fakeRow{
			// id, title, body, user_id, comments_enabled, created_at
			scan: func(dest ...any) error {
				*(dest[0].(*int64)) = 1
				*(dest[1].(*string)) = "hw"
				*(dest[2].(*string)) = "wh"
				*(dest[3].(*int64)) = 4
				*(dest[4].(*bool)) = true
				*(dest[5].(*time.Time)) = time.Now()
				return nil
			},
		})

	st := NewPostStorage(mockDB, trmpgx.DefaultCtxGetter)

	out, err := st.CreatePost(context.Background(), model.Post{
		Title:           "hw",
		Text:            "wh",
		UserID:          4,
		CommentsEnabled: true,
	})
	require.NoError(t, err)

	require.Equal(t, int64(1), out.ID)
	require.Equal(t, "hw", out.Title)
	require.Equal(t, "wh", out.Text)
	require.Equal(t, int64(4), out.UserID)
	require.True(t, out.CommentsEnabled)
	require.False(t, out.CreatedAt.IsZero())
}

func TestPostStorage_GetPostsWithCursor(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		params    storage.GetPostsParams
		setupMock func(m *mocks.MockDB)
		check     func(t *testing.T, got []model.Post, err error)
	}{
		{
			name: "success after",
			params: storage.GetPostsParams{
				Limit:     5,
				Direction: storage.DirectionAfter,
				Cursor:    pagination.Cursor{ID: 10, CreatedAt: now},
			},
			setupMock: func(m *mocks.MockDB) {
				rows := pgxmock.
					NewRows([]string{"id", "title", "body", "user_id", "comments_enabled", "created_at"}).
					AddRow(int64(1), "t1", "b1", int64(50), true, now).
					AddRow(int64(2), "t2", "b2", int64(4), true, now.Add(-time.Minute)).
					Kind()

				m.EXPECT().
					Query(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(rows, nil)
			},
			check: func(t *testing.T, got []model.Post, err error) {
				require.NoError(t, err)
				require.Len(t, got, 2)
				require.Equal(t, int64(1), got[0].ID)
				require.Equal(t, "t1", got[0].Title)
			},
		},
		{
			name: "query error",
			params: storage.GetPostsParams{
				Limit:     3,
				Direction: storage.DirectionAfter,
				Cursor:    pagination.Cursor{ID: 5, CreatedAt: now},
			},
			setupMock: func(m *mocks.MockDB) {
				m.EXPECT().
					Query(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("db fail"))
			},
			check: func(t *testing.T, got []model.Post, err error) {
				require.Error(t, err)
				require.Nil(t, got)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := mocks.NewMockDB(ctrl)
			tt.setupMock(mockDB)

			st := NewPostStorage(mockDB, trmpgx.DefaultCtxGetter)

			got, err := st.GetPostsWithCursor(context.Background(), tt.params)
			tt.check(t, got, err)
		})
	}
}

func TestPostStorage_CreatePost(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name  string
		input model.Post
		setup func(m *mocks.MockDB)
		check func(t *testing.T, got model.Post, err error)
	}{
		{
			name: "success",
			input: model.Post{
				Title:           "hw",
				Text:            "wh",
				UserID:          4,
				CommentsEnabled: true,
			},
			setup: func(m *mocks.MockDB) {
				m.EXPECT().
					QueryRow(
						gomock.Any(),               // ctx
						gomock.Any(),               // SQL от Squirrel
						"hw", "wh", int64(4), true, // args
					).
					Return(fakeRow{
						// RETURNING: id, title, body, user_id, comments_enabled, created_at
						scan: func(dest ...any) error {
							*(dest[0].(*int64)) = 1
							*(dest[1].(*string)) = "hw"
							*(dest[2].(*string)) = "wh"
							*(dest[3].(*int64)) = 4
							*(dest[4].(*bool)) = true
							*(dest[5].(*time.Time)) = now
							return nil
						},
					})
			},
			check: func(t *testing.T, got model.Post, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(1), got.ID)
				require.Equal(t, "hw", got.Title)
				require.Equal(t, "wh", got.Text)
				require.Equal(t, int64(4), got.UserID)
				require.True(t, got.CommentsEnabled)
				require.WithinDuration(t, now, got.CreatedAt, time.Second)
			},
		},
		{
			name: "db error (scan fails)",
			input: model.Post{
				Title:           "bad",
				Text:            "post",
				UserID:          1,
				CommentsEnabled: true,
			},
			setup: func(m *mocks.MockDB) {
				m.EXPECT().
					QueryRow(
						gomock.Any(),
						gomock.Any(),
						"bad", "post", int64(1), true,
					).
					Return(fakeRow{
						scan: func(dest ...any) error {
							return errors.New("db down")
						},
					})
			},
			check: func(t *testing.T, _ model.Post, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "exec error creating post")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := mocks.NewMockDB(ctrl)
			tt.setup(mockDB)

			st := NewPostStorage(mockDB, trmpgx.DefaultCtxGetter)
			got, err := st.CreatePost(context.Background(), tt.input)
			tt.check(t, got, err)
		})
	}
}

func TestPostStorage_GetPostsWithCursor_Before_Reverses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mocks.NewMockDB(ctrl)
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "title", "body", "user_id", "comments_enabled", "created_at",
	}).
		AddRow(int64(1), "t1", "b1", int64(50), true, now).
		AddRow(int64(2), "t2", "b2", int64(50), true, now.Add(time.Second)).
		Kind()

	mockDB.EXPECT().
		Query(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(rows, nil)

	st := NewPostStorage(mockDB, trmpgx.DefaultCtxGetter)

	out, err := st.GetPostsWithCursor(context.Background(), storage.GetPostsParams{
		Cursor: pagination.Cursor{
			ID:        123,
			CreatedAt: now,
		},
		Direction: storage.DirectionBefore,
		Limit:     2,
	})
	require.NoError(t, err)

	require.Len(t, out, 2)
	require.Equal(t, int64(2), out[0].ID) // reversed
	require.Equal(t, int64(1), out[1].ID)
}

func TestPostStorage_GetPostAuthorID_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		postID int64
		setup  func(m *mocks.MockDB, postID int64)
		check  func(t *testing.T, got int64, err error)
	}{
		{
			name:   "success",
			postID: 123,
			setup: func(m *mocks.MockDB, postID int64) {
				m.EXPECT().
					QueryRow(gomock.Any(), gomock.Any(), postID).
					Return(fakeRow{
						scan: func(dest ...any) error {
							*(dest[0].(*int64)) = 777 // author id
							return nil
						},
					})
			},
			check: func(t *testing.T, got int64, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(777), got)
			},
		},
		{
			name:   "not found",
			postID: 404,
			setup: func(m *mocks.MockDB, postID int64) {
				m.EXPECT().
					QueryRow(gomock.Any(), gomock.Any(), postID).
					Return(fakeRow{
						scan: func(dest ...any) error { return pgx.ErrNoRows },
					})
			},
			check: func(t *testing.T, _ int64, err error) {
				require.ErrorIs(t, err, service.ErrNotFound)
			},
		},
		{
			name:   "db error",
			postID: 500,
			setup: func(m *mocks.MockDB, postID int64) {
				m.EXPECT().
					QueryRow(gomock.Any(), gomock.Any(), postID).
					Return(fakeRow{
						scan: func(dest ...any) error { return errors.New("db down") },
					})
			},
			check: func(t *testing.T, _ int64, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "exec select author_id")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockDB(ctrl)
			tt.setup(m, tt.postID)

			st := NewPostStorage(m, trmpgx.DefaultCtxGetter)
			got, err := st.GetPostAuthorID(context.Background(), tt.postID)
			tt.check(t, got, err)
		})
	}
}

func TestPostStorage_SetCommentsEnabled(t *testing.T) {
	tests := []struct {
		name   string
		postID int64
		on     bool
		setup  func(m *mocks.MockDB, postID int64, on bool)
		check  func(t *testing.T, err error)
	}{
		{
			name:   "success",
			postID: 10,
			on:     true,
			setup: func(m *mocks.MockDB, postID int64, on bool) {
				// порядок аргументов: ctx, sql, enabled, postID
				m.EXPECT().
					QueryRow(gomock.Any(), gomock.Any(), on, postID).
					Return(fakeRow{
						// RETURNING id -> один Scan аргумент
						scan: func(dest ...any) error {
							*(dest[0].(*int64)) = postID
							return nil
						},
					})
			},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:   "not found",
			postID: 404,
			on:     false,
			setup: func(m *mocks.MockDB, postID int64, on bool) {
				m.EXPECT().
					QueryRow(gomock.Any(), gomock.Any(), on, postID).
					Return(fakeRow{
						scan: func(dest ...any) error { return pgx.ErrNoRows },
					})
			},
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, service.ErrNotFound)
			},
		},
		{
			name:   "db error",
			postID: 11,
			on:     true,
			setup: func(m *mocks.MockDB, postID int64, on bool) {
				m.EXPECT().
					QueryRow(gomock.Any(), gomock.Any(), on, postID).
					Return(fakeRow{
						scan: func(dest ...any) error { return errors.New("update failed") },
					})
			},
			check: func(t *testing.T, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "exec update comments_enabled")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockDB(ctrl)
			tt.setup(m, tt.postID, tt.on)

			st := NewPostStorage(m, trmpgx.DefaultCtxGetter)
			err := st.SetCommentsEnabled(context.Background(), tt.postID, tt.on)
			tt.check(t, err)
		})
	}
}

func TestPostStorage_GetPosts(t *testing.T) {
	now := time.Now()

	type setupFn func(m *mocks.MockDB)
	type checkFn func(t *testing.T, got []model.Post, err error)

	tests := []struct {
		name  string
		limit int
		setup setupFn
		check checkFn
	}{
		{
			name:  "success two rows (DESC as returned)",
			limit: 2,
			setup: func(m *mocks.MockDB) {
				rows := pgxmock.NewRows([]string{
					"id", "title", "body", "user_id", "comments_enabled", "created_at",
				}).
					// имитируем уже DESC порядок
					AddRow(int64(3), "t3", "b3", int64(7), true, now).
					AddRow(int64(2), "t2", "b2", int64(7), true, now.Add(-time.Minute)).
					Kind() // -> pgx.Rows

				// ВАЖНО: у GetPosts нет плейсхолдеров → Query(ctx, sql) => 2 аргумента
				m.EXPECT().
					Query(gomock.Any(), gomock.Any()).
					Return(rows, nil)
			},
			check: func(t *testing.T, got []model.Post, err error) {
				require.NoError(t, err)
				require.Len(t, got, 2)
				require.Equal(t, int64(3), got[0].ID)
				require.Equal(t, int64(2), got[1].ID)
			},
		},
		{
			name:  "limit<=0 uses default (capacity only) and still returns rows",
			limit: 0,
			setup: func(m *mocks.MockDB) {
				rows := pgxmock.NewRows([]string{
					"id", "title", "body", "user_id", "comments_enabled", "created_at",
				}).
					AddRow(int64(5), "t5", "b5", int64(7), true, now).
					Kind()

				m.EXPECT().
					Query(gomock.Any(), gomock.Any()).
					Return(rows, nil)
			},
			check: func(t *testing.T, got []model.Post, err error) {
				require.NoError(t, err)
				require.Len(t, got, 1)
				require.Equal(t, int64(5), got[0].ID)
			},
		},
		{
			name:  "query error",
			limit: 5,
			setup: func(m *mocks.MockDB) {
				m.EXPECT().
					Query(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("boom"))
			},
			check: func(t *testing.T, got []model.Post, err error) {
				require.Error(t, err)
				require.Nil(t, got)
				require.Contains(t, err.Error(), "exec error selecting posts")
			},
		},
		{
			name:  "scan error on second row",
			limit: 5,
			setup: func(m *mocks.MockDB) {
				rows := pgxmock.NewRows([]string{
					"id", "title", "body", "user_id", "comments_enabled", "created_at",
				}).
					AddRow(int64(2), "t2", "b2", int64(7), true, now).
					// испортим тип в created_at у второй строки → Scan упадёт
					AddRow(int64(1), "t1", "b1", int64(7), true, "bad_time").
					Kind()

				m.EXPECT().
					Query(gomock.Any(), gomock.Any()).
					Return(rows, nil)
			},
			check: func(t *testing.T, got []model.Post, err error) {
				require.Error(t, err)
				require.Nil(t, got)
				require.Contains(t, err.Error(), "scan error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := mocks.NewMockDB(ctrl)
			tt.setup(mockDB)

			st := NewPostStorage(mockDB, trmpgx.DefaultCtxGetter)

			got, err := st.GetPosts(context.Background(), tt.limit)
			tt.check(t, got, err)
		})
	}
}

type fakeRow struct{ scan func(dest ...any) error }

func (r fakeRow) Scan(dest ...any) error { return r.scan(dest...) }
