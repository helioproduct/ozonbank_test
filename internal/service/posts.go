package service

type PostStorage interface {
}

type PostService struct {
}

func NewPostService() *PostService {
	return &PostService{}
}
