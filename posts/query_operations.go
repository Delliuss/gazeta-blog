package posts

import (
	"gox2/models"
)

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
