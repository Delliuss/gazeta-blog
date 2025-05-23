package admin

import (
	"gox2/posts"
	"gox2/users"
)

type AdminRepository struct {
	userRepo *users.UserRepository
	postRepo *posts.PostRepository
}

func NewAdminRepository(userRepo *users.UserRepository, postRepo *posts.PostRepository) *AdminRepository {
	return &AdminRepository{
		userRepo: userRepo,
		postRepo: postRepo,
	}
}
