package inmemory

import (
	"context"
	"sync"

	"myreddit/internal/model"
)

type CommentBus struct {
	mu sync.RWMutex
	// postID -> set каналов
	subs map[int64]map[chan model.Comment]struct{}
	buf  int
}

func New(buf int) *CommentBus {
	if buf <= 0 {
		buf = 64
	}
	return &CommentBus{
		subs: make(map[int64]map[chan model.Comment]struct{}),
		buf:  buf,
	}
}

// var _ service.CommentBus = (*CommentBus)(nil)

func (b *CommentBus) Subscribe(ctx context.Context, postID int64) (<-chan model.Comment, error) {
	ch := make(chan model.Comment, b.buf)

	b.mu.Lock()
	if b.subs[postID] == nil {
		b.subs[postID] = make(map[chan model.Comment]struct{})
	}
	b.subs[postID][ch] = struct{}{}
	b.mu.Unlock()

	// closing resourses
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		if set := b.subs[postID]; set != nil {
			if _, ok := set[ch]; ok {
				delete(set, ch)
				if len(set) == 0 {
					delete(b.subs, postID)
				}
			}
		}
		b.mu.Unlock()
		close(ch)
	}()

	return ch, nil
}

func (b *CommentBus) Publish(_ context.Context, postID int64, c model.Comment) error {
	b.mu.RLock()
	set := b.subs[postID]

	// для всех подписчиков подписанных на пост рассылаем комментарий
	for ch := range set {
		select {
		case ch <- c:
		default:
		}
	}
	b.mu.RUnlock()
	return nil
}
