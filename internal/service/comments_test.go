package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"myreddit/internal/adapter/out/storage"
	"myreddit/internal/model"
	"myreddit/pkg/pagination"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCommentService_CreateComment(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name    string
		req     CreateCommentRequest
		setup   func(ms *MockCommentStorage, mb *MockCommentBus)
		wantErr error
	}{
		{
			name:    "validation error",
			req:     CreateCommentRequest{}, // пустой — провалит validator
			setup:   func(_ *MockCommentStorage, _ *MockCommentBus) {},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "storage error",
			req:  CreateCommentRequest{PostID: 10, UserID: 1, Body: "hi"},
			setup: func(ms *MockCommentStorage, _ *MockCommentBus) {
				ms.EXPECT().
					CreateComment(gomock.Any(), CreateCommentRequest{PostID: 10, UserID: 1, Body: "hi"}).
					Return(model.Comment{}, errors.New("db fail"))
			},
			wantErr: errors.New("db fail"),
		},
		{
			name: "success + publish",
			req:  CreateCommentRequest{PostID: 10, UserID: 2, Body: "ok"},
			setup: func(ms *MockCommentStorage, mb *MockCommentBus) {
				c := model.Comment{ID: 5, PostID: 10, UserID: 2, Body: "ok", CreatedAt: now}
				ms.EXPECT().
					CreateComment(gomock.Any(), CreateCommentRequest{PostID: 10, UserID: 2, Body: "ok"}).
					Return(c, nil)
				// Publish не возвращает ошибку в сервисе — просто ожидаем вызов
				mb.EXPECT().
					Publish(gomock.Any(), int64(10), c).
					Return(nil)
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ms := NewMockCommentStorage(ctrl)
			mb := NewMockCommentBus(ctrl)
			tt.setup(ms, mb)

			svc := NewCommentService(ms, mb)
			got, err := svc.CreateComment(context.Background(), tt.req)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, ErrInvalidRequest) {
					require.ErrorIs(t, err, ErrInvalidRequest)
				}
				return
			}

			require.NoError(t, err)
			require.NotZero(t, got.ID)
			require.Equal(t, tt.req.PostID, got.PostID)
			require.Equal(t, tt.req.Body, got.Body)
		})
	}
}

func TestCommentService_GetCommentByID(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		commentID int64
		setup     func(ms *MockCommentStorage)
		wantErr   error
	}{
		{
			name:      "invalid id",
			commentID: 0,
			setup:     func(_ *MockCommentStorage) {},
			wantErr:   ErrInvalidRequest,
		},
		{
			name:      "storage error",
			commentID: 7,
			setup: func(ms *MockCommentStorage) {
				ms.EXPECT().
					GetCommentByID(gomock.Any(), int64(7)).
					Return(model.Comment{}, errors.New("not found"))
			},
			wantErr: errors.New("not found"),
		},
		{
			name:      "success",
			commentID: 5,
			setup: func(ms *MockCommentStorage) {
				ms.EXPECT().
					GetCommentByID(gomock.Any(), int64(5)).
					Return(model.Comment{ID: 5, PostID: 10, Body: "ok", CreatedAt: now}, nil)
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ms := NewMockCommentStorage(ctrl)
			tt.setup(ms)

			svc := NewCommentService(ms, nil)
			got, err := svc.GetCommentByID(context.Background(), tt.commentID)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, ErrInvalidRequest) {
					require.ErrorIs(t, err, ErrInvalidRequest)
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.commentID, got.ID)
			require.WithinDuration(t, now, got.CreatedAt, time.Second)
		})
	}
}

func TestCommentService_GetCommentsByPost_NoCursors(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name          string
		req           pagination.PageRequest
		postID        int64
		mockItems     []model.Comment // peek = limit+1
		expectHasNext bool
		expectCount   int
	}{
		{
			name:   "has next page",
			postID: 10,
			req:    pagination.PageRequest{Limit: 2}, // peek=3
			mockItems: []model.Comment{
				{ID: 30, PostID: 10, Body: "a", CreatedAt: now},
				{ID: 20, PostID: 10, Body: "b", CreatedAt: now.Add(-time.Minute)},
				{ID: 10, PostID: 10, Body: "c", CreatedAt: now.Add(-2 * time.Minute)},
			},
			expectHasNext: true,
			expectCount:   2,
		},
		{
			name:   "no next page",
			postID: 10,
			req:    pagination.PageRequest{Limit: 3}, // peek=4
			mockItems: []model.Comment{
				{ID: 3, PostID: 10, Body: "a", CreatedAt: now},
				{ID: 2, PostID: 10, Body: "b", CreatedAt: now.Add(-time.Minute)},
			},
			expectHasNext: false,
			expectCount:   2,
		},
		{
			name:          "empty",
			postID:        10,
			req:           pagination.PageRequest{Limit: 5},
			mockItems:     nil,
			expectHasNext: false,
			expectCount:   0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ms := NewMockCommentStorage(ctrl)

			peek := tt.req.Limit + 1
			if tt.req.Limit <= 0 {
				peek = DefaultCommentsLimit + 1
			}
			if tt.req.Limit > MaxCommentsLimit {
				peek = MaxCommentsLimit + 1
			}

			ms.EXPECT().
				GetCommentsByPost(gomock.Any(), tt.postID, peek).
				Return(tt.mockItems, nil)

			svc := NewCommentService(ms, nil)
			page, err := svc.GetCommentsByPost(context.Background(), tt.req, tt.postID)
			require.NoError(t, err)

			require.Equal(t, tt.expectHasNext, page.HasNextPage)
			require.Equal(t, tt.expectCount, page.Count)

			if page.Count > 0 {
				start := pagination.Cursor{CreatedAt: page.Items[0].CreatedAt, ID: page.Items[0].ID}
				end := pagination.Cursor{CreatedAt: page.Items[len(page.Items)-1].CreatedAt, ID: page.Items[len(page.Items)-1].ID}
				require.Equal(t, start.Encode(), page.StartCursor)
				require.Equal(t, end.Encode(), page.EndCursor)
			} else {
				require.Nil(t, page.StartCursor)
				require.Nil(t, page.EndCursor)
			}
		})
	}
}

func TestCommentService_GetCommentsByPost_WithCursor(t *testing.T) {
	t.Parallel()

	now := time.Now()

	type capParams struct{ got storage.GetCommentsParams }

	tests := []struct {
		name        string
		postID      int64
		req         pagination.PageRequest
		setup       func(ms *MockCommentStorage, cap *capParams, ret []model.Comment)
		expectDir   storage.Direction
		expectCount int
	}{
		{
			name:   "after cursor",
			postID: 10,
			req: func() pagination.PageRequest {
				cur := pagination.Cursor{ID: 50, CreatedAt: now}
				enc := cur.Encode()
				return pagination.PageRequest{Limit: 2, AfterCursor: enc}
			}(),
			setup: func(ms *MockCommentStorage, cap *capParams, ret []model.Comment) {
				ms.EXPECT().
					GetCommentsByPostWithCursor(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p storage.GetCommentsParams) ([]model.Comment, error) {
						cap.got = p
						return ret, nil
					})
			},
			expectDir:   storage.DirectionAfter,
			expectCount: 2,
		},
		{
			name:   "before cursor",
			postID: 10,
			req: func() pagination.PageRequest {
				cur := pagination.Cursor{ID: 10, CreatedAt: now.Add(-time.Hour)}
				enc := cur.Encode()
				return pagination.PageRequest{Limit: 3, BeforeCursor: enc}
			}(),
			setup: func(ms *MockCommentStorage, cap *capParams, ret []model.Comment) {
				ms.EXPECT().
					GetCommentsByPostWithCursor(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p storage.GetCommentsParams) ([]model.Comment, error) {
						cap.got = p
						return ret, nil
					})
			},
			expectDir:   storage.DirectionBefore,
			expectCount: 3,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ms := NewMockCommentStorage(ctrl)
			cap := &capParams{}

			peek := tt.req.Limit + 1
			ret := make([]model.Comment, 0, peek)
			for i := 0; i < peek; i++ {
				ret = append(ret, model.Comment{
					ID:        int64(100 - i),
					PostID:    tt.postID,
					UserID:    1,
					Body:      "x",
					CreatedAt: now.Add(-time.Duration(i) * time.Minute),
				})
			}

			tt.setup(ms, cap, ret)

			svc := NewCommentService(ms, nil)
			page, err := svc.GetCommentsByPost(context.Background(), tt.req, tt.postID)
			require.NoError(t, err)

			// проверка параметров запроса
			require.Equal(t, peek, cap.got.Limit)
			require.Equal(t, tt.postID, cap.got.PostID)
			require.Equal(t, tt.expectDir, cap.got.Direction)

			// обрезка и курсоры
			require.True(t, page.HasNextPage)
			require.Equal(t, tt.expectCount, page.Count)
			require.NotNil(t, page.StartCursor)
			require.NotNil(t, page.EndCursor)
		})
	}
}

func TestCommentService_GetReplies_NoCursors(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name          string
		postID        int64
		parentID      int64
		req           pagination.PageRequest
		mockItems     []model.Comment
		expectHasNext bool
		expectCount   int
	}{
		{
			name:     "has next page",
			postID:   10,
			parentID: 1,
			req:      pagination.PageRequest{Limit: 2}, // peek=3
			mockItems: []model.Comment{
				{ID: 9, PostID: 10, ParentID: ptrI64(1), CreatedAt: now},
				{ID: 8, PostID: 10, ParentID: ptrI64(1), CreatedAt: now.Add(-time.Minute)},
				{ID: 7, PostID: 10, ParentID: ptrI64(1), CreatedAt: now.Add(-2 * time.Minute)},
			},
			expectHasNext: true,
			expectCount:   2,
		},
		{
			name:          "empty",
			postID:        10,
			parentID:      1,
			req:           pagination.PageRequest{Limit: 5},
			mockItems:     nil,
			expectHasNext: false,
			expectCount:   0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ms := NewMockCommentStorage(ctrl)

			peek := tt.req.Limit + 1
			if tt.req.Limit <= 0 {
				peek = DefaultCommentsLimit + 1
			}
			if tt.req.Limit > MaxCommentsLimit {
				peek = MaxCommentsLimit + 1
			}

			ms.EXPECT().
				GetReplies(gomock.Any(), tt.postID, tt.parentID, peek).
				Return(tt.mockItems, nil)

			svc := NewCommentService(ms, nil)
			page, err := svc.GetReplies(context.Background(), tt.req, tt.postID, tt.parentID)
			require.NoError(t, err)
			require.Equal(t, tt.expectHasNext, page.HasNextPage)
			require.Equal(t, tt.expectCount, page.Count)
		})
	}
}

func TestCommentService_GetReplies_WithCursor(t *testing.T) {
	t.Parallel()

	now := time.Now()

	type capParams struct{ got storage.GetRepliesParams }

	tests := []struct {
		name        string
		postID      int64
		parentID    int64
		req         pagination.PageRequest
		setup       func(ms *MockCommentStorage, cap *capParams, ret []model.Comment)
		expectDir   storage.Direction
		expectCount int
	}{
		{
			name:     "after cursor",
			postID:   10,
			parentID: 1,
			req: func() pagination.PageRequest {
				cur := pagination.Cursor{ID: 55, CreatedAt: now}
				enc := cur.Encode()
				return pagination.PageRequest{Limit: 2, AfterCursor: enc}
			}(),
			setup: func(ms *MockCommentStorage, cap *capParams, ret []model.Comment) {
				ms.EXPECT().
					GetRepliesWithCursor(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p storage.GetRepliesParams) ([]model.Comment, error) {
						cap.got = p
						return ret, nil
					})
			},
			expectDir:   storage.DirectionAfter,
			expectCount: 2,
		},
		{
			name:     "before cursor",
			postID:   10,
			parentID: 1,
			req: func() pagination.PageRequest {
				cur := pagination.Cursor{ID: 5, CreatedAt: now.Add(-time.Hour)}
				enc := cur.Encode()
				return pagination.PageRequest{Limit: 3, BeforeCursor: enc}
			}(),
			setup: func(ms *MockCommentStorage, cap *capParams, ret []model.Comment) {
				ms.EXPECT().
					GetRepliesWithCursor(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p storage.GetRepliesParams) ([]model.Comment, error) {
						cap.got = p
						return ret, nil
					})
			},
			expectDir:   storage.DirectionBefore,
			expectCount: 3,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ms := NewMockCommentStorage(ctrl)
			cap := &capParams{}

			peek := tt.req.Limit + 1
			ret := make([]model.Comment, 0, peek)
			for i := 0; i < peek; i++ {
				ret = append(ret, model.Comment{
					ID:        int64(200 - i),
					PostID:    tt.postID,
					ParentID:  ptrI64(tt.parentID),
					UserID:    1,
					Body:      "x",
					CreatedAt: now.Add(-time.Duration(i) * time.Minute),
				})
			}

			tt.setup(ms, cap, ret)

			svc := NewCommentService(ms, nil)
			page, err := svc.GetReplies(context.Background(), tt.req, tt.postID, tt.parentID)
			require.NoError(t, err)

			require.Equal(t, peek, cap.got.Limit)
			require.Equal(t, tt.postID, cap.got.PostID)
			require.Equal(t, tt.parentID, cap.got.ParentID)
			require.Equal(t, tt.expectDir, cap.got.Direction)

			require.True(t, page.HasNextPage)
			require.Equal(t, tt.expectCount, page.Count)
			require.NotNil(t, page.StartCursor)
			require.NotNil(t, page.EndCursor)
		})
	}
}

// helper
func ptrI64(v int64) *int64 { return &v }
