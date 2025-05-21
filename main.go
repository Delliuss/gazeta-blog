package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// Структуры данных
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

type User struct {
	Username string
	Password string
	IsAdmin  bool
	IsBanned bool
}

var posts []Post
var mu sync.Mutex
var db *sql.DB

// Хелперы
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// Инициализация БД
func initDB() {
	var err error
	connStr := "user=postgres dbname=go password=0000 host=localhost sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	// Сначала создаем таблицу users с нужными колонками
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            username TEXT NOT NULL PRIMARY KEY,
            password TEXT NOT NULL,
            is_admin BOOLEAN DEFAULT FALSE,
            is_banned BOOLEAN DEFAULT FALSE
        );
    `)
	if err != nil {
		panic(err)
	}

	// Затем остальные таблицы
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS posts (
            id SERIAL PRIMARY KEY,
            title TEXT NOT NULL,
            content TEXT NOT NULL,
            image_url TEXT,
            location TEXT NOT NULL,
            rating INT NOT NULL,
            likes INT DEFAULT 0,
            dislikes INT DEFAULT 0,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            author TEXT NOT NULL,
            is_recommended BOOLEAN DEFAULT FALSE,
            FOREIGN KEY (author) REFERENCES users(username) ON DELETE CASCADE
        );
        
        CREATE TABLE IF NOT EXISTS post_votes (
            post_id INT NOT NULL,
            username TEXT NOT NULL,
            vote_type TEXT NOT NULL,
            PRIMARY KEY (post_id, username),
            FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
            FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
        );
    `)
	if err != nil {
		panic(err)
	}

	// Добавляем колонки, если их вдруг нет (дополнительная проверка)
	db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN DEFAULT FALSE")
	db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS is_banned BOOLEAN DEFAULT FALSE")

	ensureAdminExists()
	seedTestData()
}

func ensureAdminExists() {
	var adminExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = 'admin')").Scan(&adminExists)
	if err != nil {
		panic(err)
	}

	if !adminExists {
		hashedPass, _ := hashPassword("admin123")
		_, err = db.Exec("INSERT INTO users (username, password, is_admin, is_banned) VALUES ($1, $2, $3, $4)",
			"admin", hashedPass, true, false)
		if err != nil {
			panic(err)
		}
	}
}

func seedTestData() {
	var adminExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = 'admin')").Scan(&adminExists)
	if err != nil {
		panic(err)
	}

	if !adminExists {
		hashedPass, _ := hashPassword("admin123")
		_, err = db.Exec("INSERT INTO users (username, password, is_admin, is_banned) VALUES ($1, $2, $3, $4)",
			"admin", hashedPass, true, false)
		if err != nil {
			panic(err)
		}
	}

	loadPostsFromDB()
}

func loadPostsFromDB() {
	rows, err := db.Query(`
		SELECT id, title, content, image_url, location, rating, likes, dislikes, 
			   created_at, author, is_recommended 
		FROM posts
		ORDER BY created_at DESC
	`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	mu.Lock()
	defer mu.Unlock()

	posts = []Post{}
	for rows.Next() {
		var p Post
		err := rows.Scan(
			&p.ID, &p.Title, &p.Content, &p.ImageURL, &p.Location,
			&p.Rating, &p.Likes, &p.Dislikes, &p.CreatedAt, &p.Author,
			&p.IsRecommended,
		)
		if err != nil {
			panic(err)
		}
		posts = append(posts, p)
	}
}

// Middleware
func adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, err := r.Cookie("username")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var isAdmin bool
		err = db.QueryRow("SELECT is_admin FROM users WHERE username = $1", username.Value).Scan(&isAdmin)
		if err != nil || !isAdmin {
			http.Error(w, "Доступ запрещён: требуется права администратора", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

// Обработчики
func homePage(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("username"); err == nil {
		var isBanned bool
		db.QueryRow("SELECT is_banned FROM users WHERE username = $1", cookie.Value).Scan(&isBanned)
		if isBanned {
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
	var filteredPosts []Post
	for _, post := range posts {
		if search == "" || contains(post.Title, search) || contains(post.Location, search) {
			filteredPosts = append(filteredPosts, post)
		}
	}

	username := ""
	var isAdmin bool
	if cookie, err := r.Cookie("username"); err == nil {
		username = cookie.Value
		db.QueryRow("SELECT is_admin FROM users WHERE username = $1", username).Scan(&isAdmin)
	}

	t, _ := template.New("webpage").Parse(tmpl)
	t.Execute(w, struct {
		Posts    []Post
		Username string
		IsAdmin  bool
	}{
		filteredPosts,
		username,
		isAdmin,
	})
}

func adminPanel(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT username, is_admin, is_banned FROM users ORDER BY username")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Username, &u.IsAdmin, &u.IsBanned); err == nil {
			users = append(users, u)
		}
	}

	postsRows, err := db.Query("SELECT id, title, author FROM posts ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer postsRows.Close()

	var allPosts []Post
	for postsRows.Next() {
		var p Post
		if err := postsRows.Scan(&p.ID, &p.Title, &p.Author); err == nil {
			allPosts = append(allPosts, p)
		}
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
		Users []User
		Posts []Post
	}{users, allPosts})
}

func toggleBanUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	_, err := db.Exec("UPDATE users SET is_banned = NOT is_banned WHERE username = $1 AND is_admin = FALSE",
		r.FormValue("username"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func adminDeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	postID := r.FormValue("post_id")
	db.Exec("DELETE FROM post_votes WHERE post_id = $1", postID)
	db.Exec("DELETE FROM posts WHERE id = $1", postID)
	loadPostsFromDB()
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func registerPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		hashedPass, err := hashPassword(password)
		if err != nil {
			http.Error(w, "Ошибка хеширования пароля", http.StatusInternalServerError)
			return
		}

		_, err = db.Exec("INSERT INTO users (username, password, is_admin, is_banned) VALUES ($1, $2, $3, $4)",
			username, hashedPass, false, false)

		if err != nil {
			http.Error(w, "Ошибка регистрации: имя пользователя уже занято", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tmpl := `
    <!DOCTYPE html>
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

func loginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		var storedHash string
		var isBanned bool
		err := db.QueryRow("SELECT password, is_banned FROM users WHERE username = $1", username).
			Scan(&storedHash, &isBanned)

		if err != nil {
			// Пользователь не найден
			http.Error(w, "Неверные данные", http.StatusUnauthorized)
			return
		}

		// Сравниваем хеш пароля
		if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)); err != nil {
			http.Error(w, "Неверные данные", http.StatusUnauthorized)
			return
		}

		if isBanned {
			http.Error(w, "Аккаунт заблокирован", http.StatusForbidden)
			return
		}

		// Устанавливаем куки при успешной аутентификации
		http.SetCookie(w, &http.Cookie{
			Name:    "username",
			Value:   username,
			Path:    "/",
			Expires: time.Now().Add(24 * time.Hour),
		})

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl := `
    <!DOCTYPE html>
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

func logoutPage(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "username",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func profilePage(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию пользователя
	username, err := r.Cookie("username")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Обработка удаления поста
	if r.Method == "POST" && r.FormValue("delete") != "" {
		postID := r.FormValue("delete")

		// Удаляем все голоса связанные с постом
		_, err := db.Exec("DELETE FROM post_votes WHERE post_id = $1", postID)
		if err != nil {
			http.Error(w, "Ошибка при удалении голосов: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Удаляем сам пост
		_, err = db.Exec("DELETE FROM posts WHERE id = $1 AND author = $2", postID, username.Value)
		if err != nil {
			http.Error(w, "Ошибка при удалении поста: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Обновляем кэш постов
		loadPostsFromDB()
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Получаем посты пользователя
	rows, err := db.Query(`
        SELECT id, title, content, image_url, location, rating, likes, dislikes,
               created_at, is_recommended
        FROM posts
        WHERE author = $1
        ORDER BY created_at DESC`,
		username.Value)
	if err != nil {
		http.Error(w, "Ошибка загрузки постов: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var userPosts []Post
	for rows.Next() {
		var p Post
		err := rows.Scan(
			&p.ID, &p.Title, &p.Content, &p.ImageURL, &p.Location,
			&p.Rating, &p.Likes, &p.Dislikes, &p.CreatedAt, &p.IsRecommended,
		)
		if err != nil {
			http.Error(w, "Ошибка сканирования поста: "+err.Error(), http.StatusInternalServerError)
			return
		}
		p.Author = username.Value
		userPosts = append(userPosts, p)
	}

	// Функция для шаблона - форматирование даты
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

	tmpl := `
    <!DOCTYPE html>
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
		Posts    []Post
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

func newPostPage(w http.ResponseWriter, r *http.Request) {
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
		rating := r.FormValue("rating")
		isRecommended := r.FormValue("is_recommended") == "on"

		_, err := db.Exec(`
            INSERT INTO posts
            (title, content, image_url, location, rating, author, is_recommended)
            VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			title, content, imageURL, location, rating, username.Value, isRecommended)

		if err != nil {
			http.Error(w, "Ошибка создания поста", http.StatusInternalServerError)
			return
		}

		loadPostsFromDB()
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	tmpl := `
    <!DOCTYPE html>
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

func editPostPage(w http.ResponseWriter, r *http.Request) {
	// Проверка авторизации
	username, err := r.Cookie("username")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Проверка админских прав
	var isAdmin bool
	err = db.QueryRow("SELECT is_admin FROM users WHERE username = $1", username.Value).Scan(&isAdmin)
	if err != nil {
		http.Error(w, "Ошибка проверки прав", http.StatusInternalServerError)
		return
	}

	if r.Method == "POST" {
		// Обработка формы редактирования
		postID := r.FormValue("id")
		title := r.FormValue("title")
		content := r.FormValue("content")
		imageURL := r.FormValue("image_url")
		location := r.FormValue("location")
		rating := r.FormValue("rating")
		isRecommended := r.FormValue("is_recommended") == "on"

		// Проверяем существование поста и авторство (если не админ)
		var author string
		err := db.QueryRow("SELECT author FROM posts WHERE id = $1", postID).Scan(&author)
		if err != nil {
			http.Error(w, "Пост не найден", http.StatusNotFound)
			return
		}

		if !isAdmin && author != username.Value {
			http.Error(w, "Вы можете редактировать только свои посты", http.StatusForbidden)
			return
		}

		// Обновляем пост в базе данных
		_, err = db.Exec(`
            UPDATE posts SET
                title = $1,
                content = $2,
                image_url = $3,
                location = $4,
                rating = $5,
                is_recommended = $6,
                created_at = created_at  // Сохраняем оригинальную дату создания
            WHERE id = $7`,
			title, content, imageURL, location, rating, isRecommended, postID)

		if err != nil {
			http.Error(w, "Ошибка обновления поста: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Обновляем кэш постов
		loadPostsFromDB()
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// GET запрос - показываем форму редактирования
	postID := r.URL.Query().Get("id")
	if postID == "" {
		http.Error(w, "Не указан ID поста", http.StatusBadRequest)
		return
	}

	var post Post
	query := `
        SELECT id, title, content, image_url, location, rating, is_recommended
        FROM posts WHERE id = $1`
	args := []interface{}{postID}

	// Если не админ - проверяем авторство
	if !isAdmin {
		query += " AND author = $2"
		args = append(args, username.Value)
	}

	err = db.QueryRow(query, args...).
		Scan(&post.ID, &post.Title, &post.Content, &post.ImageURL,
			&post.Location, &post.Rating, &post.IsRecommended)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Пост не найден или у вас нет прав на его редактирование", http.StatusNotFound)
		} else {
			http.Error(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Рендерим шаблон
	tmpl := `
    <!DOCTYPE html>
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

	// Добавляем флаг isAdmin в данные для шаблона
	data := struct {
		Post
		IsAdmin bool
	}{
		Post:    post,
		IsAdmin: isAdmin,
	}

	t := template.Must(template.New("editPost").Parse(tmpl))
	err = t.Execute(w, data)
	if err != nil {
		http.Error(w, "Ошибка отображения страницы: "+err.Error(), http.StatusInternalServerError)
	}
}

func likeHandler(w http.ResponseWriter, r *http.Request) {
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

	var column string
	if action == "like" {
		column = "likes"
	} else if action == "dislike" {
		column = "dislikes"
	} else {
		http.Error(w, "Неверное действие", http.StatusBadRequest)
		return
	}

	// Проверяем, не голосовал ли уже пользователь за этот пост
	var exists bool
	err = db.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM post_votes
            WHERE post_id = $1 AND username = $2
        )`, postID, username.Value).Scan(&exists)

	if err != nil {
		http.Error(w, "Ошибка проверки голоса", http.StatusInternalServerError)
		return
	}

	if exists {
		http.Error(w, "Вы уже голосовали за этот пост", http.StatusBadRequest)
		return
	}

	// Обновляем счетчик лайков/дизлайков
	_, err = db.Exec(`
        UPDATE posts
        SET `+column+` = `+column+` + 1
        WHERE id = $1`, postID)

	if err != nil {
		http.Error(w, "Ошибка обновления поста", http.StatusInternalServerError)
		return
	}

	// Записываем факт голосования
	_, err = db.Exec(`
        INSERT INTO post_votes (post_id, username, vote_type)
        VALUES ($1, $2, $3)`,
		postID, username.Value, action)

	if err != nil {
		http.Error(w, "Ошибка сохранения голоса", http.StatusInternalServerError)
		return
	}

	loadPostsFromDB()
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func main() {
	initDB()
	defer db.Close()

	http.HandleFunc("/", homePage)
	http.HandleFunc("/register", registerPage)
	http.HandleFunc("/login", loginPage)
	http.HandleFunc("/logout", logoutPage)
	http.HandleFunc("/profile", profilePage)
	http.HandleFunc("/new-post", newPostPage)
	http.HandleFunc("/edit-post", editPostPage)
	http.HandleFunc("/like", likeHandler)

	// Админ-роуты
	http.HandleFunc("/admin", adminOnly(adminPanel))
	http.HandleFunc("/admin/toggle-ban", adminOnly(toggleBanUser))
	http.HandleFunc("/admin/delete-post", adminOnly(adminDeletePost))

	fmt.Println("Сервер запущен на http://localhost:8081")
	http.ListenAndServe(":8081", nil)
}
