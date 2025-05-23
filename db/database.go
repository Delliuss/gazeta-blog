package db

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// DB представляет обертку для работы с базой данных
type DB struct {
	*sql.DB
	mu sync.Mutex
}

// New создает новое подключение к базе данных
func New(connStr string) (*DB, error) {
	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть соединение с БД: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("не удалось проверить соединение с БД: %v", err)
	}

	db := &DB{DB: sqlDB}
	if err := db.initialize(); err != nil {
		return nil, err
	}

	return db, nil
}

// initialize создает таблицы и проверяет наличие админа
func (db *DB) initialize() error {
	// Создаем таблицы
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            username TEXT NOT NULL PRIMARY KEY,
            password TEXT NOT NULL,
            is_admin BOOLEAN DEFAULT FALSE,
            is_banned BOOLEAN DEFAULT FALSE
        );
       
        CREATE TABLE IF NOT EXISTS posts (
            id SERIAL PRIMARY KEY,
            title TEXT NOT NULL,
            content TEXT NOT NULL,
            image_url TEXT,
            location TEXT NOT NULL,
            rating INT NOT NULL,
            likes INT DEFAULT 0,
            dislikes INT DEFAULT 0,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            author TEXT NOT NULL,
            is_recommended BOOLEAN DEFAULT FALSE,
            FOREIGN KEY (author) REFERENCES users(username) ON DELETE CASCADE
        );
       
        CREATE TABLE IF NOT EXISTS post_votes (
            post_id INT NOT NULL,
            username TEXT NOT NULL,
            vote_type TEXT NOT NULL,
            PRIMARY KEY (post_id, username),
            FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
            FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
        );
    `)
	if err != nil {
		return fmt.Errorf("не удалось создать таблицы: %v", err)
	}

	// Добавляем колонки, если их вдруг нет
	db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN DEFAULT FALSE")
	db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS is_banned BOOLEAN DEFAULT FALSE")

	// Проверяем наличие админа
	return db.ensureAdminExists()
}

// ensureAdminExists проверяет наличие администратора и создает его при необходимости
func (db *DB) ensureAdminExists() error {
	var adminExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = 'admin')").Scan(&adminExists)
	if err != nil {
		return fmt.Errorf("ошибка проверки наличия админа: %v", err)
	}

	if !adminExists {
		hashedPass, err := hashPassword("admin123")
		if err != nil {
			return fmt.Errorf("ошибка хеширования пароля админа: %v", err)
		}

		_, err = db.Exec(
			"INSERT INTO users (username, password, is_admin, is_banned) VALUES ($1, $2, $3, $4)",
			"admin", hashedPass, true, false,
		)
		if err != nil {
			return fmt.Errorf("не удалось создать администратора: %v", err)
		}
	}

	return nil
}

// hashPassword хеширует пароль
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// User представляет пользователя системы
type User struct {
	Username string
	Password string
	IsAdmin  bool
	IsBanned bool
}

// Post представляет пост/отзыв
type Post struct {
	ID            int
	Title         string
	Content       string
	ImageURL      string
	Location      string
	Rating        int
	Likes         int
	Dislikes      int
	CreatedAt     time.Time
	Author        string
	IsRecommended bool
}

// LoadPosts загружает все посты из базы данных
func (db *DB) LoadPosts() ([]Post, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	rows, err := db.Query(`
        SELECT id, title, content, image_url, location, rating, likes, dislikes,
               created_at, author, is_recommended
        FROM posts
        ORDER BY created_at DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
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

// GetUser возвращает пользователя по имени
func (db *DB) GetUser(username string) (*User, error) {
	var user User
	err := db.QueryRow(`
        SELECT username, password, is_admin, is_banned
        FROM users
        WHERE username = $1`,
		username,
	).Scan(&user.Username, &user.Password, &user.IsAdmin, &user.IsBanned)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// CreateUser создает нового пользователя
func (db *DB) CreateUser(username, password string) error {
	hashedPass, err := hashPassword(password)
	if err != nil {
		return err
	}

	_, err = db.Exec(
		"INSERT INTO users (username, password) VALUES ($1, $2)",
		username, hashedPass,
	)
	return err
}

// ToggleUserBan переключает статус блокировки пользователя
func (db *DB) ToggleUserBan(username string) error {
	_, err := db.Exec(
		"UPDATE users SET is_banned = NOT is_banned WHERE username = $1 AND is_admin = FALSE",
		username,
	)
	return err
}

// DeletePost удаляет пост по ID
func (db *DB) DeletePost(postID string) error {
	tx, err := db.Begin()
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

// GetUserPosts возвращает посты пользователя
func (db *DB) GetUserPosts(username string) ([]Post, error) {
	rows, err := db.Query(`
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

	var posts []Post
	for rows.Next() {
		var p Post
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

// GetAllUsers возвращает всех пользователей
func (db *DB) GetAllUsers() ([]User, error) {
	rows, err := db.Query("SELECT username, is_admin, is_banned FROM users ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Username, &u.IsAdmin, &u.IsBanned); err == nil {
			users = append(users, u)
		}
	}

	return users, nil
}

// GetAllPosts возвращает все посты
func (db *DB) GetAllPosts() ([]Post, error) {
	rows, err := db.Query("SELECT id, title, author FROM posts ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Author); err == nil {
			posts = append(posts, p)
		}
	}

	return posts, nil
}

// CreatePost создает новый пост
func (db *DB) CreatePost(title, content, imageURL, location, author string, rating int, isRecommended bool) error {
	_, err := db.Exec(`
        INSERT INTO posts
        (title, content, image_url, location, rating, author, is_recommended)
        VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		title, content, imageURL, location, rating, author, isRecommended,
	)
	return err
}

// GetPost возвращает пост по ID
func (db *DB) GetPost(postID string) (*Post, error) {
	var post Post
	err := db.QueryRow(`
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
func (db *DB) UpdatePost(postID, title, content, imageURL, location string, rating int, isRecommended bool) error {
	_, err := db.Exec(`
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
func (db *DB) AddVote(postID, username, action string) error {
	tx, err := db.Begin()
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
