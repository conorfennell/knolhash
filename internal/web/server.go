package web

import (
	"database/sql"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/conorfennell/knolhash/internal/fsrs"
	"github.com/conorfennell/knolhash/internal/storage"
	"github.com/conorfennell/knolhash/internal/sync"
)

//go:embed all:static
var staticFiles embed.FS

//go:embed all:templates
var templateFiles embed.FS

// Server holds the dependencies for the HTTP server.
type Server struct {
	db        *storage.DB
	router    *http.ServeMux
	fsrs      *fsrs.Params
	templates *template.Template
}

// NewServer creates and configures a new server.
func NewServer(db *storage.DB) *Server {
	// Parse templates
	tpl, err := template.ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		slog.Error("Failed to parse templates", "error", err)
		os.Exit(1)
	}

	s := &Server{
		db:        db,
		router:    http.NewServeMux(),
		fsrs:      fsrs.DefaultParams(),
		templates: tpl,
	}
	s.routes()
	return s
}

// ServeHTTP implements the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// routes sets up the routing for the server.
func (s *Server) routes() {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		slog.Error("Failed to create sub-filesystem for static assets", "error", err)
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	s.router.Handle("/static/", http.StripPrefix("/static/", fileServer))
	s.router.Handle("/", fileServer)

	// HTMX-based routes
	s.router.HandleFunc("/deck", s.handleGetDeck())
	s.router.HandleFunc("/review/next", s.handleGetNextReview())
	s.router.HandleFunc("/review/answer/", s.handleShowAnswer())
	s.router.HandleFunc("/review/", s.handlePostReview())

	// Source management routes
	s.router.HandleFunc("/sources", s.handleSources())
	s.router.HandleFunc("/sources/", s.handleDeleteSource())
	s.router.HandleFunc("/sync", s.handlePostSync())
}

// handlePostSync triggers a manual sync and re-renders the source list.
func (s *Server) handlePostSync() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		sync.RunSync(s.db) // Run in the foreground to make the user wait

		// Re-render the source list to be swapped by HTMX
		sources, err := s.db.GetAllSources()
		if err != nil {
			slog.Error("Error getting sources after sync", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		data := map[string]interface{}{
			"Sources": sources,
		}

		// Render both the success message and the updated list
		s.templates.ExecuteTemplate(w, "sync_success", nil)
		s.templates.ExecuteTemplate(w, "source_list", data)
	}
}

// handleSources handles both GET and POST for the sources page.
func (s *Server) handleSources() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleGetSources(w, r)
		case http.MethodPost:
			s.handlePostSource(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleGetSources renders the main sources management page.
func (s *Server) handleGetSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.db.GetAllSources()
	if err != nil {
		slog.Error("Error getting sources", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	data := map[string]interface{}{
		"Sources": sources,
	}
	s.templates.ExecuteTemplate(w, "sources", data)
}

// handlePostSource adds a new source and re-renders the source list.
func (s *Server) handlePostSource(w http.ResponseWriter, r *http.Request) {
	path := r.PostFormValue("path")
	if path == "" {
		http.Error(w, "Path cannot be empty", http.StatusBadRequest)
		return
	}

	// This is a simplified version of the logic in main.go's addNewSource.
	// A refactoring would be to move that logic into a shared package.
	sourceType := "local"
	if strings.HasSuffix(path, ".git") || strings.HasPrefix(path, "git@") || strings.HasPrefix(path, "https://") {
		sourceType = "git"
	}

	if _, err := s.db.InsertSource(path, sourceType); err != nil {
		slog.Error("Error inserting new source", "error", err)
		http.Error(w, "Failed to add source", http.StatusInternalServerError)
		return
	}

	// Re-render the source list to be swapped by HTMX
	sources, err := s.db.GetAllSources()
	if err != nil {
		slog.Error("Error getting sources after add", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	data := map[string]interface{}{
		"Sources": sources,
	}
	s.templates.ExecuteTemplate(w, "source_list", data)
}

// handleDeleteSource deletes a source and re-renders the source list.
func (s *Server) handleDeleteSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		idStr := strings.TrimPrefix(r.URL.Path, "/sources/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid source ID", http.StatusBadRequest)
			return
		}

		if err := s.db.DeleteSource(id); err != nil {
			slog.Error("Error deleting source", "id", id, "error", err)
			http.Error(w, "Failed to delete source", http.StatusInternalServerError)
			return
		}

		// Re-render the source list to be swapped by HTMX
		sources, err := s.db.GetAllSources()
		if err != nil {
			slog.Error("Error getting sources after delete", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		data := map[string]interface{}{
			"Sources": sources,
		}
		s.templates.ExecuteTemplate(w, "source_list", data)
	}
}

// handleGetDeck renders the deck view, showing the number of due cards.
func (s *Server) handleGetDeck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dueCards, err := s.db.GetDueCards()
		if err != nil {
			slog.Error("Error getting due cards for deck view", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		data := map[string]interface{}{
			"DueCount":    len(dueCards),
			"HasDueCards": len(dueCards) > 0,
		}
		s.templates.ExecuteTemplate(w, "deck", data)
	}
}

// handleGetNextReview renders the front of the next due card.
func (s *Server) handleGetNextReview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dueCards, err := s.db.GetDueCards()
		if err != nil {
			slog.Error("Error getting next due card", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if len(dueCards) == 0 {
			s.templates.ExecuteTemplate(w, "deck", map[string]interface{}{
				"DueCount":    0,
				"HasDueCards": false,
			})
			return
		}
		nextCard := dueCards[0]
		s.templates.ExecuteTemplate(w, "card_front", nextCard)
	}
}

// handleShowAnswer renders the back of a card.
func (s *Server) handleShowAnswer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hash := strings.TrimPrefix(r.URL.Path, "/review/answer/")
		card, err := s.db.FindCardByHash(hash)
		if err != nil || card == nil {
			http.NotFound(w, r)
			return
		}
		s.templates.ExecuteTemplate(w, "card_back", card)
	}
}

// handlePostReview processes a review and renders the next card.
func (s *Server) handlePostReview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hash := strings.TrimPrefix(r.URL.Path, "/review/")
		gradeStr := r.PostFormValue("grade")
		grade, err := strconv.Atoi(gradeStr)
		if err != nil {
			http.Error(w, "Invalid grade", http.StatusBadRequest)
			return
		}

		card, err := s.db.FindCardByHash(hash)
		if err != nil || card == nil {
			http.NotFound(w, r)
			return
		}

		currentFSRSState := fsrs.CardState{
			Stability:  card.Stability,
			Difficulty: card.Difficulty,
			LastReview: card.LastReview.Time,
		}

		newFSRSState := s.fsrs.NextState(currentFSRSState, fsrs.Rating(grade))
		newDueDate := fsrs.NextDueDate(newFSRSState.Stability)

		card.Stability = newFSRSState.Stability
		card.Difficulty = newFSRSState.Difficulty
		card.DueDate = newDueDate
		card.LastReview = sql.NullTime{Time: newFSRSState.LastReview, Valid: true}
		card.State = 2 // Mark as in-review

		if err := s.db.UpdateCard(card); err != nil {
			slog.Error("Error updating card state for hash", "hash", hash, "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// After review, show the next card
		s.handleGetNextReview()(w, r)
	}
}

