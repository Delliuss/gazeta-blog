package posts

import (
	"database/sql"
	"sync"
)

type PostRepository struct {
	db *sql.DB
	mu sync.Mutex
}

func NewPostRepository(db *sql.DB) *PostRepository {
	return &PostRepository{db: db}
}
