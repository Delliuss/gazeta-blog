package authentication

import (
	"gox2/users"
	"html/template"
)

type AuthHandler struct {
	users *users.UserRepository
	tmpl  *template.Template
}

func NewAuthHandler(users *users.UserRepository, tmpl *template.Template) *AuthHandler {
	return &AuthHandler{
		users: users,
		tmpl:  tmpl,
	}
}
