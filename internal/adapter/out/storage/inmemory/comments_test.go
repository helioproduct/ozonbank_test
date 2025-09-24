package inmemory

import (
	"context"
	"myreddit/internal/adapter/out/storage"
	"myreddit/internal/model"
	"myreddit/internal/service"
	"myreddit/pkg/pagination"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCommentStorage_CreateAndGetByID(t *testing.T) {
	t.Parallel()

	st := NewCommentStorage()

	root, err := st.CreateComment(context.Background(), service.CreateCommentRequest{
		PostID: 10, UserID: 1, Text: "root",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), root.ID)
	require.Nil(t, root.ParentID)
	require.Equal(t, int64(10), root.PostID)
	require.WithinDuration(t, time.Now(), root.CreatedAt, time.Second)

	parent := root.ID
	reply, err := st.CreateComment(context.Background(), service.CreateCommentRequest{
		PostID: 10, UserID: 2, Text: "reply", ParentID: &parent,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), reply.ID)
	require.NotNil(t, reply.ParentID)
	require.Equal(t, parent, *reply.ParentID)

	got1, err := st.GetCommentByID(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, root, got1)

	got2, err := st.GetCommentByID(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, reply, got2)
}

func TestCommentStorage_GetCommentByID_NotFound(t *testing.T) {
	t.Parallel()

	st := NewCommentStorage()
	_, err := st.GetCommentByID(context.Background(), 99)
	require.ErrorIs(t, err, service.ErrNotFound)
}

func TestCommentStorage_GetCommentsByPost_OrderDESC_and_Limit(t *testing.T) {
	t.Parallel()

	st := NewCommentStorage()

	for i := 0; i < 5; i++ {
		_, err := st.CreateComment(context.Background(), service.CreateCommentRequest{
			PostID: 10, UserID: 1, Text: "c",
		})
		require.NoError(t, err)
	}
	_, _ = st.CreateComment(context.Background(), service.CreateCommentRequest{
		PostID: 20, UserID: 1, Text: "x",
	})

	got, err := st.GetCommentsByPost(context.Background(), 10, 3)
	require.NoError(t, err)
	require.Equal(t, []int64{5, 4, 3}, collectCommentIDs(got))

	gotNil, err := st.GetCommentsByPost(context.Background(), 30, 10)
	require.NoError(t, err)
	require.Nil(t, gotNil)
}

func TestCommentStorage_GetCommentsByPostWithCursor_After_Before(t *testing.T) {
	t.Parallel()

	st := NewCommentStorage()

	for i := 0; i < 5; i++ {
		_, err := st.CreateComment(context.Background(), service.CreateCommentRequest{
			PostID: 10, UserID: 1, Text: "c",
		})
		require.NoError(t, err)
	}

	after, err := st.GetCommentsByPostWithCursor(context.Background(), storage.GetCommentsParams{
		PostID:    10,
		Cursor:    pagination.Cursor{ID: 4},
		Limit:     2,
		Direction: storage.DirectionAfter,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{3, 2}, collectCommentIDs(after))

	before, err := st.GetCommentsByPostWithCursor(context.Background(), storage.GetCommentsParams{
		PostID:    10,
		Cursor:    pagination.Cursor{ID: 2},
		Limit:     2,
		Direction: storage.DirectionBefore,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{4, 3}, collectCommentIDs(before))
}

func TestCommentStorage_GetRepliesWithCursor_After_Before(t *testing.T) {
	t.Parallel()

	st := NewCommentStorage()

	parent, err := st.CreateComment(context.Background(), service.CreateCommentRequest{
		PostID: 10, UserID: 1, Text: "p",
	})
	require.NoError(t, err)
	pid := parent.ID

	for i := 0; i < 5; i++ {
		_, err := st.CreateComment(context.Background(), service.CreateCommentRequest{
			PostID: 10, UserID: 2, Text: "r", ParentID: &pid,
		})
		require.NoError(t, err)
	}

	after, err := st.GetRepliesWithCursor(context.Background(), storage.GetRepliesParams{
		PostID:    10,
		ParentID:  pid,
		Cursor:    pagination.Cursor{ID: 5},
		Limit:     2,
		Direction: storage.DirectionAfter,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{4, 3}, collectCommentIDs(after))

	before, err := st.GetRepliesWithCursor(context.Background(), storage.GetRepliesParams{
		PostID:    10,
		ParentID:  pid,
		Cursor:    pagination.Cursor{ID: 3},
		Limit:     2,
		Direction: storage.DirectionBefore,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{5, 4}, collectCommentIDs(before))
}

func collectCommentIDs(in []model.Comment) []int64 {
	out := make([]int64, 0, len(in))
	for _, c := range in {
		out = append(out, c.ID)
	}
	return out
}
