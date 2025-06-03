package handlers

import (
	"html/template"

	"gox2/admin"
	"gox2/authentication"
	"gox2/posts"
	"gox2/users"
)

// AppHandlers содержит все обработчики приложения
type AppHandlers struct {
	auth  *authentication.AuthHandler
	users *users.UserRepository
	posts *posts.PostRepository
	admin *admin.AdminHandler
	tmpl  *template.Template
}

// NewAppHandlers создает новый экземпляр AppHandlers
func NewAppHandlers(
	auth *authentication.AuthHandler,
	users *users.UserRepository,
	posts *posts.PostRepository,
	admin *admin.AdminHandler,
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
