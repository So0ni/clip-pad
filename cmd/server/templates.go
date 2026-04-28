package main

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"time"
)

type templateRenderer struct {
	templates map[string]*template.Template
}

func newTemplateRenderer(basePath string) (*templateRenderer, error) {
	funcMap := template.FuncMap{
		"formatTime": func(value *time.Time) string {
			if value == nil {
				return "Never"
			}
			return value.UTC().Format(time.RFC3339)
		},
		"formatTimeValue": func(value time.Time) string {
			return value.UTC().Format(time.RFC3339)
		},
	}

	pages := []string{"index.html", "paste.html", "burn.html", "notepad.html", "notfound.html"}
	renderer := &templateRenderer{templates: make(map[string]*template.Template, len(pages))}
	layout := filepath.Join(basePath, "layout.html")
	for _, page := range pages {
		tmpl, err := template.New("layout.html").Funcs(funcMap).ParseFiles(layout, filepath.Join(basePath, page))
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", page, err)
		}
		renderer.templates[page] = tmpl
	}
	return renderer, nil
}

func (r *templateRenderer) Render(w http.ResponseWriter, status int, name string, data any) error {
	tmpl, ok := r.templates[name]
	if !ok {
		return fmt.Errorf("template %s not found", name)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	return tmpl.ExecuteTemplate(w, "layout", data)
}
