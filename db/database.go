package db

import (
	"database/sql"
	"fmt"
	"gox2/models"
	"sync"

	_ "github.com/lib/pq"
)

// DB представляет обертку для работы с базой данных (только пользователи)
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
		hashedPass, err := models.HashPassword("admin123")
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

// GetUser возвращает пользователя по имени
func (db *DB) GetUser(username string) (*models.User, error) {
	var user models.User
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
	hashedPass, err := models.HashPassword(password)
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

// GetAllUsers возвращает всех пользователей
func (db *DB) GetAllUsers() ([]models.User, error) {
	rows, err := db.Query("SELECT username, is_admin, is_banned FROM users ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.Username, &u.IsAdmin, &u.IsBanned); err == nil {
			users = append(users, u)
		}
	}

	return users, nil
}
