package main

import (
	"fmt"
	"net/http"

	"gox2/authentication"
	"gox2/config"
	"gox2/db"
	"gox2/handlers"
	"gox2/repositories"
	"gox2/routes"

	_ "github.com/lib/pq"
)

func main() {
	tmpl, err := InitTemplates()
	if err != nil {
		panic(fmt.Sprintf("Не удалось загрузить шаблоны: %v", err))
	}

	dbConfig, err := config.LoadDBConfig()
	if err != nil {
		panic(fmt.Sprintf("Не удалось загрузить конфигурацию БД: %v", err))
	}

	database, err := db.New(dbConfig.ConnectionString())
	if err != nil {
		panic(fmt.Sprintf("Не удалось подключиться к БД: %v", err))
	}
	defer database.Close()

	repos := repositories.Init(database.DB)

	authHandler := authentication.NewAuthHandler(repos.UserRepo, tmpl)
	adminHandler := handlers.NewAdminHandler(repos.UserRepo, repos.PostRepo, tmpl)
	appHandlers := handlers.NewAppHandlers(authHandler, repos.UserRepo, repos.PostRepo, adminHandler, tmpl)

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	routes.Setup(appHandlers, authHandler, adminHandler)

	fmt.Println("Сервер запущен на http://localhost:8081")
	http.ListenAndServe(":8081", nil)
}
