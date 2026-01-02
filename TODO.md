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

## Milestone 8: LaTeX & Enhanced Markdown Rendering

**Goal:** Allow users to include LaTeX math and use a richer set of Markdown features within their flashcards, rendered correctly in the web UI.

### 8.1 Backend Enhancements (Markdown Parsing & Pre-rendering)

- [ ] **Integrate Robust Markdown-to-HTML Renderer:**
    - [ ] Add `github.com/gomarkdown/markdown` dependency.
    - [ ] Modify `internal/parser/parser.go`:
        - [ ] Update `domain.Card` fields (Question, Answer, Context) to `template.HTML` type.
        - [ ] Convert extracted Markdown content to HTML using `gomarkdown`.
        - [ ] Configure `gomarkdown` to recognize LaTeX blocks (e.g., `$...$` for inline, `$$...$$` for display) and wrap them in specific HTML tags (e.g., `<span class="latex-math">...</span>`, `<div class="latex-display">...</div>`).
        - [ ] (Optional) Enable `gomarkdown` extensions (tables, footnotes, etc.) if desired.
- [ ] **Implement Image Path Resolution:**
    - [ ] Enhance `internal/parser` to process Markdown image links (`![](path/to/img.png)`).
    - [ ] Implement logic to resolve image paths:
        - [ ] Relative to the deck's location.
        - [ ] Relative to the collection root (for `@/` prefixed paths).
    - [ ] Generate unique, web-accessible URLs for these images (e.g., `/assets/{sourceID}/{deckPathHash}/{imageName}`).

### 8.2 Frontend Enhancements (LaTeX & Rich HTML Display)

- [ ] **Include KaTeX Library:**
    - [ ] Download `katex.min.css`, `katex.min.js`, and `contrib/auto-render.min.js` into `internal/web/static`.
- [ ] **Modify Base HTML Template (`internal/web/static/index.html` or equivalent):**
    - [ ] Link `katex.min.css` in the `<head>`.
    - [ ] Include `katex.min.js` and `auto-render.min.js` scripts, ensuring `auto-render` is called on page load.
    - [ ] Add an HTMX event listener (`htmx:afterSwap`) to re-trigger KaTeX rendering on new content loaded via HTMX.
- [ ] **Update Card Display Templates (`internal/web/templates/card_front.html`, `card_back.html`):**
    - [ ] Ensure `{{ .Question }}`, `{{ .Answer }}`, `{{ .Context }}` are used directly (without `html` template function if already `template.HTML`) to render the pre-rendered HTML content.

### 8.3 Web Server Asset Serving

- [ ] **Implement Image Asset Handler:**
    - [ ] Create a new HTTP handler in `internal/web/server.go` (e.g., `handleImageAssets()`).
    - [ ] This handler should process requests to `/assets/{sourceID}/{deckPathHash}/{imageName}`.
    - [ ] Implement strict validation and canonicalization of paths to prevent directory traversal vulnerabilities.
    - [ ] Serve the actual image files from the resolved disk locations.

### 8.4 Testing

- [ ] **Unit Tests for Markdown Parsing:** Verify correct Markdown-to-HTML conversion and LaTeX block detection.
- [ ] **Integration Tests for Web UI:** Confirm KaTeX rendering and image display in the browser.
- [ ] **Security Testing:** Ensure image asset handler is secure against path traversal.
