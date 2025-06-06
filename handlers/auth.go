package handlers

import (
	"html/template"

	"gox2/authentication"
	"gox2/posts"
	"gox2/users"
)

type AppHandlers struct {
	auth  *authentication.AuthHandler
	users *users.UserRepository
	posts *posts.PostRepository
	admin *AdminHandler
	tmpl  *template.Template
}

func NewAppHandlers(
	auth *authentication.AuthHandler,
	users *users.UserRepository,
	posts *posts.PostRepository,
	admin *AdminHandler,
	tmpl *template.Template,
) *AppHandlers {
	return &AppHandlers{
		auth:  auth,
		users: users,
		posts: posts,
		admin: admin,
		tmpl:  tmpl,
	}
}
