package service

import (
	"context"
	"errors"
	"myreddit/internal/adapter/out/storage"
	"myreddit/internal/model"
	"myreddit/pkg/pagination"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPostService_CreatePost(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name    string
		req     CreatePostRequest
		setup   func(m *MockPostStorage)
		wantErr error
	}{
		{
			name:    "validation error",
			req:     CreatePostRequest{},
			setup:   func(_ *MockPostStorage) {},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "storage error",
			req:  CreatePostRequest{UserID: 7, Title: "t", Text: "x", CommentsEnabled: true},
			setup: func(m *MockPostStorage) {
				m.EXPECT().
					CreatePost(gomock.Any(), model.Post{
						UserID:          7,
						Title:           "t",
						Text:            "x",
						CommentsEnabled: true,
					}).
					Return(model.Post{}, errors.New("db fail"))
			},
			wantErr: errors.New("db fail"),
		},
		{
			name: "success",
			req:  CreatePostRequest{UserID: 7, Title: "t", Text: "x", CommentsEnabled: true},
			setup: func(m *MockPostStorage) {
				m.EXPECT().
					CreatePost(gomock.Any(), model.Post{
						UserID:          7,
						Title:           "t",
						Text:            "x",
						CommentsEnabled: true,
					}).
					Return(model.Post{ID: 10, UserID: 7, Title: "t", Text: "x", CommentsEnabled: true, CreatedAt: now}, nil)
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := NewMockPostStorage(ctrl)
			tt.setup(m)

			svc := NewPostService(m)
			got, err := svc.CreatePost(context.Background(), tt.req)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, ErrInvalidRequest) {
					require.ErrorIs(t, err, ErrInvalidRequest)
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, int64(10), got.ID)
			require.WithinDuration(t, now, got.CreatedAt, time.Second)
		})
	}
}

func TestPostService_GetPostByID(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name    string
		postID  int64
		setup   func(m *MockPostStorage)
		wantErr error
	}{
		{
			name:    "invalid id",
			postID:  0,
			setup:   func(_ *MockPostStorage) {},
			wantErr: ErrInvalidRequest,
		},
		{
			name:   "storage error",
			postID: 123,
			setup: func(m *MockPostStorage) {
				m.EXPECT().
					GetPostByID(gomock.Any(), int64(123)).
					Return(model.Post{}, errors.New("not found"))
			},
			wantErr: errors.New("not found"),
		},
		{
			name:   "success",
			postID: 5,
			setup: func(m *MockPostStorage) {
				m.EXPECT().
					GetPostByID(gomock.Any(), int64(5)).
					Return(model.Post{ID: 5, Title: "a", Text: "b", UserID: 1, CreatedAt: now}, nil)
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := NewMockPostStorage(ctrl)
			tt.setup(m)

			svc := NewPostService(m)
			got, err := svc.GetPostByID(context.Background(), tt.postID)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, ErrInvalidRequest) {
					require.ErrorIs(t, err, ErrInvalidRequest)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.postID, got.ID)
			require.WithinDuration(t, now, got.CreatedAt, time.Second)
		})
	}
}

func TestPostService_GetPosts_NoCursors(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name          string
		req           pagination.PageRequest
		mockPosts     []model.Post
		expectHasNext bool
		expectCount   int
	}{
		{
			name: "has next page (peek item present)",
			req:  pagination.PageRequest{Limit: 2},
			mockPosts: []model.Post{
				{ID: 30, CreatedAt: now},
				{ID: 20, CreatedAt: now.Add(-time.Minute)},
				{ID: 10, CreatedAt: now.Add(-2 * time.Minute)},
			},
			expectHasNext: true,
			expectCount:   2,
		},
		{
			name: "no next page (exact <= limit)",
			req:  pagination.PageRequest{Limit: 3},
			mockPosts: []model.Post{
				{ID: 3, CreatedAt: now},
				{ID: 2, CreatedAt: now.Add(-time.Minute)},
			},
			expectHasNext: false,
			expectCount:   2,
		},
		{
			name:          "empty list",
			req:           pagination.PageRequest{Limit: 5},
			mockPosts:     nil,
			expectHasNext: false,
			expectCount:   0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := NewMockPostStorage(ctrl)

			peek := tt.req.Limit + 1
			if tt.req.Limit <= 0 {
				peek = DefaultPostsLimit + 1
			}
			if tt.req.Limit > MaxPostsLimit {
				peek = MaxPostsLimit + 1
			}

			m.EXPECT().
				GetPosts(gomock.Any(), peek).
				Return(tt.mockPosts, nil)

			svc := NewPostService(m)
			page, err := svc.GetPosts(context.Background(), tt.req)
			require.NoError(t, err)
			require.Equal(t, tt.expectHasNext, page.HasNextPage)
			require.Equal(t, tt.expectCount, page.Count)

			if page.Count > 0 {
				start := pagination.Cursor{CreatedAt: tt.mockPosts[0].CreatedAt, ID: tt.mockPosts[0].ID}
				end := pagination.Cursor{CreatedAt: tt.mockPosts[min(tt.expectCount-1, len(tt.mockPosts)-1)].CreatedAt, ID: tt.
					mockPosts[min(tt.expectCount-1, len(tt.mockPosts)-1)].ID}
				require.Equal(t, start.Encode(), page.StartCursor)
				require.Equal(t, end.Encode(), page.EndCursor)
			} else {
				require.False(t, page.HasNextPage)
				require.Nil(t, page.StartCursor)
				require.Nil(t, page.EndCursor)
			}
		})
	}
}

func TestPostService_GetPosts_WithCursor(t *testing.T) {
	t.Parallel()

	now := time.Now()

	type capParams struct {
		got storage.GetPostsParams
	}

	tests := []struct {
		name        string
		req         pagination.PageRequest
		setup       func(m *MockPostStorage, cap *capParams, ret []model.Post)
		expectDir   storage.Direction
		expectCount int
	}{
		{
			name: "after cursor",
			req: func() pagination.PageRequest {
				cur := pagination.Cursor{ID: 100, CreatedAt: now}
				enc := cur.Encode()
				return pagination.PageRequest{Limit: 2, AfterCursor: enc}
			}(),
			setup: func(m *MockPostStorage, cap *capParams, ret []model.Post) {
				m.EXPECT().GetPostsWithCursor(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p storage.GetPostsParams) ([]model.Post, error) {
						cap.got = p
						return ret, nil
					})
			},
			expectDir:   storage.DirectionAfter,
			expectCount: 2,
		},
		{
			name: "before cursor",
			req: func() pagination.PageRequest {
				cur := pagination.Cursor{ID: 50, CreatedAt: now.Add(-time.Hour)}
				enc := cur.Encode()
				return pagination.PageRequest{Limit: 3, BeforeCursor: enc}
			}(),
			setup: func(m *MockPostStorage, cap *capParams, ret []model.Post) {
				m.EXPECT().GetPostsWithCursor(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p storage.GetPostsParams) ([]model.Post, error) {
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
			m := NewMockPostStorage(ctrl)
			cap := &capParams{}

			peek := tt.req.Limit + 1
			ret := make([]model.Post, 0, peek)
			for i := 0; i < peek; i++ {
				ret = append(ret, model.Post{
					ID:        int64(1000 - i),
					CreatedAt: now.Add(-time.Duration(i) * time.Minute),
					Title:     "t",
					Text:      "x",
					UserID:    1,
				})
			}
			tt.setup(m, cap, ret)

			svc := NewPostService(m)
			page, err := svc.GetPosts(context.Background(), tt.req)
			require.NoError(t, err)

			require.Equal(t, peek, cap.got.Limit)
			require.Equal(t, tt.expectDir, cap.got.Direction)

			require.True(t, page.HasNextPage)
			require.Equal(t, tt.expectCount, page.Count)

			start := pagination.Cursor{CreatedAt: page.Items[0].CreatedAt, ID: page.Items[0].ID}
			end := pagination.Cursor{CreatedAt: page.Items[len(page.Items)-1].CreatedAt, ID: page.Items[len(page.Items)-1].ID}
			require.Equal(t, start.Encode(), page.StartCursor)
			require.Equal(t, end.Encode(), page.EndCursor)
		})
	}
}

func TestPostService_ChangePostCommentPermission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		postID    int64
		userID    int64
		enabled   bool
		setup     func(m *MockPostStorage)
		wantError error
	}{
		{
			name:      "invalid args",
			postID:    0,
			userID:    1,
			setup:     func(_ *MockPostStorage) {},
			wantError: ErrInvalidRequest,
		},
		{
			name:   "get author error",
			postID: 10, userID: 1, enabled: true,
			setup: func(m *MockPostStorage) {
				m.EXPECT().GetPostAuthorID(gomock.Any(), int64(10)).
					Return(int64(0), errors.New("db fail"))
			},
			wantError: errors.New("db fail"),
		},
		{
			name:   "not owner -> forbidden",
			postID: 10, userID: 2, enabled: false,
			setup: func(m *MockPostStorage) {
				m.EXPECT().GetPostAuthorID(gomock.Any(), int64(10)).
					Return(int64(1), nil)
			},
			wantError: ErrForbidden,
		},
		{
			name:   "set comments error",
			postID: 10, userID: 1, enabled: true,
			setup: func(m *MockPostStorage) {
				m.EXPECT().GetPostAuthorID(gomock.Any(), int64(10)).
					Return(int64(1), nil)
				m.EXPECT().SetCommentsEnabled(gomock.Any(), int64(10), true).
					Return(errors.New("update fail"))
			},
			wantError: errors.New("update fail"),
		},
		{
			name:   "success",
			postID: 10, userID: 1, enabled: false,
			setup: func(m *MockPostStorage) {
				m.EXPECT().GetPostAuthorID(gomock.Any(), int64(10)).
					Return(int64(1), nil)
				m.EXPECT().SetCommentsEnabled(gomock.Any(), int64(10), false).
					Return(nil)
			},
			wantError: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := NewMockPostStorage(ctrl)
			tt.setup(m)

			svc := NewPostService(m)
			err := svc.ChangePostCommentPermission(context.Background(), tt.postID, tt.userID, tt.enabled)

			if tt.wantError != nil {
				require.Error(t, err)
				if errors.Is(tt.wantError, ErrInvalidRequest) {
					require.ErrorIs(t, err, ErrInvalidRequest)
				} else if errors.Is(tt.wantError, ErrForbidden) {
					require.ErrorIs(t, err, ErrForbidden)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}
