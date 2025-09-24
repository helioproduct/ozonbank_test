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

type PostStorage struct {
	mu    sync.RWMutex
	posts []model.Post
	byID  map[int64]model.Post
}

func NewPostStorage() *PostStorage {
	return &PostStorage{
		posts: []model.Post{model.Post{}},
		byID:  make(map[int64]model.Post),
	}
}

func (s *PostStorage) CreatePost(_ context.Context, in model.Post) (model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	in.ID = int64(len(s.posts))
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}
	s.posts = append(s.posts, in)
	s.byID[in.ID] = in
	return in, nil
}

func (s *PostStorage) GetPostByID(_ context.Context, postID int64) (model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if post, ok := s.byID[postID]; ok {
		return post, nil
	}
	return model.Post{}, service.ErrNotFound
}

func (s *PostStorage) SetCommentsEnabled(_ context.Context, postID int64, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.byID[postID]
	if !ok {
		return service.ErrNotFound
	}
	p.CommentsEnabled = enabled
	s.byID[postID] = p
	s.posts[postID] = p
	return nil
}

func (s *PostStorage) GetPosts(_ context.Context, limit int) ([]model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := len(s.posts) - 1
	if n <= 0 {
		return nil, nil
	}

	out := make([]model.Post, 0, min(limit, n))
	for id := n; id >= 1 && len(out) < limit; id-- {
		p := s.posts[id]
		if p.ID != 0 {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *PostStorage) GetPostsWithCursor(_ context.Context, params storage.GetPostsParams) ([]model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := params.Limit
	if limit <= 0 {
		limit = service.DefaultPostsLimit
	}

	out := make([]model.Post, 0, limit)

	switch params.Direction {
	case storage.DirectionAfter:
		for id := min(int(params.Cursor.ID)-1, len(s.posts)-1); id >= 1 && len(out) < limit; id-- {
			p := s.posts[id]
			if p.ID != 0 {
				out = append(out, p)
			}
		}
		return out, nil

	case storage.DirectionBefore:
		for id := max(int(params.Cursor.ID)+1, 1); id <= len(s.posts)-1 && len(out) < limit; id++ {
			p := s.posts[id]
			if p.ID != 0 {
				out = append(out, p)
			}
		}
		slices.Reverse(out)
		return out, nil

	default:
		return nil, errors.New("invalid keyset direction")
	}
}

func (s *PostStorage) GetPostAuthorID(_ context.Context, postID int64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.byID[postID]
	if !ok || p.ID == 0 {
		return 0, service.ErrNotFound
	}
	return p.UserID, nil
}
