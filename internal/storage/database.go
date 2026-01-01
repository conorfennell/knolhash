package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/conorfennell/knolhash/internal/domain"
	_ "modernc.org/sqlite" // Registers the sqlite driver
)

// DB represents a wrapper around the SQL database connection.
type DB struct {
	conn *sql.DB
}

// Open creates a new database connection and ensures the schema is up to date.
func Open(dsn string) (*DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Execute the schema to create tables if they don't exist.
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return &DB{conn: db}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// CardState represents the FSRS state of a card.
type CardState struct {
	Hash       string
	Question   string
	Stability  float64
	Difficulty float64
	DueDate    time.Time
	LastReview sql.NullTime // Use NullTime for nullable last_review
	State      int          // 0: New, 1: Learning, 2: Review
	SourceID   sql.NullInt64 // Use NullInt64 for nullable source_id
}

// InsertCard inserts a new card into the database.
// It also sets initial FSRS values for new cards.
func (db *DB) InsertCard(card domain.Card, sourceID int66) error {
	_, err := db.conn.Exec(`
		INSERT INTO cards (hash, question, stability, difficulty, due_date, state, source_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		card.Hash,
		card.Question,
		0.0, // Initial stability
		0.0, // Initial difficulty
		time.Now(), // Initial due date (today)
		0,   // Initial state: New
		sourceID,
	)
	if err != nil {
		return fmt.Errorf("failed to insert card %s: %w", card.Hash, err)
	}
	return nil
}

// FindCardStateByHash retrieves a card's state from the database by its hash.
func (db *DB) FindCardStateByHash(hash string) (*CardState, error) {
	var cs CardState
	row := db.conn.QueryRow(`
		SELECT hash, question, stability, difficulty, due_date, last_review, state, source_id
		FROM cards WHERE hash = ?
	`, hash)

	err := row.Scan(
		&cs.Hash,
		&cs.Question,
		&cs.Stability,
		&cs.Difficulty,
		&cs.DueDate,
		&cs.LastReview,
		&cs.State,
		&cs.SourceID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Card not found
		}
		return nil, fmt.Errorf("failed to find card state by hash %s: %w", hash, err)
	}
	return &cs, nil
}

// UpdateCardState updates an existing card's FSRS state and review information.
func (db *DB) UpdateCardState(cs *CardState) error {
	_, err := db.conn.Exec(`
		UPDATE cards
		SET stability = ?, difficulty = ?, due_date = ?, last_review = ?, state = ?
		WHERE hash = ?
	`,
		cs.Stability,
		cs.Difficulty,
		cs.DueDate,
		cs.LastReview,
		cs.State,
		cs.Hash,
	)
	if err != nil {
		return fmt.Errorf("failed to update card state for hash %s: %w", cs.Hash, err)
	}
	return nil
}
