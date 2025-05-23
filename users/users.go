package users

import (
	"database/sql"
	"fmt"
	"gox2/models"
	"sync"
)

// UserRepository represents a repository for user operations
type UserRepository struct {
	db *sql.DB
	mu sync.Mutex
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetUser returns a user by username
func (r *UserRepository) GetUser(username string) (*models.User, error) {
	var user models.User
	err := r.db.QueryRow(`
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

// CreateUser creates a new user
func (r *UserRepository) CreateUser(username, password string) error {
	hashedPass, err := models.HashPassword(password)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(
		"INSERT INTO users (username, password) VALUES ($1, $2)",
		username, hashedPass,
	)
	return err
}

// ToggleUserBan toggles user ban status
func (r *UserRepository) ToggleUserBan(username string) error {
	_, err := r.db.Exec(
		"UPDATE users SET is_banned = NOT is_banned WHERE username = $1 AND is_admin = FALSE",
		username,
	)
	return err
}

// GetAllUsers returns all users
func (r *UserRepository) GetAllUsers() ([]models.User, error) {
	rows, err := r.db.Query("SELECT username, is_admin, is_banned FROM users ORDER BY username")
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

// ensureAdminExists checks if admin exists and creates one if not
func (r *UserRepository) ensureAdminExists() error {
	var adminExists bool
	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = 'admin')").Scan(&adminExists)
	if err != nil {
		return fmt.Errorf("error checking admin existence: %v", err)
	}

	if !adminExists {
		hashedPass, err := models.HashPassword("admin123")
		if err != nil {
			return fmt.Errorf("error hashing admin password: %v", err)
		}

		_, err = r.db.Exec(
			"INSERT INTO users (username, password, is_admin, is_banned) VALUES ($1, $2, $3, $4)",
			"admin", hashedPass, true, false,
		)
		if err != nil {
			return fmt.Errorf("failed to create admin: %v", err)
		}
	}

	return nil
}
