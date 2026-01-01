package web

import (
	"database/sql" // Added for sql.NullTime
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/conorfennell/knolhash/internal/fsrs"
	"github.com/conorfennell/knolhash/internal/storage"
)

//go:embed all:static
var staticFiles embed.FS

// Server holds the dependencies for the HTTP server.
type Server struct {
	db     *storage.DB
	router *http.ServeMux
	fsrs   *fsrs.Params // Add FSRS parameters to the server
}

// NewServer creates and configures a new server.
func NewServer(db *storage.DB) *Server {
	s := &Server{
		db:     db,
		router: http.NewServeMux(),
		fsrs:   fsrs.DefaultParams(), // Initialize FSRS with default parameters
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
	// Create a file server for the embedded static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create sub-filesystem for static assets: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	// Serve static files
	s.router.Handle("/static/", http.StripPrefix("/static/", fileServer))
	s.router.Handle("/", fileServer)

	// API endpoints
	s.router.HandleFunc("/api/next", s.handleGetNextCard())
	s.router.HandleFunc("/api/review", s.handlePostReview())
	s.router.HandleFunc("/api/sync", s.handlePostSync())
}

// NextCardResponse represents the JSON response for the next card.
type NextCardResponse struct {
	Hash     string `json:"hash"`
	Question string `json:"question"`
}
// ... (rest of the file is the same)

// handleGetNextCard returns the next due card.
func (s *Server) handleGetNextCard() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		dueCards, err := s.db.GetDueCards()
		if err != nil {
			log.Printf("Error getting due cards: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if len(dueCards) == 0 {
			w.WriteHeader(http.StatusNoContent)
			fmt.Fprintf(w, "No cards due for review.")
			return
		}

		nextCard := dueCards[0] // Get the first due card
		resp := NextCardResponse{
			Hash:     nextCard.Hash,
			Question: nextCard.Question,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// ReviewRequest represents the JSON request for a card review.
type ReviewRequest struct {
	CardHash string    `json:"card_hash"`
	Grade    fsrs.Rating `json:"grade"`
}

// handlePostReview accepts a card review and updates its state.
func (s *Server) handlePostReview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ReviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Find the card state
		cardState, err := s.db.FindCardStateByHash(req.CardHash)
		if err != nil {
			log.Printf("Error finding card state for hash %s: %v", req.CardHash, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if cardState == nil {
			http.Error(w, "Card not found", http.StatusNotFound)
			return
		}

		// Convert storage.CardState to fsrs.CardState
		currentFSRSState := fsrs.CardState{
			Stability:  cardState.Stability,
			Difficulty: cardState.Difficulty,
			LastReview: cardState.LastReview.Time, // Assuming LastReview is valid
		}
		
		// Calculate new FSRS state
		newFSRSState := s.fsrs.NextState(currentFSRSState, req.Grade)

		// Calculate new due date

		newDueDate := fsrs.NextDueDate(newFSRSState.Stability)

		// Update storage.CardState
		cardState.Stability = newFSRSState.Stability
		cardState.Difficulty = newFSRSState.Difficulty
		cardState.DueDate = newDueDate
		cardState.LastReview = sql.NullTime{Time: newFSRSState.LastReview, Valid: true} // Update LastReview
		cardState.State = 2 // Mark as reviewed (Learning/Review state for now)

		// Update card in DB
		if err := s.db.UpdateCardState(cardState); err != nil {
			log.Printf("Error updating card state for hash %s: %v", req.CardHash, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// handlePostSync is a placeholder for triggering a manual sync.
func (s *Server) handlePostSync() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// In a real implementation, this would trigger the reconciliation logic
		// which is currently in cmd/knolhash/main.go. This will require refactoring.
		fmt.Fprintf(w, "Sync triggered (placeholder)")
	}
}

