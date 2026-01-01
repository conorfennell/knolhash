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
- [ ] HTTP Server: Set up net/http or chi.
- [ ] Endpoints:
    - [ ] GET /api/next: Returns the next due card.
    - [ ] POST /api/review: Accepts the card ID and the grade (1-4) and updates the DB.
    - [ ] POST /api/sync: Manual trigger to re-scan the Markdown files.
- [ ] Static Embedding: Use //go:embed to include your HTML/CSS/JS inside the Go binary.

## Milestone 5: The Interface (Mobile-Friendly UI)
- [ ] Tech Choice: Use HTMX to handle interactivity by fetching HTML fragments from the Go backend.
- [ ] The "Deck" View: A simple counter showing "Due Today: X".
- [ ] The "Study" View:
    - [ ] Card front (Question).
    - [ ] "Show Answer" button.
    - [ ] Card back (Answer) + 4 rating buttons (Again, Hard, Good, Easy).
- [ ] Responsive Design: Use a simple CSS framework (like Pico.css or Tailwind CSS) to ensure the buttons are large and usable on mobile.
- [ ] PWA Manifest: Add a `manifest.json` so you can "Install" the site on your phone and hide the browser address bar.

## Milestone 6: Remote & Git Integration
- [ ] Git Consumer: Integrate go-git to clone/pull from public URLs.
- [ ] Background Worker: Set up a "Ticker" in Go that pulls from all Git sources every 30 minutes.
- [ ] Source Manager UI: A screen to add/remove local paths or GitHub URLs.

## Milestone 7: Hardening & Deployment
- [ ] Simple Auth: Implement a "Secret Key" or basic password login via Middleware.
- [ ] Dockerization: Create a Dockerfile that bundles the Go app and provides a volume for the SQLite DB and Markdown files.
- [ ] Reverse Proxy Setup: Instructions for using Caddy to get an automatic HTTPS certificate (e.g., https://knolhash.yourdomain.com).