package main

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"gox2/admin"
	"gox2/authentication"
	"gox2/db"
	"gox2/handlers"
	"gox2/posts"
	"gox2/users"

	_ "github.com/lib/pq"
)

func initTemplates() (*template.Template, error) {
	return template.New("").Funcs(template.FuncMap{
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
}

func main() {
	tmpl, err := initTemplates()
	if err != nil {
		panic(fmt.Sprintf("Не удалось загрузить шаблоны: %v", err))
	}

	database, err := db.New("user=postgres dbname=go password=0000 host=localhost sslmode=disable")
	if err != nil {
		panic(fmt.Sprintf("Не удалось подключиться к БД: %v", err))
	}
	defer database.Close()

	userRepo := users.NewUserRepository(database.DB)
	postRepo := posts.NewPostRepository(database.DB)

	authHandler := authentication.NewAuthHandler(userRepo, tmpl)
	adminHandler := admin.NewAdminHandler(userRepo, postRepo, tmpl)
	appHandlers := handlers.NewAppHandlers(authHandler, userRepo, postRepo, adminHandler, tmpl)

	// Основные маршруты
	http.HandleFunc("/", appHandlers.HomePage)
	http.HandleFunc("/register", authHandler.RegisterPage)
	http.HandleFunc("/login", authHandler.LoginPage)
	http.HandleFunc("/logout", authHandler.LogoutPage)
	http.HandleFunc("/profile", appHandlers.ProfilePage)
	http.HandleFunc("/new-post", appHandlers.NewPostPage)
	http.HandleFunc("/edit-post", appHandlers.EditPostPage)
	http.HandleFunc("/like", appHandlers.LikeHandler)

	// Админские маршруты
	http.HandleFunc("/admin", appHandlers.AdminOnly(adminHandler.AdminPanel))
	http.HandleFunc("/admin/toggle-ban", appHandlers.AdminOnly(adminHandler.ToggleBanUser))
	http.HandleFunc("/admin/delete-post", appHandlers.AdminOnly(adminHandler.DeletePost))

	fmt.Println("Сервер запущен на http://localhost:8081")
	http.ListenAndServe(":8081", nil)
}
