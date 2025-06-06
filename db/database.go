package db

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
	mu sync.Mutex
}

func New(connStr string) (*DB, error) {
	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB connection: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping DB: %v", err)
	}

	db := &DB{DB: sqlDB}
	if err := db.initialize(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) initialize() error {
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
		return fmt.Errorf("failed to create tables: %v", err)
	}

	db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN DEFAULT FALSE")
	db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS is_banned BOOLEAN DEFAULT FALSE")

	return nil
}
