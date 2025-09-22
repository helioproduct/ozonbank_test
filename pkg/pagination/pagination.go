package pagination

type PageRequest struct {
	AfterCursor *string
	Limit       int
}

type Page[T any] struct {
	Count       int
	Elements    []T
	StartCursor *string
	EndCursor   *string
	HasNextPage bool
}
