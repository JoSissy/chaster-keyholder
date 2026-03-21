package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"chaster-keyholder/models"
	"chaster-keyholder/storage"
)

// Server hosts the web dashboard.
type Server struct {
	db           *storage.DB
	statePath    string
	telegramLink string
	password     string
}

// New wires up all routes and returns the http.Handler for the dashboard.
func New(db *storage.DB, statePath, botUsername, password string) http.Handler {
	link := "https://t.me/" + botUsername
	if botUsername == "" {
		link = "#"
	}
	s := &Server{
		db:           db,
		statePath:    statePath,
		telegramLink: link,
		password:     password,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.auth(s.handleDashboard))
	mux.HandleFunc("/calendar", s.auth(s.handleCalendar))
	mux.HandleFunc("/tasks", s.auth(s.handleTasks))
	mux.HandleFunc("/permissions", s.auth(s.handlePermissions))
	mux.HandleFunc("/toys", s.auth(s.handleToys))
	mux.HandleFunc("/wardrobe", s.auth(s.handleWardrobe))
	mux.HandleFunc("/chatasks", s.auth(s.handleChatasks))
	mux.HandleFunc("/gallery", s.auth(s.handleGallery))
	mux.HandleFunc("/contract", s.auth(s.handleContract))
	mux.HandleFunc("/checkins", s.auth(s.handleCheckins))
	return mux
}

// auth wraps a handler with HTTP Basic Auth when a password is set.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	if s.password == "" {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		if !ok || pass != s.password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Jolie's Diary"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// loadState reads state.json and returns the app state.
func (s *Server) loadState() *models.AppState {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		return &models.AppState{}
	}
	var st models.AppState
	if err := json.Unmarshal(data, &st); err != nil {
		return &models.AppState{}
	}
	return &st
}

// render executes the base template with the given page template injected as "content".
func (s *Server) render(w http.ResponseWriter, pageTemplate string, data any) {
	tmpl, err := template.New("base").Funcs(funcMap()).Parse(baseHTML)
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), 500)
		return
	}
	if _, err = tmpl.Parse(pageTemplate); err != nil {
		http.Error(w, "page template parse error: "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("[web] template execute: %v", err)
	}
}

// funcMap returns the template helper functions.
func funcMap() template.FuncMap {
	months := []string{"", "enero", "febrero", "marzo", "abril", "mayo", "junio",
		"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

	return template.FuncMap{
		"formatDate": func(t time.Time) string { return t.Format("02 Jan 2006") },
		"formatShort": func(t time.Time) string { return t.Format("02 Jan") },
		"formatTime": func(t time.Time) string { return t.Format("15:04") },
		"formatDateTime": func(t time.Time) string { return t.Format("02 Jan 2006 15:04") },
		"formatDateTimePtr": func(t *time.Time) string {
			if t == nil {
				return ""
			}
			return t.Format("02 Jan 2006 15:04")
		},
		"formatISO": func(t *time.Time) string {
			if t == nil {
				return ""
			}
			return t.UTC().Format(time.RFC3339)
		},
		"formatDatePtr": func(t *time.Time) string {
			if t == nil {
				return "—"
			}
			return t.Format("02 Jan 2006")
		},
		"monthNameES": func(m int) string {
			if m < 1 || m > 12 {
				return ""
			}
			return months[m]
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"truncate": func(s string, n int) string {
			runes := []rune(s)
			if len(runes) <= n {
				return s
			}
			return string(runes[:n]) + "…"
		},
		"safeURL": func(u string) template.URL { return template.URL(u) },
		"pctStyle": func(part, total int) template.CSS {
			pct := 0
			if total > 0 {
				pct = part * 100 / total
			}
			return template.CSS(fmt.Sprintf("width:%d%%", pct))
		},
		"percent": func(part, total int) int {
			if total == 0 {
				return 0
			}
			return part * 100 / total
		},
		"statusBadge": func(status string) template.HTML {
			switch status {
			case "completed":
				return template.HTML(`<span class="badge badge-success">completada</span>`)
			case "failed":
				return template.HTML(`<span class="badge badge-danger">fallida</span>`)
			default:
				return template.HTML(`<span class="badge badge-warning">pendiente</span>`)
			}
		},
		"typeIcon": func(t string) string {
			switch t {
			case "cage":
				return "🔒"
			case "plug":
				return "🔌"
			case "dildo":
				return "🍆"
			case "vibrator":
				return "💜"
			case "nipple":
				return "🌀"
			case "restraint":
				return "⛓️"
			default:
				return "🎀"
			}
		},
		"typeLabel": func(t string) string {
			switch t {
			case "cage":
				return "Jaula"
			case "plug":
				return "Plug"
			case "dildo":
				return "Dildo"
			case "vibrator":
				return "Vibrador"
			case "nipple":
				return "Ventosas"
			case "restraint":
				return "Restricción"
			default:
				return "Juguete"
			}
		},
		"clothingIcon": func(t string) string {
			switch t {
			case "thong":
				return "🩲"
			case "bra":
				return "👙"
			case "stockings":
				return "🦵"
			case "socks":
				return "🧦"
			case "collar":
				return "💎"
			case "lingerie":
				return "🌸"
			case "dress":
				return "👗"
			case "top":
				return "👚"
			case "bottom":
				return "👘"
			case "shoes":
				return "👠"
			case "accessory":
				return "💍"
			default:
				return "🎀"
			}
		},
		"clothingLabel": func(t string) string {
			switch t {
			case "thong":
				return "Tanga"
			case "bra":
				return "Sujetador"
			case "stockings":
				return "Medias"
			case "socks":
				return "Calcetines"
			case "collar":
				return "Collar"
			case "lingerie":
				return "Lencería"
			case "dress":
				return "Vestido"
			case "top":
				return "Top"
			case "bottom":
				return "Falda/Pantalón"
			case "shoes":
				return "Zapatos"
			case "accessory":
				return "Accesorio"
			default:
				return "Prenda"
			}
		},
	}
}
