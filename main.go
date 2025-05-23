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

func main() {
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

	tmpl := `<!DOCTYPE html><html><head>
		<title>Вкусная еда в путешествиях</title>
		<style>.post{border:1px solid #ddd;padding:15px;margin-bottom:20px}
		.recommended{background:#f8fff8;border-left:4px solid #4CAF50}</style>
		</head><body>
		<h1>Вкусная еда в путешествиях</h1>
		{{if .Username}}
			<p>Добро пожаловать, {{.Username}}! |
			<a href="/profile">Профиль</a> |
			{{if .IsAdmin}}<a href="/admin">Админ-панель</a> | {{end}}
			<a href="/logout">Выйти</a></p>
		{{else}}<p><a href="/login">Войти</a> | <a href="/register">Регистрация</a></p>{{end}}
		<form method="GET" action="/"><input type="text" name="search" placeholder="Поиск...">
		<input type="submit" value="Найти"></form>
		<h2>Последние отзывы</h2>
		{{range .Posts}}<div class="post {{if .IsRecommended}}recommended{{end}}">
			<h3>{{.Title}}</h3><p>{{.Location}} • {{.CreatedAt.Format "02.01.2006"}}</p>
			<p>Оценка: {{.Rating}}/5 {{if .IsRecommended}}⭐{{end}}</p>
			{{if .ImageURL}}<img src="{{.ImageURL}}" style="max-width:300px">{{end}}
			<p>{{.Content}}</p>
			<p>Автор: {{.Author}} • 👍 {{.Likes}} 👎 {{.Dislikes}}</p>
			{{if $.Username}}<form method="POST" action="/like" style="display:inline">
				<input type="hidden" name="post_id" value="{{.ID}}">
				<input type="hidden" name="action" value="like">
				<button type="submit">👍</button></form>
				<form method="POST" action="/like" style="display:inline">
				<input type="hidden" name="post_id" value="{{.ID}}">
				<input type="hidden" name="action" value="dislike">
				<button type="submit">👎</button></form>
			{{end}}
		</div>{{else}}<p>Нет отзывов</p>{{end}}
		</body></html>`

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

	t, _ := template.New("webpage").Parse(tmpl)
	t.Execute(w, struct {
		Posts    []models.Post
		Username string
		IsAdmin  bool
	}{
		filteredPosts,
		username,
		isAdmin,
	})
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

	tmpl := `<!DOCTYPE html>
	<html>
	<head>
		<title>Регистрация</title>
	</head>
	<body>
		<h1>Регистрация</h1>
		<form method="POST">
			<input type="text" name="username" placeholder="Имя пользователя" required>
			<input type="password" name="password" placeholder="Пароль" required>
			<input type="submit" value="Зарегистрироваться">
		</form>
		<a href="/">На главную</a>
	</body>
	</html>`

	t, _ := template.New("register").Parse(tmpl)
	t.Execute(w, nil)
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

	tmpl := `<!DOCTYPE html>
	<html>
	<head>
		<title>Вход</title>
	</head>
	<body>
		<h1>Вход</h1>
		<form method="POST">
			<input type="text" name="username" placeholder="Имя пользователя" required>
			<input type="password" name="password" placeholder="Пароль" required>
			<input type="submit" value="Войти">
		</form>
		<a href="/">На главную</a>
	</body>
	</html>`

	t, _ := template.New("login").Parse(tmpl)
	t.Execute(w, nil)
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

	funcMap := template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format("02.01.2006 15:04")
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
	}

	tmpl := `<!DOCTYPE html>
	<html>
	<head>
		<title>Мой профиль</title>
		<style>
			body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
			.header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
			.post { border: 1px solid #ddd; padding: 15px; margin-bottom: 15px; border-radius: 5px; }
			.post-header { display: flex; justify-content: space-between; margin-bottom: 10px; }
			.post-title { font-size: 1.2em; font-weight: bold; color: #333; margin: 0; }
			.post-meta { color: #666; font-size: 0.9em; }
			.post-content { margin: 10px 0; }
			.post-actions { margin-top: 10px; }
			.btn { padding: 5px 10px; text-decoration: none; border-radius: 3px; }
			.btn-edit { background: #4CAF50; color: white; }
			.btn-delete { background: #f44336; color: white; border: none; cursor: pointer; }
			.btn-new { background: #2196F3; color: white; }
			.rating { color: #FF9800; font-weight: bold; }
			.recommended { color: #4CAF50; }
		</style>
	</head>
	<body>
		<div class="header">
			<h1>Мой профиль: {{.Username}}</h1>
			<div>
				<a href="/" class="btn">На главную</a>
				<a href="/new-post" class="btn btn-new">Новый пост</a>
			</div>
		</div>

		<h2>Мои отзывы ({{len .Posts}})</h2>

		{{if .Posts}}
			{{range .Posts}}
				<div class="post {{if .IsRecommended}}recommended-border{{end}}">
					<div class="post-header">
						<h3 class="post-title">{{.Title}}</h3>
						<span class="rating">Оценка: {{.Rating}}/5 {{if .IsRecommended}}<span class="recommended">★ Рекомендую</span>{{end}}</span>
					</div>
					<div class="post-meta">
						<span>{{.Location}} • {{formatDate .CreatedAt}}</span>
					</div>
					<div class="post-content">
						{{truncate .Content 150}}
					</div>
					<div class="post-meta">
						👍 {{.Likes}} • 👎 {{.Dislikes}}
					</div>
					<div class="post-actions">
						<a href="/edit-post?id={{.ID}}" class="btn btn-edit">Редактировать</a>
						<form method="POST" style="display: inline;">
							<input type="hidden" name="delete" value="{{.ID}}">
							<button type="submit" class="btn btn-delete">Удалить</button>
						</form>
					</div>
				</div>
			{{end}}
		{{else}}
			<p>У вас пока нет ни одного отзыва. <a href="/new-post">Создайте первый!</a></p>
		{{end}}
	</body>
	</html>`

	data := struct {
		Username string
		Posts    []models.Post
	}{
		Username: username.Value,
		Posts:    userPosts,
	}

	t := template.Must(template.New("profile").Funcs(funcMap).Parse(tmpl))
	err = t.Execute(w, data)
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

	tmpl := `<!DOCTYPE html>
	<html>
	<head>
		<title>Новый отзыв</title>
	</head>
	<body>
		<h1>Новый отзыв</h1>
		<form method="POST">
			<div>
				<label>Название заведения:</label>
				<input type="text" name="title" required>
			</div>
			<div>
				<label>Местоположение (город, страна):</label>
				<input type="text" name="location" required>
			</div>
			<div>
				<label>Оценка (1-5):</label>
				<input type="number" name="rating" min="1" max="5" required>
			</div>
			<div>
				<label>URL изображения:</label>
				<input type="text" name="image_url">
			</div>
			<div>
				<label>Отзыв:</label>
				<textarea name="content" rows="5" required></textarea>
			</div>
			<div>
				<label>
					<input type="checkbox" name="is_recommended">
					Рекомендую это место
				</label>
			</div>
			<input type="submit" value="Опубликовать">
		</form>
		<a href="/profile">Отмена</a>
	</body>
	</html>`

	t, _ := template.New("newPost").Parse(tmpl)
	t.Execute(w, nil)
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

	tmpl := `<!DOCTYPE html>
	<html>
	<head>
		<title>Редактировать отзыв</title>
		<style>
			.form-group { margin-bottom: 15px; }
			label { display: block; margin-bottom: 5px; }
			input[type="text"], textarea, select {
				width: 100%;
				padding: 8px;
				box-sizing: border-box;
				margin-bottom: 10px;
			}
			textarea { height: 150px; }
			.submit-btn {
				background: #4CAF50;
				color: white;
				padding: 10px 15px;
				border: none;
				cursor: pointer;
			}
			.submit-btn:hover { background: #45a049; }
		</style>
	</head>
	<body>
		<h1>Редактировать отзыв</h1>
		{{if .IsAdmin}}<p style="color: red;">Вы редактируете этот пост как администратор</p>{{end}}
		<form method="POST">
			<input type="hidden" name="id" value="{{.ID}}">

			<div class="form-group">
				<label>Название заведения:</label>
				<input type="text" name="title" value="{{.Title}}" required>
			</div>

			<div class="form-group">
				<label>Местоположение (город, страна):</label>
				<input type="text" name="location" value="{{.Location}}" required>
			</div>

			<div class="form-group">
				<label>Оценка (1-5):</label>
				<select name="rating" required>
					<option value="1" {{if eq .Rating 1}}selected{{end}}>1</option>
					<option value="2" {{if eq .Rating 2}}selected{{end}}>2</option>
					<option value="3" {{if eq .Rating 3}}selected{{end}}>3</option>
					<option value="4" {{if eq .Rating 4}}selected{{end}}>4</option>
					<option value="5" {{if eq .Rating 5}}selected{{end}}>5</option>
				</select>
			</div>

			<div class="form-group">
				<label>URL изображения:</label>
				<input type="text" name="image_url" value="{{.ImageURL}}">
			</div>

			<div class="form-group">
				<label>Отзыв:</label>
				<textarea name="content" required>{{.Content}}</textarea>
			</div>

			<div class="form-group">
				<label>
					<input type="checkbox" name="is_recommended" {{if .IsRecommended}}checked{{end}}>
					Рекомендую это место
				</label>
			</div>

			<button type="submit" class="submit-btn">Сохранить изменения</button>
		</form>
		<a href="/profile">Вернуться в профиль</a>
	</body>
	</html>`

	data := struct {
		models.Post
		IsAdmin bool
	}{
		Post:    *post,
		IsAdmin: user.IsAdmin,
	}

	t := template.Must(template.New("editPost").Parse(tmpl))
	err = t.Execute(w, data)
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

	tmpl := `<!DOCTYPE html><html><head><title>Админ-панель</title>
		<style>table{width:100%} .banned{background:#ffdddd} .admin{background:#ddffdd}</style>
		</head><body>
		<h1>Админ-панель</h1><a href="/">На главную</a>
		<h2>Пользователи</h2>
		<table><tr><th>Имя</th><th>Статус</th><th>Действия</th></tr>
		{{range .Users}}<tr class="{{if .IsBanned}}banned{{else if .IsAdmin}}admin{{end}}">
		<td>{{.Username}}</td>
		<td>{{if .IsAdmin}}Админ{{else if .IsBanned}}Заблокирован{{else}}Обычный{{end}}</td>
		<td>{{if not .IsAdmin}}<form action="/admin/toggle-ban" method="POST" style="display:inline">
			<input type="hidden" name="username" value="{{.Username}}">
			<button type="submit">{{if .IsBanned}}Разблокировать{{else}}Заблокировать{{end}}</button>
		</form>{{end}}</td></tr>{{end}}</table>
		<h2>Все посты</h2>
		<table><tr><th>ID</th><th>Название</th><th>Автор</th><th>Действия</th></tr>
		{{range .Posts}}<tr><td>{{.ID}}</td><td>{{.Title}}</td><td>{{.Author}}</td>
		<td><a href="/edit-post?id={{.ID}}">Редактировать</a> |
		<form action="/admin/delete-post" method="POST" style="display:inline">
			<input type="hidden" name="post_id" value="{{.ID}}">
			<button type="submit">Удалить</button>
		</form></td></tr>{{end}}</table>
		</body></html>`

	t := template.Must(template.New("admin").Parse(tmpl))
	t.Execute(w, struct {
		Users []models.User
		Posts []models.Post
	}{users, allPosts})
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
