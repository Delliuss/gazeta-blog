package authentication

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		user, err := h.users.GetUser(username)
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

	err := h.tmpl.ExecuteTemplate(w, "login.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
