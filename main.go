package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"gox2/db"
	"gox2/models"
	"gox2/posts"
	"gox2/users"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// App представляет основное приложение
type App struct {
	users *users.UserRepository
	posts *posts.PostRepository
}

var templates *template.Template

func initTemplates() error {
	var err error
	templates, err = template.New("").Funcs(template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format("02.01.2006 15:04")
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
	}).ParseGlob("templates/*.html")
	return err
}

func main() {
	// Инициализация шаблонов
	if err := initTemplates(); err != nil {
		panic(fmt.Sprintf("Не удалось загрузить шаблоны: %v", err))
	}

	// Инициализация базы данных
	database, err := db.New("user=postgres dbname=go password=0000 host=localhost sslmode=disable")
	if err != nil {
		panic(fmt.Sprintf("Не удалось подключиться к БД: %v", err))
	}
	defer database.Close()

	app := &App{
		users: users.NewUserRepository(database.DB),
		posts: posts.NewPostRepository(database.DB),
	}

	// Настройка маршрутов
	http.HandleFunc("/", app.homePage)
	http.HandleFunc("/register", app.registerPage)
	http.HandleFunc("/login", app.loginPage)
	http.HandleFunc("/logout", app.logoutPage)
	http.HandleFunc("/profile", app.profilePage)
	http.HandleFunc("/new-post", app.newPostPage)
	http.HandleFunc("/edit-post", app.editPostPage)
	http.HandleFunc("/like", app.likeHandler)

	// Админ-роуты
	http.HandleFunc("/admin", app.adminOnly(app.adminPanel))
	http.HandleFunc("/admin/toggle-ban", app.adminOnly(app.toggleBanUser))
	http.HandleFunc("/admin/delete-post", app.adminOnly(app.adminDeletePost))

	fmt.Println("Сервер запущен на http://localhost:8081")
	http.ListenAndServe(":8081", nil)
}

// homePage отображает главную страницу
func (app *App) homePage(w http.ResponseWriter, r *http.Request) {
	// Проверка блокировки пользователя
	if cookie, err := r.Cookie("username"); err == nil {
		user, err := app.users.GetUser(cookie.Value)
		if err == nil && user.IsBanned {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "<h1>Ваш аккаунт заблокирован</h1><p>Обратитесь к администратору.</p>")
			return
		}
	}

	search := r.URL.Query().Get("search")
	posts, err := app.posts.LoadPosts()
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
		user, err := app.users.GetUser(cookie.Value)
		if err == nil {
			username = user.Username
			isAdmin = user.IsAdmin
		}
	}

	err = templates.ExecuteTemplate(w, "home.html", struct {
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

// registerPage обрабатывает страницу регистрации
func (app *App) registerPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		err := app.users.CreateUser(username, password)
		if err != nil {
			http.Error(w, "Ошибка регистрации: имя пользователя уже занято", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err := templates.ExecuteTemplate(w, "register.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// loginPage обрабатывает страницу входа
func (app *App) loginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		user, err := app.users.GetUser(username)
		if err != nil {
			http.Error(w, "Неверные данные", http.StatusUnauthorized)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			http.Error(w, "Неверные данные", http.StatusUnauthorized)
			return
		}

		if user.IsBanned {
			http.Error(w, "Аккаунт заблокирован", http.StatusForbidden)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:    "username",
			Value:   username,
			Path:    "/",
			Expires: time.Now().Add(24 * time.Hour),
		})

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	err := templates.ExecuteTemplate(w, "login.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// logoutPage обрабатывает выход пользователя
func (app *App) logoutPage(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "username",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// profilePage отображает профиль пользователя
func (app *App) profilePage(w http.ResponseWriter, r *http.Request) {
	username, err := r.Cookie("username")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" && r.FormValue("delete") != "" {
		postID := r.FormValue("delete")
		if err := app.posts.DeletePost(postID); err != nil {
			http.Error(w, "Ошибка при удалении поста: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	userPosts, err := app.posts.GetUserPosts(username.Value)
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

	err = templates.ExecuteTemplate(w, "profile.html", data)
	if err != nil {
		http.Error(w, "Ошибка отображения страницы: "+err.Error(), http.StatusInternalServerError)
	}
}

// newPostPage отображает форму создания нового поста
func (app *App) newPostPage(w http.ResponseWriter, r *http.Request) {
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
		if err := app.posts.CreatePost(title, content, imageURL, location, username.Value, rating, isRecommended); err != nil {
			http.Error(w, "Ошибка создания поста", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	err = templates.ExecuteTemplate(w, "new_post.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// editPostPage отображает форму редактирования поста
func (app *App) editPostPage(w http.ResponseWriter, r *http.Request) {
	username, err := r.Cookie("username")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := app.users.GetUser(username.Value)
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
		if err := app.posts.UpdatePost(postID, title, content, imageURL, location, rating, isRecommended); err != nil {
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

	post, err := app.posts.GetPost(postID)
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

	err = templates.ExecuteTemplate(w, "edit_post.html", data)
	if err != nil {
		http.Error(w, "Ошибка отображения страницы: "+err.Error(), http.StatusInternalServerError)
	}
}

// likeHandler обрабатывает лайки/дизлайки
func (app *App) likeHandler(w http.ResponseWriter, r *http.Request) {
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

	if err := app.posts.AddVote(postID, username.Value, action); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

// adminPanel отображает админ-панель
func (app *App) adminPanel(w http.ResponseWriter, r *http.Request) {
	users, err := app.users.GetAllUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	allPosts, err := app.posts.GetAllPosts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = templates.ExecuteTemplate(w, "admin.html", struct {
		Users []models.User
		Posts []models.Post
	}{users, allPosts})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// toggleBanUser переключает статус блокировки пользователя
func (app *App) toggleBanUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	if err := app.users.ToggleUserBan(r.FormValue("username")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// adminDeletePost удаляет пост (админ)
func (app *App) adminDeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	postID := r.FormValue("post_id")
	if err := app.posts.DeletePost(postID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// adminOnly middleware проверяет права администратора
func (app *App) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, err := r.Cookie("username")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		user, err := app.users.GetUser(username.Value)
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
