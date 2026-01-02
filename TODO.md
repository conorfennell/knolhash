# Project Todos

## Milestone 1: The "Knol" Foundations (Core Logic)
- [x] Initialize Go module and create project structure
- [x] Define Card and ReviewLog structs
- [x] Implement the Markdown parser
- [x] Implement the KnolHasher
- [x] Create the CLI entrypoint

## Milestone 2: Persistence (The Database)
- [x] **Integrate SQLite driver:** Add `modernc.org/sqlite`.
- [x] **Design and create database schema:**
    - [x] Create a `storage` package.
    - [x] Define SQL for `cards` and `sources` tables.
- [x] **Implement database interaction layer:**
    - [x] Write functions to `Open` the database.
    - [x] Write `Find`, `Insert`, `Update` methods for cards.
    - [x] Write `Find`, `Insert`, `GetAll`, `Update` methods for sources.
- [x] **Implement the reconciler logic:**
    - [x] If a hash is in the file but not the DB: Insert it.
    - [x] If a hash is in the DB but not the file: Mark it as "orphaned" and delete it.

## Milestone 3: The Brain (FSRS Algorithm)
- [x] **Port FSRS:** 
    - [x] Create `internal/fsrs` package.
    - [x] Implement the core FSRS math based on the simplified formula.
    - [x] Define default parameters for `a, b, c, d` and retention.
- [x] **The Scheduler:** 
    - [x] Write a function `GetDueCards()` that returns a list of hashes sorted by their due date.

## Milestone 4: The Web Bridge (Backend API)
- [x] **HTTP Server:** 
    - [x] Add `net/http` router dependency.
    - [x] Set up `net/http` server in `internal/web` package.
    - [x] Define routes for the API.
- [x] **Endpoints:**
    - [x] `GET /api/next`: Returns the next due card.
    - [x] `POST /api/review`: Accepts the card ID and the grade (1-4) and updates the DB.
    - [x] `POST /api/sync`: Manual trigger to re-scan the Markdown files.
- [x] **Static Embedding:** 
    - [x] Use `//go:embed` to include your HTML/CSS/JS inside the Go binary.

## Milestone 5: The Interface (Mobile-Friendly UI)
- [x] **Integrate HTMX and Pico.css:**
    - [x] Download HTMX and Pico.css to `internal/web/static`.
    - [x] Refactor `index.html` to use HTMX attributes.
- [x] **Implement Deck and Study Views:**
    - [x] Create Go templates for HTML fragments.
    - [x] `GET /deck`: Renders the deck view (due card count).
    - [x] `GET /review/next`: Renders the front of the next card.
    - [x] `GET /review/answer/{hash}`: Renders the back of the card.
    - [x] `POST /review/{hash}`: Accepts a grade and renders the next card.
- [x] **PWA Manifest:** 
    - [x] Add a `manifest.json` to `internal/web/static`.

## Milestone 6: Remote & Git Integration
- [x] **Git Consumer:**
    - [x] Add `go-git` dependency.
    - [x] Create `internal/gitsource` package.
    - [x] Implement logic to clone or pull public Git repositories.
- [x] **Refactor Source Handling:**
    - [x] Add `type` column to `sources` table (`local` vs `git`).
    - [x] Update reconciliation logic to handle different source types.
- [x] **Background Worker:** 
    - [x] Set up a "Ticker" in Go that pulls from all Git sources every 30 minutes.
- [x] **Source Manager UI:** 
    - [x] A screen to add/remove local paths or GitHub URLs.

## Milestone 7: Hardening & Deployment
- [x] **Simple Auth:** *Skipped for now*
- [x] **Dockerization:**
    - [x] Create a `Dockerfile` for the Go application.
    - [x] Create a `docker-compose.yml` for easy execution.
- [x] **Reverse Proxy Setup:**
    - [x] Create `CADDY_SETUP.md` with instructions for using Caddy.
    - [x] Create `docker-compose.override.yml` and `docker-compose.prod.yml`.