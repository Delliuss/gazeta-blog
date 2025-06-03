package main

import (
	"html/template"
	"time"
)

func InitTemplates() (*template.Template, error) {
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
