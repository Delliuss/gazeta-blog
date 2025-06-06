package handlers

import (
	"gox2/models"
	"gox2/posts"
	"gox2/users"
	"html/template"
	"net/http"
)

type AdminHandler struct {
	users *users.UserRepository
	posts *posts.PostRepository
	tmpl  *template.Template
}

func NewAdminHandler(users *users.UserRepository, posts *posts.PostRepository, tmpl *template.Template) *AdminHandler {
	return &AdminHandler{
		users: users,
		posts: posts,
		tmpl:  tmpl,
	}
}

func (h *AdminHandler) AdminPanel(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.GetAllUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	allPosts, err := h.posts.GetAllPosts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = h.tmpl.ExecuteTemplate(w, "admin.html", struct {
		Users []models.User
		Posts []models.Post
	}{users, allPosts})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *AdminHandler) ToggleBanUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	if err := h.users.ToggleUserBan(r.FormValue("username")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *AdminHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	postID := r.FormValue("post_id")
	if err := h.posts.DeletePost(postID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
