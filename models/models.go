package models

import (
	"time"
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
