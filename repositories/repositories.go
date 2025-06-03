package repositories

import (
	"database/sql"
	"gox2/posts"
	"gox2/users"
)

type Repositories struct {
	UserRepo *users.UserRepository
	PostRepo *posts.PostRepository
}

func Init(db *sql.DB) *Repositories {
	return &Repositories{
		UserRepo: users.NewUserRepository(db),
		PostRepo: posts.NewPostRepository(db),
	}
}
