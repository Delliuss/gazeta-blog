package posts

import (
	"database/sql"
	"sync"
)

// PostRepository представляет репозиторий для работы с постами
type PostRepository struct {
	db *sql.DB
	mu sync.Mutex
}

// NewPostRepository создает новый репозиторий постов
func NewPostRepository(db *sql.DB) *PostRepository {
	return &PostRepository{db: db}
}
