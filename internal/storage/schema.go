package storage

const schema = `
-- The 'cards' table stores the core information about each flashcard.
CREATE TABLE IF NOT EXISTS cards (
    hash TEXT PRIMARY KEY,
    question TEXT NOT NULL,
    stability REAL,
    difficulty REAL,
    due_date DATETIME NOT NULL,
    last_review DATETIME,
    state INTEGER DEFAULT 0, -- 0: New, 1: Learning, 2: Review
    source_id INTEGER,
    
    FOREIGN KEY(source_id) REFERENCES sources(id)
);

-- The 'sources' table tracks the origin of the cards, either a local directory or a git repository.
CREATE TABLE IF NOT EXISTS sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    last_scanned DATETIME
);
`
