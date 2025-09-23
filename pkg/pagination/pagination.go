package pagination

type PageRequest struct {
	BeforeCursor *string
	AfterCursor  *string
	Limit        int
}

type Page[T any] struct {
	Count           int
	Items           []T
	StartCursor     *string
	EndCursor       *string
	HasNextPage     bool
	HasPreviousPage bool
}
