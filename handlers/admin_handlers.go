package handlers

import "net/http"

// AdminOnly middleware проверяет права администратора
func (h *AppHandlers) AdminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, err := r.Cookie("username")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		user, err := h.users.GetUser(username.Value)
		if err != nil || !user.IsAdmin {
			http.Error(w, "Доступ запрещён: требуется права администратора", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
