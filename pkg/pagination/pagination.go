package pagination

type PageRequest struct {
	AfterCursor *string
	Limit       int
}

type Page[T any] struct {
	Count       int
	Items       []T
	EndCursor   *string
	HasNextPage bool
}
