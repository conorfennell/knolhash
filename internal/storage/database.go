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

// Card represents the data for a card as stored in the database.
type Card struct {
	Hash       string
	Question   string
	Answer     string
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
		INSERT INTO cards (hash, question, answer, stability, difficulty, due_date, state, source_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		card.Hash,
		card.Question,
		card.Answer,
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

// FindCardByHash retrieves a card's state from the database by its hash.
func (db *DB) FindCardByHash(hash string) (*Card, error) {
	var cs Card
	row := db.conn.QueryRow(`
		SELECT hash, question, answer, stability, difficulty, due_date, last_review, state, source_id
		FROM cards WHERE hash = ?
	`, hash)

	err := row.Scan(
		&cs.Hash,
		&cs.Question,
		&cs.Answer,
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
		return nil, fmt.Errorf("failed to find card by hash %s: %w", hash, err)
	}
	return &cs, nil
}

// UpdateCard updates an existing card's FSRS state and review information.
func (db *DB) UpdateCard(cs *Card) error {
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
		return fmt.Errorf("failed to update card for hash %s: %w", cs.Hash, err)
	}
	return nil
}

// Source represents a card source, either a local path or a Git URL.
type Source struct {
	ID          int64
	Path        string
	Type        string // 'local' or 'git'
	LastScanned sql.NullTime
}

// InsertSource inserts a new source path into the database and returns its ID.
func (db *DB) InsertSource(path, sourceType string) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO sources (path, type, last_scanned)
		VALUES (?, ?, ?)
	`, path, sourceType, time.Now())
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
		SELECT id, path, type, last_scanned
		FROM sources WHERE path = ?
	`, path)

	err := row.Scan(&s.ID, &s.Path, &s.Type, &s.LastScanned)
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
		SELECT id, path, type, last_scanned
		FROM sources
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all sources: %w", err)
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.ID, &s.Path, &s.Type, &s.LastScanned); err != nil {
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
func (db *DB) GetCardsBySourceID(sourceID int64) ([]Card, error) {
	rows, err := db.conn.Query(`
		SELECT hash, question, answer, stability, difficulty, due_date, last_review, state, source_id
		FROM cards WHERE source_id = ?
	`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cards for source ID %d: %w", sourceID, err)
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var cs Card
		if err := rows.Scan(
			&cs.Hash,
			&cs.Question,
			&cs.Answer,
			&cs.Stability,
			&cs.Difficulty,
			&cs.DueDate,
			&cs.LastReview,
			&cs.State,
			&cs.SourceID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan card row for source ID %d: %w", sourceID, err)
		}
		cards = append(cards, cs)
	}
	return cards, nil
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

// GetDueCards retrieves all cards that are due for review, sorted by due date.
func (db *DB) GetDueCards() ([]Card, error) {
	rows, err := db.conn.Query(`
		SELECT hash, question, answer, stability, difficulty, due_date, last_review, state, source_id
		FROM cards
		WHERE due_date <= ?
		ORDER BY due_date ASC
	`, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get due cards: %w", err)
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var cs Card
		if err := rows.Scan(
			&cs.Hash,
			&cs.Question,
			&cs.Answer,
			&cs.Stability,
			&cs.Difficulty,
			&cs.DueDate,
			&cs.LastReview,
			&cs.State,
			&cs.SourceID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan due card row: %w", err)
		}
		cards = append(cards, cs)
	}
	return cards, nil
}

// DeleteSource deletes a source and all its associated cards from the database.
func (db *DB) DeleteSource(id int64) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback on error or if not committed

	// Delete associated cards first
	_, err = tx.Exec(`DELETE FROM cards WHERE source_id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete cards for source %d: %w", id, err)
	}

	// Delete the source itself
	_, err = tx.Exec(`DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete source %d: %w", id, err)
	}

	return tx.Commit()
}

// CardWithSource extends the Card model to include the path of its source.
type CardWithSource struct {
	Hash       string
	Question   string
	Answer     string
	Stability  float64
	Difficulty float64
	DueDate    time.Time
	LastReview sql.NullTime
	State      int
	SourceID   sql.NullInt64
	SourcePath sql.NullString
}

// GetAllCardsSortedByDueDate retrieves all cards from the database, sorted by due date.
func (db *DB) GetAllCardsSortedByDueDate() ([]CardWithSource, error) {
	rows, err := db.conn.Query(`
		SELECT c.hash, c.question, c.answer, c.stability, c.difficulty, c.due_date, c.last_review, c.state, c.source_id, s.path
		FROM cards c
		LEFT JOIN sources s ON c.source_id = s.id
		ORDER BY c.due_date ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all cards sorted by due date: %w", err)
	}
	defer rows.Close()

	var cards []CardWithSource
	for rows.Next() {
		var cs CardWithSource
		if err := rows.Scan(
			&cs.Hash,
			&cs.Question,
			&cs.Answer,
			&cs.Stability,
			&cs.Difficulty,
			&cs.DueDate,
			&cs.LastReview,
			&cs.State,
			&cs.SourceID,
			&cs.SourcePath,
		); err != nil {
			return nil, fmt.Errorf("failed to scan card row: %w", err)
		}
		cards = append(cards, cs)
	}
	return cards, nil
}
