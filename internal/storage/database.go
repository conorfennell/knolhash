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
func (db *DB) InsertCard(card domain.Card, sourceID int64) error {
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

// Source represents a card source, either a local path or a Git URL.
type Source struct {
	ID         int64
	Path       string
	LastScanned sql.NullTime
}

// InsertSource inserts a new source path into the database and returns its ID.
func (db *DB) InsertSource(path string) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO sources (path, last_scanned)
		VALUES (?, ?)
	`, path, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to insert source %s: %w", path, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID for source %s: %w", path, err)
	}
	return id, nil
}

// FindSourceByPath retrieves a source from the database by its path.
func (db *DB) FindSourceByPath(path string) (*Source, error) {
	var s Source
	row := db.conn.QueryRow(`
		SELECT id, path, last_scanned
		FROM sources WHERE path = ?
	`, path)

	err := row.Scan(&s.ID, &s.Path, &s.LastScanned)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Source not found
		}
		return nil, fmt.Errorf("failed to find source by path %s: %w", path, err)
	}
	return &s, nil
}

// GetAllSources retrieves all stored sources from the database.
func (db *DB) GetAllSources() ([]Source, error) {
	rows, err := db.conn.Query(`
		SELECT id, path, last_scanned
		FROM sources
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all sources: %w", err)
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.ID, &s.Path, &s.LastScanned); err != nil {
			return nil, fmt.Errorf("failed to scan source row: %w", err)
		}
		sources = append(sources, s)
	}
	return sources, nil
}

// UpdateSourceLastScanned updates the last_scanned timestamp for a source.
func (db *DB) UpdateSourceLastScanned(sourceID int64) error {
	_, err := db.conn.Exec(`
		UPDATE sources
		SET last_scanned = ?
		WHERE id = ?
	`, time.Now(), sourceID)
	if err != nil {
		return fmt.Errorf("failed to update last scanned for source ID %d: %w", sourceID, err)
	}
	return nil
}

// GetCardsBySourceID retrieves all card states associated with a specific source ID.
func (db *DB) GetCardsBySourceID(sourceID int64) ([]CardState, error) {
	rows, err := db.conn.Query(`
		SELECT hash, question, stability, difficulty, due_date, last_review, state, source_id
		FROM cards WHERE source_id = ?
	`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cards for source ID %d: %w", sourceID, err)
	}
	defer rows.Close()

	var cardStates []CardState
	for rows.Next() {
		var cs CardState
		if err := rows.Scan(
			&cs.Hash,
			&cs.Question,
			&cs.Stability,
			&cs.Difficulty,
			&cs.DueDate,
			&cs.LastReview,
			&cs.State,
			&cs.SourceID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan card state row for source ID %d: %w", sourceID, err)
		}
		cardStates = append(cardStates, cs)
	}
	return cardStates, nil
}

// DeleteCardByHash removes a card from the database by its hash.
func (db *DB) DeleteCardByHash(hash string) error {
	_, err := db.conn.Exec(`
		DELETE FROM cards
		WHERE hash = ?
	`, hash)
	if err != nil {
		return fmt.Errorf("failed to delete card with hash %s: %w", hash, err)
	}
	return nil
}

