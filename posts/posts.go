package posts

import (
	"database/sql"
	"fmt"
	"gox2/models"
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

// LoadPosts загружает все посты из базы данных
func (r *PostRepository) LoadPosts() ([]models.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rows, err := r.db.Query(`
		SELECT id, title, content, image_url, location, rating, likes, dislikes,
			   created_at, author, is_recommended
		FROM posts
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		err := rows.Scan(
			&p.ID, &p.Title, &p.Content, &p.ImageURL, &p.Location,
			&p.Rating, &p.Likes, &p.Dislikes, &p.CreatedAt, &p.Author,
			&p.IsRecommended,
		)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}

	return posts, nil
}

// GetUserPosts возвращает посты пользователя
func (r *PostRepository) GetUserPosts(username string) ([]models.Post, error) {
	rows, err := r.db.Query(`
		SELECT id, title, content, image_url, location, rating, likes, dislikes,
			   created_at, is_recommended
		FROM posts
		WHERE author = $1
		ORDER BY created_at DESC`,
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		err := rows.Scan(
			&p.ID, &p.Title, &p.Content, &p.ImageURL, &p.Location,
			&p.Rating, &p.Likes, &p.Dislikes, &p.CreatedAt, &p.IsRecommended,
		)
		if err != nil {
			return nil, err
		}
		p.Author = username
		posts = append(posts, p)
	}

	return posts, nil
}

// GetAllPosts возвращает все посты
func (r *PostRepository) GetAllPosts() ([]models.Post, error) {
	rows, err := r.db.Query("SELECT id, title, author FROM posts ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Author); err == nil {
			posts = append(posts, p)
		}
	}

	return posts, nil
}

// CreatePost создает новый пост
func (r *PostRepository) CreatePost(title, content, imageURL, location, author string, rating int, isRecommended bool) error {
	_, err := r.db.Exec(`
		INSERT INTO posts
		(title, content, image_url, location, rating, author, is_recommended)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		title, content, imageURL, location, rating, author, isRecommended,
	)
	return err
}

// GetPost возвращает пост по ID
func (r *PostRepository) GetPost(postID string) (*models.Post, error) {
	var post models.Post
	err := r.db.QueryRow(`
		SELECT id, title, content, image_url, location, rating, is_recommended
		FROM posts WHERE id = $1`,
		postID,
	).Scan(&post.ID, &post.Title, &post.Content, &post.ImageURL,
		&post.Location, &post.Rating, &post.IsRecommended)

	if err != nil {
		return nil, err
	}

	return &post, nil
}

// UpdatePost обновляет пост
func (r *PostRepository) UpdatePost(postID, title, content, imageURL, location string, rating int, isRecommended bool) error {
	_, err := r.db.Exec(`
		UPDATE posts SET
			title = $1,
			content = $2,
			image_url = $3,
			location = $4,
			rating = $5,
			is_recommended = $6
		WHERE id = $7`,
		title, content, imageURL, location, rating, isRecommended, postID,
	)
	return err
}

// AddVote добавляет голос за пост
func (r *PostRepository) AddVote(postID, username, action string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	// Проверяем, не голосовал ли уже пользователь
	var exists bool
	err = tx.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM post_votes
			WHERE post_id = $1 AND username = $2
		)`, postID, username).Scan(&exists)

	if err != nil {
		tx.Rollback()
		return err
	}

	if exists {
		tx.Rollback()
		return fmt.Errorf("пользователь уже голосовал за этот пост")
	}

	// Обновляем счетчик
	var column string
	if action == "like" {
		column = "likes"
	} else if action == "dislike" {
		column = "dislikes"
	} else {
		tx.Rollback()
		return fmt.Errorf("неверное действие")
	}

	_, err = tx.Exec(`
		UPDATE posts
		SET `+column+` = `+column+` + 1
		WHERE id = $1`, postID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Записываем голос
	_, err = tx.Exec(`
		INSERT INTO post_votes (post_id, username, vote_type)
		VALUES ($1, $2, $3)`,
		postID, username, action,
	)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// DeletePost удаляет пост по ID
func (r *PostRepository) DeletePost(postID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM post_votes WHERE post_id = $1", postID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("DELETE FROM posts WHERE id = $1", postID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
