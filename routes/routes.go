package routes

import (
	"gox2/admin"
	"gox2/authentication"
	"gox2/handlers"
	"net/http"
)

func Setup(h *handlers.AppHandlers, auth *authentication.AuthHandler, admin *admin.AdminHandler) {
	http.HandleFunc("/", h.HomePage)
	http.HandleFunc("/register", auth.RegisterPage)
	http.HandleFunc("/login", auth.LoginPage)
	http.HandleFunc("/logout", auth.LogoutPage)
	http.HandleFunc("/profile", h.ProfilePage)
	http.HandleFunc("/new-post", h.NewPostPage)
	http.HandleFunc("/edit-post", h.EditPostPage)
	http.HandleFunc("/like", h.LikeHandler)

	http.HandleFunc("/admin", h.AdminOnly(admin.AdminPanel))
	http.HandleFunc("/admin/toggle-ban", h.AdminOnly(admin.ToggleBanUser))
	http.HandleFunc("/admin/delete-post", h.AdminOnly(admin.DeletePost))
}
