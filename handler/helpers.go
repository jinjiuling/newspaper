package handler

import (
	"html/template"
	"net/http"
	"os"
)

// TemplateFuncs returns custom template functions
var TemplateFuncs = template.FuncMap{
	"minus": func(a, b int) int { return a - b },
	"plus":  func(a, b int) int { return a + b },
    "safeHTML": func(s string) template.HTML { return template.HTML(s) },
}

// execHTML sets Content-Type for HTML responses and executes the template
func execHTML(w http.ResponseWriter, t *template.Template, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.Execute(w, data)
}

func getAdminPassword() string {
	return os.Getenv("ADMIN_PASSWORD")
}
