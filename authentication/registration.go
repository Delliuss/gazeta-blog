package authentication

import (
	"net/http"
)

func (h *AuthHandler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		err := h.users.CreateUser(username, password)
		if err != nil {
			http.Error(w, "Ошибка регистрации: имя пользователя уже занято", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err := h.tmpl.ExecuteTemplate(w, "register.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
