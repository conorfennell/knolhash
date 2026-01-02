# KnolHash Project Overview for Gemini

This document provides a concise overview of the KnolHash project, its architecture, technology stack, and core functionalities, intended to quickly orient Gemini for efficient interaction and task execution.

## 1. Project Goal & Purpose

KnolHash is a learning tool designed to transform Markdown files into "knols" (knowledge units) represented as flashcards. It uses a spaced repetition system (FSRS algorithm) to schedule card reviews, helping users retain information effectively. The application supports both local Markdown files and Git repositories as sources for these knols, offering a web-based interface for review and management.

## 2. Technology Stack

*   **Backend:** Go (Golang)
    *   **Web Framework:** `net/http` for routing and handling.
    *   **Database:** SQLite (`modernc.org/sqlite`) for persistence.
    *   **Git Integration:** `go-git` for cloning and pulling Git repositories.
    *   **Logging:** `log/slog` for structured logging.
*   **Frontend:** HTML, CSS, JavaScript
    *   **Interactivity:** HTMX for dynamic content loading and partial page updates.
    *   **Styling:** Pico.css for a minimalist, responsive design.
    *   **Asset Embedding:** `//go:embed` for embedding static and template files directly into the Go binary.
*   **Containerization:** Docker (`Dockerfile`, `docker-compose.yml`) for easy deployment and local development.
*   **Reverse Proxy:** Caddy (`Caddyfile.local`, `Caddyfile.prod`) for serving the application, handling HTTPS, and acting as a reverse proxy.

## 3. Project Structure (High-Level)

The repository follows a standard Go project layout:

*   **`.github/workflows/`**: Contains GitHub Actions for CI/CD (e.g., `build.yml`).
*   **`cmd/knolhash/`**: The main entry point for the application. Handles CLI arguments and starts the web server or sync process.
*   **`internal/`**: Contains the core logic and internal packages, not intended for public consumption.
    *   **`domain/`**: Defines core data structures like `Card` and `ReviewLog`.
    *   **`fsrs/`**: Implements the FSRS (Free Spaced Repetition Scheduler) algorithm for spaced repetition.
    *   **`gitsource/`**: Handles interactions with Git repositories (cloning, pulling).
    *   **`knol/`**: Contains the `KnolHasher` for normalizing and hashing Markdown content.
    *   **`parser/`**: Parses Markdown files to extract Q&A cards.
    *   **`storage/`**: Manages SQLite database interactions (schema, CRUD operations for cards and sources).
    *   **`sync/`**: Orchestrates the synchronization process, reconciling file system changes with the database.
    *   **`web/`**: Contains the HTTP server implementation, HTML templates, static assets (HTMX, Pico.css, manifest.json), and handlers for web routes.
*   **`examples/`**: Sample Markdown files (`complex.md`, `empty.md`, `go.md`) to demonstrate card parsing.
*   **`Caddyfile*`**: Caddy server configuration files for local and production environments.
*   **`docker-compose*`**: Docker Compose configurations for orchestrating services.

## 4. Core Components & Functionality Flow

### a. Knol Generation and Hashing (`knol/`, `parser/`)
Markdown files with `Q:`, `A:`, and optionally `C:` (context) blocks are parsed. Each card's content is then normalized (lowercased, trimmed, standardized line endings) and hashed using SHA-256 to create a unique `KnolHash`.

### b. Data Persistence (`storage/`)
A SQLite database (`knolhash.db`) stores `cards` (KnolHash, FSRS state, due date) and `sources` (paths to local folders or Git URLs). The `storage` package provides an API for database operations.

### c. Synchronization (`sync/`, `gitsource/`)
The `sync` package is responsible for:
*   Scanning registered `sources` (local directories or Git repositories).
*   For Git sources, it uses `gitsource` to clone or pull the latest changes.
*   Reconciling discovered cards with the database:
    *   New cards are inserted.
    *   Cards present in the database but no longer in the source are marked as "orphaned" and deleted.
    *   Source metadata (like last scanned time) is updated.
This sync process can be triggered manually or run as a background task in server mode.

### d. Spaced Repetition (FSRS Algorithm - `fsrs/`)
The `fsrs` package implements the FSRS algorithm, which calculates a card's `stability` and `difficulty` based on user reviews (grades 1-4). This determines the `next due date` for a card, optimizing the review schedule.

### e. Web Interface (`web/`)
The `web` package provides an HTTP server with HTMX-driven routes:
*   **`/deck`**: Shows the number of cards due for review.
*   **`/review/next`**: Displays the front of the next due card.
*   **`/review/answer/{hash}`**: Displays the back of a card and provides grading options.
*   **`/review/{hash}`**: Processes the user's review grade and updates the card's FSRS state and due date in the database.
*   **`/sources`**: Allows users to manage (add, delete) local or Git sources.
*   **`/sync`**: Manually triggers a synchronization process.
Static assets (CSS, JS, PWA manifest) are embedded.

## 5. Development Status (from `TODO.md` and `README.md`)

The project is largely complete through Milestone 6 (Remote & Git Integration). Key features implemented include:
*   Markdown parsing and KnolHashing.
*   SQLite persistence with schema and CRUD operations.
*   FSRS algorithm integration for spaced repetition.
*   Web-based UI using HTMX for card review and source management.
*   Git repository synchronization (clone/pull).
*   Background sync worker.
*   Dockerization for deployment.

Milestone 7 (Hardening & Deployment) has Dockerization and Reverse Proxy setup completed, but "Simple Auth" is currently skipped.

## 6. Important Files & Configuration

*   **`go.mod`**: Go module definition and dependencies.
*   **`Dockerfile`**: Defines the Docker image for the application.
*   **`docker-compose.yml`**: Orchestrates the Go application and a Caddy server.
*   **`Caddyfile.local` / `Caddyfile.prod`**: Caddy server configurations. Note: Caddy's `log` directive in these files is for generating access logs for the reverse proxy itself. These logs are distinct from the application's internal logs and are important for monitoring HTTP traffic at the Caddy layer.
*   **`knolhash.db`**: The SQLite database file (ignored by Git).
*   **`repos/`**: Directory where Git repositories are cloned (ignored by Git).
*   **`.gitignore`**: Specifies files and directories to be ignored by Git.

This overview should provide a solid foundation for understanding the KnolHash project.