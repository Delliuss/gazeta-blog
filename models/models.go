package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username string
	Password string
	IsAdmin  bool
	IsBanned bool
}

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

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
