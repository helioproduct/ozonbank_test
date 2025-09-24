package inmemory

import (
	"context"
	"fmt"
	"myreddit/internal/adapter/out/storage"
	"myreddit/internal/model"
	"myreddit/internal/service"
	"myreddit/pkg/pagination"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPostStorage_CreateAndGetByID(t *testing.T) {
	t.Parallel()

	st := NewPostStorage()

	tests := []struct {
		name    string
		input   model.Post
		wantID  int64
		wantErr error
	}{
		{
			name:   "first post",
			input:  model.Post{UserID: 1, Title: "t1", Text: "b1", CommentsEnabled: true},
			wantID: 1,
		},
		{
			name:   "second post",
			input:  model.Post{UserID: 2, Title: "t2", Text: "b2", CommentsEnabled: false},
			wantID: 2,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			out, err := st.CreatePost(context.Background(), tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.wantID, out.ID)
			require.Equal(t, tt.input.UserID, out.UserID)
			require.Equal(t, tt.input.Title, out.Title)
			require.Equal(t, tt.input.Text, out.Text)
			require.WithinDuration(t, time.Now(), out.CreatedAt, time.Second)

			got, err := st.GetPostByID(context.Background(), tt.wantID)
			require.NoError(t, err)
			require.Equal(t, out, got)
		})
	}
}

func TestPostStorage_GetPostByID_NotFound(t *testing.T) {
	t.Parallel()

	st := NewPostStorage()

	_, err := st.GetPostByID(context.Background(), 10)
	require.ErrorIs(t, err, service.ErrNotFound)
}

func TestPostStorage_SetCommentsEnabled(t *testing.T) {
	t.Parallel()

	st := NewPostStorage()

	require.ErrorIs(t, st.SetCommentsEnabled(context.Background(), 1, true), service.ErrNotFound)

	p, err := st.CreatePost(context.Background(), model.Post{
		UserID: 7, Title: "x", Text: "y", CommentsEnabled: false,
	})
	require.NoError(t, err)

	require.NoError(t, st.SetCommentsEnabled(context.Background(), p.ID, true))

	got, err := st.GetPostByID(context.Background(), p.ID)
	require.NoError(t, err)
	require.True(t, got.CommentsEnabled)

	require.NoError(t, st.SetCommentsEnabled(context.Background(), p.ID, false))
	got, err = st.GetPostByID(context.Background(), p.ID)
	require.NoError(t, err)
	require.False(t, got.CommentsEnabled)
}

func TestPostStorage_GetPosts_OrderDESC_and_Limit(t *testing.T) {
	t.Parallel()

	st := NewPostStorage()

	for i := 1; i <= 5; i++ {
		_, err := st.CreatePost(context.Background(), model.Post{
			UserID: int64(i), Title: "t", Text: "b", CommentsEnabled: true,
		})
		require.NoError(t, err)
	}

	got, err := st.GetPosts(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, int64(5), got[0].ID)
	require.Equal(t, int64(4), got[1].ID)
	require.Equal(t, int64(3), got[2].ID)

	st2 := NewPostStorage()
	list, err := st2.GetPosts(context.Background(), 10)
	require.NoError(t, err)
	require.Nil(t, list)
}

func TestPostStorage_GetPostsWithCursor_After_Before(t *testing.T) {
	t.Parallel()

	st := NewPostStorage()

	// [1, 2, 3, 4, 5]
	for i := 1; i <= 5; i++ {
		_, err := st.CreatePost(context.Background(), model.Post{
			UserID: int64(i), Title: "t", Text: "b", CommentsEnabled: true,
		})
		require.NoError(t, err)
	}

	// старге
	gotAfter, err := st.GetPostsWithCursor(context.Background(), storage.GetPostsParams{
		Cursor:    pagination.Cursor{ID: 4},
		Limit:     2,
		Direction: storage.DirectionAfter,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{3, 2}, collectIDs(gotAfter))

	// новее
	gotBefore, err := st.GetPostsWithCursor(context.Background(), storage.GetPostsParams{
		Cursor:    pagination.Cursor{ID: 2},
		Limit:     2,
		Direction: storage.DirectionBefore,
	})

	fmt.Println(collectIDs(gotBefore))

	require.NoError(t, err)
	require.Equal(t, []int64{4, 3}, collectIDs(gotBefore))
}

func TestPostStorage_GetPostsWithCursor_InvalidDirection(t *testing.T) {
	t.Parallel()

	st := NewPostStorage()

	_, err := st.GetPostsWithCursor(context.Background(), storage.GetPostsParams{
		Cursor: pagination.Cursor{ID: 10},
		Limit:  2,
	})
	require.Error(t, err)
}

func TestPostStorage_GetPostAuthorID(t *testing.T) {
	t.Parallel()

	st := NewPostStorage()

	_, err := st.GetPostAuthorID(context.Background(), 1)
	require.ErrorIs(t, err, service.ErrNotFound)

	p, err := st.CreatePost(context.Background(), model.Post{
		UserID: 99, Title: "a", Text: "b", CommentsEnabled: true,
	})
	require.NoError(t, err)

	uid, err := st.GetPostAuthorID(context.Background(), p.ID)
	require.NoError(t, err)
	require.Equal(t, int64(99), uid)
}

func collectIDs(posts []model.Post) []int64 {
	out := make([]int64, 0, len(posts))
	for _, p := range posts {
		out = append(out, p.ID)
	}
	return out
}
