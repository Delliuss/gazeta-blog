package handlers

import (
	"net/http"

	"gox2/models"
)

func (h *AppHandlers) ProfilePage(w http.ResponseWriter, r *http.Request) {
	username, err := r.Cookie("username")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" && r.FormValue("delete") != "" {
		postID := r.FormValue("delete")
		if err := h.posts.DeletePost(postID); err != nil {
			http.Error(w, "Ошибка при удалении поста: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	userPosts, err := h.posts.GetUserPosts(username.Value)
	if err != nil {
		http.Error(w, "Ошибка загрузки постов: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Username string
		Posts    []models.Post
	}{
		Username: username.Value,
		Posts:    userPosts,
	}

	if err := h.tmpl.ExecuteTemplate(w, "profile.html", data); err != nil {
		http.Error(w, "Ошибка отображения страницы: "+err.Error(), http.StatusInternalServerError)
	}
}
