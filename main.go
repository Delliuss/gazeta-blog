package main

import (
	"fmt"
	"net/http"

	"gox2/admin"
	"gox2/authentication"
	"gox2/config"
	"gox2/db"
	"gox2/handlers"
	"gox2/repositories"
	"gox2/routes"

	_ "github.com/lib/pq"
)

func main() {
	// Инициализация шаблонов
	tmpl, err := InitTemplates()
	if err != nil {
		panic(fmt.Sprintf("Не удалось загрузить шаблоны: %v", err))
	}

	// Загрузка конфигурации БД
	dbConfig, err := config.LoadDBConfig()
	if err != nil {
		panic(fmt.Sprintf("Не удалось загрузить конфигурацию БД: %v", err))
	}

	// Подключение к базе данных
	database, err := db.New(dbConfig.ConnectionString())
	if err != nil {
		panic(fmt.Sprintf("Не удалось подключиться к БД: %v", err))
	}
	defer database.Close()

	// Инициализация репозиториев
	repos := repositories.Init(database.DB)

	// Создание обработчиков
	authHandler := authentication.NewAuthHandler(repos.UserRepo, tmpl)
	adminHandler := admin.NewAdminHandler(repos.UserRepo, repos.PostRepo, tmpl)
	appHandlers := handlers.NewAppHandlers(authHandler, repos.UserRepo, repos.PostRepo, adminHandler, tmpl)

	// Настройка маршрутов
	routes.Setup(appHandlers, authHandler, adminHandler)

	// Запуск сервера
	fmt.Println("Сервер запущен на http://localhost:8081")
	http.ListenAndServe(":8081", nil)
}
