package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"gox2/admin"
	"gox2/authentication"
	"gox2/models"
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

// homePage отображает главную страницу
func (h *AppHandlers) HomePage(w http.ResponseWriter, r *http.Request) {
	// Проверка блокировки пользователя
	if cookie, err := r.Cookie("username"); err == nil {
		user, err := h.users.GetUser(cookie.Value)
		if err == nil && user.IsBanned {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "<h1>Ваш аккаунт заблокирован</h1><p>Обратитесь к администратору.</p>")
			return
		}
	}

	search := r.URL.Query().Get("search")
	posts, err := h.posts.LoadPosts()
	if err != nil {
		http.Error(w, "Ошибка загрузки постов", http.StatusInternalServerError)
		return
	}

	var filteredPosts []models.Post
	for _, post := range posts {
		if search == "" || contains(post.Title, search) || contains(post.Location, search) {
			filteredPosts = append(filteredPosts, post)
		}
	}

	username := ""
	var isAdmin bool
	if cookie, err := r.Cookie("username"); err == nil {
		user, err := h.users.GetUser(cookie.Value)
		if err == nil {
			username = user.Username
			isAdmin = user.IsAdmin
		}
	}

	err = h.tmpl.ExecuteTemplate(w, "home.html", struct {
		Posts    []models.Post
		Username string
		IsAdmin  bool
	}{
		filteredPosts,
		username,
		isAdmin,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// profilePage отображает профиль пользователя
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

	err = h.tmpl.ExecuteTemplate(w, "profile.html", data)
	if err != nil {
		http.Error(w, "Ошибка отображения страницы: "+err.Error(), http.StatusInternalServerError)
	}
}

// newPostPage отображает форму создания нового поста
func (h *AppHandlers) NewPostPage(w http.ResponseWriter, r *http.Request) {
	username, err := r.Cookie("username")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		title := r.FormValue("title")
		content := r.FormValue("content")
		imageURL := r.FormValue("image_url")
		location := r.FormValue("location")
		isRecommended := r.FormValue("is_recommended") == "on"
		ratingStr := r.FormValue("rating")
		rating, err := strconv.Atoi(ratingStr)
		if err != nil {
			http.Error(w, "Оценка должна быть числом от 1 до 5", http.StatusBadRequest)
			return
		}

		if rating < 1 || rating > 5 {
			http.Error(w, "Оценка должна быть от 1 до 5", http.StatusBadRequest)
			return
		}
		if err := h.posts.CreatePost(title, content, imageURL, location, username.Value, rating, isRecommended); err != nil {
			http.Error(w, "Ошибка создания поста", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	err = h.tmpl.ExecuteTemplate(w, "new_post.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// editPostPage отображает форму редактирования поста
func (h *AppHandlers) EditPostPage(w http.ResponseWriter, r *http.Request) {
	username, err := r.Cookie("username")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.users.GetUser(username.Value)
	if err != nil {
		http.Error(w, "Ошибка проверки прав", http.StatusInternalServerError)
		return
	}

	if r.Method == "POST" {
		postID := r.FormValue("id")
		title := r.FormValue("title")
		content := r.FormValue("content")
		imageURL := r.FormValue("image_url")
		location := r.FormValue("location")
		isRecommended := r.FormValue("is_recommended") == "on"
		ratingStr := r.FormValue("rating")
		rating, err := strconv.Atoi(ratingStr)
		if err != nil {
			http.Error(w, "Оценка должна быть числом от 1 до 5", http.StatusBadRequest)
			return
		}

		if rating < 1 || rating > 5 {
			http.Error(w, "Оценка должна быть от 1 до 5", http.StatusBadRequest)
			return
		}
		if err := h.posts.UpdatePost(postID, title, content, imageURL, location, rating, isRecommended); err != nil {
			http.Error(w, "Ошибка обновления поста: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	postID := r.URL.Query().Get("id")
	if postID == "" {
		http.Error(w, "Не указан ID поста", http.StatusBadRequest)
		return
	}

	post, err := h.posts.GetPost(postID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Пост не найден или у вас нет прав на его редактирование", http.StatusNotFound)
		} else {
			http.Error(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	data := struct {
		models.Post
		IsAdmin bool
	}{
		Post:    *post,
		IsAdmin: user.IsAdmin,
	}

	err = h.tmpl.ExecuteTemplate(w, "edit_post.html", data)
	if err != nil {
		http.Error(w, "Ошибка отображения страницы: "+err.Error(), http.StatusInternalServerError)
	}
}

// likeHandler обрабатывает лайки/дизлайки
func (h *AppHandlers) LikeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	username, err := r.Cookie("username")
	if err != nil {
		http.Error(w, "Необходимо войти в систему", http.StatusUnauthorized)
		return
	}

	postID := r.FormValue("post_id")
	action := r.FormValue("action")

	if err := h.posts.AddVote(postID, username.Value, action); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

// adminOnly middleware проверяет права администратора
func (h *AppHandlers) AdminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, err := r.Cookie("username")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		user, err := h.users.GetUser(username.Value)
		if err != nil || !user.IsAdmin {
			http.Error(w, "Доступ запрещён: требуется права администратора", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

// contains проверяет наличие подстроки
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
