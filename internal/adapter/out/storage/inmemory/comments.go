package inmemory

import (
	"context"
	"errors"
	"myreddit/internal/adapter/out/storage"
	"myreddit/internal/model"
	"myreddit/internal/service"
	"slices"
	"sync"
	"time"
)

type CommentStorage struct {
	mu sync.RWMutex

	comments []model.Comment
	byPost   map[int64][]int64
	byParent map[int64][]int64
}

func NewCommentStorage() *CommentStorage {
	return &CommentStorage{
		comments: []model.Comment{{}},
		byPost:   make(map[int64][]int64),
		byParent: make(map[int64][]int64),
	}
}

func (s *CommentStorage) CreateComment(_ context.Context, req service.CreateCommentRequest) (model.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c := model.Comment{
		ID:        int64(len(s.comments)),
		PostID:    req.PostID,
		UserID:    req.UserID,
		Body:      req.Text,
		CreatedAt: time.Now(),
	}
	if req.ParentID != nil {
		pid := *req.ParentID
		c.ParentID = &pid
	}

	s.comments = append(s.comments, c)
	s.byPost[c.PostID] = append(s.byPost[c.PostID], c.ID)
	if c.ParentID != nil {
		s.byParent[*c.ParentID] = append(s.byParent[*c.ParentID], c.ID)
	}

	return c, nil
}

func (s *CommentStorage) GetCommentByID(_ context.Context, commentID int64) (model.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if commentID <= 0 || int(commentID) >= len(s.comments) {
		return model.Comment{}, service.ErrNotFound
	}
	c := s.comments[commentID]
	if c.ID == 0 {
		return model.Comment{}, service.ErrNotFound
	}
	return c, nil
}

func (s *CommentStorage) GetCommentsByPost(_ context.Context, postID int64, limit int) ([]model.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.byPost[postID]
	if len(ids) == 0 {
		return nil, nil
	}

	out := make([]model.Comment, 0, min(limit, len(ids)))
	for i := len(ids) - 1; i >= 0 && len(out) < limit; i-- {
		id := ids[i]
		out = append(out, s.comments[id])
	}
	return out, nil
}

func (s *CommentStorage) GetReplies(_ context.Context, postID, parentID int64, limit int) ([]model.Comment, error) {
	if limit <= 0 {
		limit = service.DefaultCommentsLimit
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	childIDs := s.byParent[parentID]
	if len(childIDs) == 0 {
		return nil, nil
	}

	out := make([]model.Comment, 0, min(limit, len(childIDs)))
	for i := len(childIDs) - 1; i >= 0 && len(out) < limit; i-- {
		id := childIDs[i]
		c := s.comments[id]
		if c.PostID == postID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *CommentStorage) GetCommentsByPostWithCursor(_ context.Context, p storage.GetCommentsParams) ([]model.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.byPost[p.PostID]
	if len(ids) == 0 {
		return nil, nil
	}

	out := make([]model.Comment, 0, p.Limit)
	switch p.Direction {
	case storage.DirectionAfter:
		for i := len(ids) - 1; i >= 0 && len(out) < p.Limit; i-- {
			id := ids[i]
			if id < p.Cursor.ID {
				out = append(out, s.comments[id])
			}
		}
		return out, nil

	case storage.DirectionBefore:
		for i := 0; i < len(ids) && len(out) < p.Limit; i++ {
			id := ids[i]
			if id > p.Cursor.ID {
				out = append(out, s.comments[id])
			}
		}
		slices.Reverse(out)
		return out, nil

	default:
		return nil, errors.New("invalid keyset direction")
	}
}

func (s *CommentStorage) GetRepliesWithCursor(_ context.Context, p storage.GetRepliesParams) ([]model.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	childIDs := s.byParent[p.ParentID]
	if len(childIDs) == 0 {
		return nil, nil
	}

	out := make([]model.Comment, 0, p.Limit)
	switch p.Direction {
	case storage.DirectionAfter:
		for i := len(childIDs) - 1; i >= 0 && len(out) < p.Limit; i-- {
			id := childIDs[i]
			if id < p.Cursor.ID {
				c := s.comments[id]
				if c.PostID == p.PostID {
					out = append(out, c)
				}
			}
		}
		return out, nil

	case storage.DirectionBefore:
		for i := 0; i < len(childIDs) && len(out) < p.Limit; i++ {
			id := childIDs[i]
			if id > p.Cursor.ID {
				c := s.comments[id]
				if c.PostID == p.PostID {
					out = append(out, c)
				}
			}
		}
		slices.Reverse(out)
		return out, nil

	default:
		return nil, errors.New("invalid keyset direction")
	}
}
