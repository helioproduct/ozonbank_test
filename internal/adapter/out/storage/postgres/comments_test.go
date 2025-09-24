package postgres

import (
	"context"
	"errors"
	"myreddit/internal/adapter/out/storage"
	"myreddit/internal/adapter/out/storage/postgres/mocks"
	"myreddit/internal/model"
	"myreddit/internal/service"
	"myreddit/pkg/pagination"
	"myreddit/pkg/tableinfo"
	"testing"
	"time"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_getCommentsQueryBuilder(t *testing.T) {
	cur := pagination.Cursor{
		ID:        321,
		CreatedAt: time.Date(2025, 9, 24, 12, 0, 0, 0, time.UTC),
	}
	tests := []struct {
		name      string
		params    storage.GetCommentsParams
		wantOrder string
		wantOps   []string
		wantErr   bool
	}{
		{
			name: "after",
			params: storage.GetCommentsParams{
				PostID:    10,
				Cursor:    cur,
				Direction: storage.DirectionAfter,
				Limit:     20,
			},
			wantOrder: "ORDER BY " + tableinfo.CommentCreatedAtColumn + " DESC, " + tableinfo.CommentIDColumn + " DESC",
			wantOps:   []string{"<", tableinfo.CommentCreatedAtColumn, tableinfo.CommentIDColumn},
		},
		{
			name: "before",
			params: storage.GetCommentsParams{
				PostID:    10,
				Cursor:    cur,
				Direction: storage.DirectionBefore,
				Limit:     5,
			},
			wantOrder: "ORDER BY " + tableinfo.CommentCreatedAtColumn + " ASC, " + tableinfo.CommentIDColumn + " ASC",
			wantOps:   []string{">", tableinfo.CommentCreatedAtColumn, tableinfo.CommentIDColumn},
		},
		{
			name: "invalid direction",
			params: storage.GetCommentsParams{
				PostID: 10, Cursor: cur, Limit: 3,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qb, err := getCommentsQueryBuilder(tt.params)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			sql, _, err := qb.ToSql()
			require.NoError(t, err)

			require.Contains(t, sql, tt.wantOrder)
			for _, s := range tt.wantOps {
				require.Contains(t, sql, s)
			}
		})
	}
}

func Test_getRepliesQueryBuilder(t *testing.T) {
	cur := pagination.Cursor{ID: 11, CreatedAt: time.Now()}
	tests := []struct {
		name      string
		params    storage.GetRepliesParams
		wantOrder string
		wantOps   []string
		wantErr   bool
	}{
		{
			name: "after",
			params: storage.GetRepliesParams{
				PostID: 77, ParentID: 5, Cursor: cur, Direction: storage.DirectionAfter, Limit: 10,
			},
			wantOrder: "ORDER BY " + tableinfo.CommentCreatedAtColumn + " DESC, " + tableinfo.CommentIDColumn + " DESC",
			wantOps:   []string{"<", tableinfo.CommentCreatedAtColumn, tableinfo.CommentIDColumn},
		},
		{
			name: "before",
			params: storage.GetRepliesParams{
				PostID: 77, ParentID: 5, Cursor: cur, Direction: storage.DirectionBefore, Limit: 10,
			},
			wantOrder: "ORDER BY " + tableinfo.CommentCreatedAtColumn + " ASC, " + tableinfo.CommentIDColumn + " ASC",
			wantOps:   []string{">", tableinfo.CommentCreatedAtColumn, tableinfo.CommentIDColumn},
		},
		{
			name: "invalid",
			params: storage.GetRepliesParams{
				PostID: 77, ParentID: 5, Cursor: cur, Limit: 10,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qb, err := getRepliesQueryBuilder(tt.params)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			sql, _, err := qb.ToSql()
			require.NoError(t, err)

			require.Contains(t, sql, tt.wantOrder)
			for _, s := range tt.wantOps {
				require.Contains(t, sql, s)
			}
			require.Contains(t, sql, tableinfo.CommentPostIDColumn)
			require.Contains(t, sql, tableinfo.CommentParentIDColumn)
		})
	}
}

func TestCommentStorage_CreateComment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockDB(ctrl)
	now := time.Now()

	m.EXPECT().
		QueryRow(gomock.Any(), gomock.Any(), int64(1), gomock.Nil(), int64(50), "hello").
		Return(fakeRow{
			scan: func(dest ...any) error {
				*(dest[0].(*int64)) = 1001
				*(dest[1].(*int64)) = 1
				*(dest[2].(**int64)) = nil
				*(dest[3].(*int64)) = 50
				*(dest[4].(*string)) = "hello"
				*(dest[5].(*time.Time)) = now
				return nil
			},
		})

	st := NewCommentStorage(m, trmpgx.DefaultCtxGetter)
	out, err := st.CreateComment(context.Background(), service.CreateCommentRequest{
		PostID: 1, ParentID: nil, UserID: 50, Text: "hello",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1001), out.ID)
	require.Equal(t, "hello", out.Body)
	require.WithinDuration(t, now, out.CreatedAt, time.Second)
}

func TestCommentStorage_CreateComment_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockDB(ctrl)
	m.EXPECT().
		QueryRow(gomock.Any(), gomock.Any(), int64(1), gomock.Nil(), int64(7), "boom").
		Return(fakeRow{scan: func(dest ...any) error { return errors.New("insert failed") }})

	st := NewCommentStorage(m, trmpgx.DefaultCtxGetter)
	_, err := st.CreateComment(context.Background(), service.CreateCommentRequest{
		PostID: 1, UserID: 7, Text: "boom",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exec insert comment")
}

func TestCommentStorage_GetCommentByID(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		setup func(m *mocks.MockDB)
		check func(t *testing.T, c model.Comment, err error)
	}{
		{
			name: "success",
			setup: func(m *mocks.MockDB) {
				m.EXPECT().
					QueryRow(gomock.Any(), gomock.Any(), int64(5)).
					Return(fakeRow{
						scan: func(dest ...any) error {
							*(dest[0].(*int64)) = 5
							*(dest[1].(*int64)) = 10
							*(dest[2].(**int64)) = nil
							*(dest[3].(*int64)) = 77
							*(dest[4].(*string)) = "ok"
							*(dest[5].(*time.Time)) = now
							return nil
						},
					})
			},
			check: func(t *testing.T, c model.Comment, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(5), c.ID)
				require.Equal(t, "ok", c.Body)
			},
		},
		{
			name: "not found",
			setup: func(m *mocks.MockDB) {
				m.EXPECT().
					QueryRow(gomock.Any(), gomock.Any(), int64(404)).
					Return(fakeRow{scan: func(dest ...any) error { return pgx.ErrNoRows }})
			},
			check: func(t *testing.T, _ model.Comment, err error) {
				require.ErrorIs(t, err, service.ErrNotFound)
			},
		},
		{
			name: "db error",
			setup: func(m *mocks.MockDB) {
				m.EXPECT().
					QueryRow(gomock.Any(), gomock.Any(), int64(1)).
					Return(fakeRow{scan: func(dest ...any) error { return errors.New("db down") }})
			},
			check: func(t *testing.T, _ model.Comment, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "exec select comment by id")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := mocks.NewMockDB(ctrl)
			tt.setup(m)
			st := NewCommentStorage(m, trmpgx.DefaultCtxGetter)
			got, err := st.GetCommentByID(context.Background(), map[string]int64{"success": 5, "not found": 404, "db error": 1}[tt.name])
			tt.check(t, got, err)
		})
	}
}

func TestCommentStorage_GetCommentsByPost_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockDB(ctrl)

	now := time.Now()
	rows := pgxmock.NewRows([]string{
		"id", "post_id", "parent_id", "user_id", "body", "created_at",
	}).
		AddRow(int64(3), int64(10), nil, int64(7), "c3", now).
		AddRow(int64(2), int64(10), nil, int64(7), "c2", now.Add(-time.Minute)).
		Kind()

	// у функции есть плейсхолдеры → Query(ctx, sql, args...)
	m.EXPECT().
		Query(gomock.Any(), gomock.Any(), int64(10)).
		Return(rows, nil)

	st := NewCommentStorage(m, trmpgx.DefaultCtxGetter)
	got, err := st.GetCommentsByPost(context.Background(), 10, 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, int64(3), got[0].ID)
	require.Equal(t, "c2", got[1].Body)
}

func TestCommentStorage_GetCommentsByPost_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockDB(ctrl)

	m.EXPECT().
		Query(gomock.Any(), gomock.Any(), int64(10)).
		Return(nil, errors.New("boom"))

	st := NewCommentStorage(m, trmpgx.DefaultCtxGetter)
	got, err := st.GetCommentsByPost(context.Background(), 10, 5)
	require.Error(t, err)
	require.Nil(t, got)
	require.Contains(t, err.Error(), "exec select comments")
}

func TestCommentStorage_GetCommentsByPost_ScanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockDB(ctrl)

	rows := pgxmock.NewRows([]string{
		"id", "post_id", "parent_id", "user_id", "body", "created_at",
	}).
		AddRow(int64(1), int64(10), nil, int64(7), "ok", time.Now()).
		AddRow(int64(2), int64(10), nil, int64(7), "bad", "oops").
		Kind()

	m.EXPECT().
		Query(gomock.Any(), gomock.Any(), int64(10)).
		Return(rows, nil)

	st := NewCommentStorage(m, trmpgx.DefaultCtxGetter)
	got, err := st.GetCommentsByPost(context.Background(), 10, 5)
	require.Error(t, err)
	require.Nil(t, got)
	require.Contains(t, err.Error(), "scan comment")
}

func TestCommentStorage_GetCommentsByPostWithCursor_After_Before(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		params    storage.GetCommentsParams
		setupMock func(m *mocks.MockDB)
		expectIDs []int64
	}{
		{
			name: "after: DESC как пришло",
			params: storage.GetCommentsParams{
				PostID: 10, Limit: 3, Direction: storage.DirectionAfter,
				Cursor: pagination.Cursor{ID: 5, CreatedAt: now},
			},
			setupMock: func(m *mocks.MockDB) {
				rows := pgxmock.NewRows([]string{"id", "post_id", "parent_id", "user_id", "body", "created_at"}).
					AddRow(int64(9), int64(10), nil, int64(1), "a", now).
					AddRow(int64(8), int64(10), nil, int64(1), "b", now.Add(-time.Minute)).
					Kind()

				m.EXPECT().
					Query(
						gomock.Any(),
						gomock.Any(),
						int64(10),
						now,
						now,
						int64(5),
					).
					Return(rows, nil)
			},
			expectIDs: []int64{9, 8},
		},
		{
			name: "before: должен развернуть ASC->DESC",
			params: storage.GetCommentsParams{
				PostID: 10, Limit: 2, Direction: storage.DirectionBefore,
				Cursor: pagination.Cursor{ID: 5, CreatedAt: now},
			},
			setupMock: func(m *mocks.MockDB) {
				rows := pgxmock.NewRows([]string{"id", "post_id", "parent_id", "user_id", "body", "created_at"}).
					AddRow(int64(6), int64(10), nil, int64(2), "x", now).
					AddRow(int64(7), int64(10), nil, int64(2), "y", now.Add(time.Second)).
					Kind()

				m.EXPECT().
					Query(
						gomock.Any(),
						gomock.Any(),
						int64(10),
						now,
						now,
						int64(5),
					).
					Return(rows, nil)
			},
			expectIDs: []int64{7, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockDB(ctrl)
			tt.setupMock(m)

			st := NewCommentStorage(m, trmpgx.DefaultCtxGetter)
			got, err := st.GetCommentsByPostWithCursor(context.Background(), tt.params)
			require.NoError(t, err)

			var ids []int64
			for _, c := range got {
				ids = append(ids, c.ID)
			}
			require.Equal(t, tt.expectIDs, ids)
		})
	}
}
