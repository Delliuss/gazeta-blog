package posts

import (
	"gox2/models"
)

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
