package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

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
}

var posts []Post
var mu sync.Mutex
var db *sql.DB

func initDB() {
	var err error
	connStr := "user=postgres dbname=go password=0000 host=localhost sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}

	// Проверяем соединение с БД
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	// Создаем таблицы, если они не существуют
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			username TEXT NOT NULL PRIMARY KEY,
			password TEXT NOT NULL
		);
		
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

	// Добавляем тестовые данные, если их нет
	seedTestData()
}
func seedTestData() {
	// Проверяем, есть ли уже пользователи
	var userCount int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		panic(err)
	}

	if userCount == 0 {
		// Добавляем тестовых пользователей
		_, err = db.Exec(`
			INSERT INTO users (username, password) VALUES 
			('foodlover', 'gourmet123'),
			('traveler', 'wanderlust456');
		`)
		if err != nil {
			panic(err)
		}

		// Добавляем тестовые посты
		_, err = db.Exec(`
			INSERT INTO posts 
			(title, content, image_url, location, rating, author, is_recommended) 
			VALUES 
			('La Pergola - Рим', 'Лучший вид на город и потрясающая кухня!', 'https://example.com/lapergola.jpg', 'Рим, Италия', 5, 'foodlover', TRUE),
			('Sukiyabashi Jiro - Токио', 'Легендарные суши от мастера', 'https://example.com/jiro.jpg', 'Токио, Япония', 5, 'traveler', TRUE),
			('Le Jules Verne - Париж', 'Ужин с видом на Эйфелеву башню', 'https://example.com/julesverne.jpg', 'Париж, Франция', 4, 'foodlover', FALSE);
		`)
		if err != nil {
			panic(err)
		}
	}

	// Загружаем посты из базы
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

func homePage(w http.ResponseWriter, r *http.Request) {
	tmpl := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Вкусная еда в путешествиях</title>
		<style>
			.post { border: 1px solid #ddd; padding: 15px; margin-bottom: 20px; border-radius: 5px; }
			.likes { color: green; font-weight: bold; }
			.dislikes { color: red; font-weight: bold; }
			.recommended { background-color: #f8fff8; border-left: 4px solid #4CAF50; }
			.rating { color: #FFA500; font-weight: bold; }
			.location { color: #666; font-style: italic; }
			.author { font-size: 0.9em; color: #333; }
			.date { font-size: 0.8em; color: #999; }
		</style>
	</head>
	<body>
		<h1>Вкусная еда в путешествиях</h1>
		
		{{if .Username}}
			<p>Добро пожаловать, {{.Username}}! | <a href="/profile">Мой профиль</a> | <a href="/new-post">Новый пост</a> | <a href="/logout">Выйти</a></p>
		{{else}}
			<p><a href="/login">Войти</a> | <a href="/register">Зарегистрироваться</a></p>
		{{end}}
		
		<form method="GET" action="/" style="margin-bottom: 20px;">
			<input type="text" name="search" placeholder="Поиск по местоположению или названию" style="padding: 8px; width: 300px;">
			<input type="submit" value="Поиск" style="padding: 8px 15px;">
		</form>
		
		<h2>Последние отзывы</h2>
		{{if .Posts}}
			{{range .Posts}}
				<div class="post {{if .IsRecommended}}recommended{{end}}">
					<h3>{{.Title}}</h3>
					<p class="location">{{.Location}}</p>
					<p class="rating">Оценка: {{.Rating}}/5 {{if .IsRecommended}}⭐ Рекомендую!{{end}}</p>
					{{if .ImageURL}}<img src="{{.ImageURL}}" alt="{{.Title}}" style="max-width: 300px; margin: 10px 0;">{{end}}
					<p>{{.Content}}</p>
					<div style="margin-top: 15px;">
						<span class="likes">👍 {{.Likes}}</span> | 
						<span class="dislikes">👎 {{.Dislikes}}</span>
						<span style="float: right;">
							<span class="author">{{.Author}}</span>, 
							<span class="date">{{.CreatedAt.Format "02.01.2006 15:04"}}</span>
						</span>
					</div>
					
					{{if $.Username}}
						<div style="margin-top: 10px;">
							<form method="POST" action="/like" style="display: inline;">
								<input type="hidden" name="post_id" value="{{.ID}}">
								<input type="hidden" name="action" value="like">
								<input type="submit" value="👍 Нравится" style="padding: 5px 10px;">
							</form>
							<form method="POST" action="/like" style="display: inline;">
								<input type="hidden" name="post_id" value="{{.ID}}">
								<input type="hidden" name="action" value="dislike">
								<input type="submit" value="👎 Не нравится" style="padding: 5px 10px;">
							</form>
						</div>
					{{end}}
				</div>
			{{end}}
		{{else}}
			<p>Пока нет отзывов. Будьте первым!</p>
		{{end}}
	</body>
	</html>`

	search := r.URL.Query().Get("search")
	var filteredPosts []Post

	for _, post := range posts {
		if search == "" || contains(post.Title, search) || contains(post.Location, search) {
			filteredPosts = append(filteredPosts, post)
		}
	}

	username := ""
	if cookie, err := r.Cookie("username"); err == nil {
		username = cookie.Value
	}

	data := struct {
		Posts    []Post
		Username string
	}{
		Posts:    filteredPosts,
		Username: username,
	}

	t, _ := template.New("webpage").Parse(tmpl)
	t.Execute(w, data)
}

func registerPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		mu.Lock()
		_, err := db.Exec("INSERT INTO users (username, password) VALUES ($1, $2)", username, password)
		mu.Unlock()

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

		var storedPassword string
		err := db.QueryRow("SELECT password FROM users WHERE username = $1", username).Scan(&storedPassword)

		if err != nil || storedPassword != password {
			http.Error(w, "Неверное имя пользователя или пароль", http.StatusUnauthorized)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  "username",
			Value: username,
			Path:  "/",
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
	// Проверяем авторизацию пользователя
	username, err := r.Cookie("username")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		// Обработка отправки формы редактирования
		postID := r.FormValue("id")
		title := r.FormValue("title")
		content := r.FormValue("content")
		imageURL := r.FormValue("image_url")
		location := r.FormValue("location")
		rating := r.FormValue("rating")
		isRecommended := r.FormValue("is_recommended") == "on"

		// Обновляем пост в базе данных
		_, err := db.Exec(`
            UPDATE posts 
            SET title = $1, content = $2, image_url = $3, location = $4, 
                rating = $5, is_recommended = $6
            WHERE id = $7 AND author = $8`,
			title, content, imageURL, location, rating, isRecommended, postID, username.Value)

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
	err = db.QueryRow(`
        SELECT id, title, content, image_url, location, rating, is_recommended 
        FROM posts 
        WHERE id = $1 AND author = $2`,
		postID, username.Value).
		Scan(&post.ID, &post.Title, &post.Content, &post.ImageURL, &post.Location, &post.Rating, &post.IsRecommended)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Пост не найден или у вас нет прав на его редактирование", http.StatusNotFound)
		} else {
			http.Error(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

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
            .submit-btn { background: #4CAF50; color: white; padding: 10px 15px; border: none; cursor: pointer; }
            .submit-btn:hover { background: #45a049; }
        </style>
    </head>
    <body>
        <h1>Редактировать отзыв</h1>
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

	t, _ := template.New("editPost").Parse(tmpl)
	t.Execute(w, post)
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

func main() {
	// Инициализация базы данных
	initDB()
	defer db.Close()

	// Настройка маршрутов
	http.HandleFunc("/", homePage)
	http.HandleFunc("/register", registerPage)
	http.HandleFunc("/login", loginPage)
	http.HandleFunc("/logout", logoutPage)
	http.HandleFunc("/profile", profilePage)
	http.HandleFunc("/new-post", newPostPage)
	http.HandleFunc("/edit-post", editPostPage)
	http.HandleFunc("/like", likeHandler)

	// Запуск сервера
	fmt.Println("Сервер запущен на http://localhost:8081")
	http.ListenAndServe(":8081", nil)
}
