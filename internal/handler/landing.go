package handler

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/*
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

func (h *Handler) handleLanding(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "landing.html", nil); err != nil {
		h.logger.Error("template render failed", "template", "landing", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
