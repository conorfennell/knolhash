# KnolHash Development Plan

### 🛠 Milestone 1: The "Knol" Foundations (Core Logic)
**Goal:** Transform Markdown files into unique, hashed data structures.
- **Project Init:** Create `go.mod` and define the folder structure (`cmd`, `internal`, `web`).
- **Define the Schema:** Create the `Card` and `ReviewLog` structs.
- **The Parser:** Write a scanner to find `Q:`, `A:`, and `C:` blocks. Handle multi-line answers.
- **The KnolHasher:** Implement the normalization pipeline (lowercase, trim whitespace, normalize line endings) before SHA-256 hashing.
- **CLI Test:** Create a simple command that points at a folder and prints: `Found 50 cards, 0 errors.`

### 🗄 Milestone 2: Persistence (The Database)
**Goal:** Link the ephemeral Markdown hashes to long-term memory progress.
- **SQLite Setup:** Integrate a CGO-free driver (like `modernc.org/sqlite`).
- **Table Design:**
    - `cards`: Stores the KnolHash, current FSRS state (stability, difficulty), and due date.
    - `sources`: Stores paths to local folders or Git URLs.
- **The Reconciler:** Write the logic that runs on startup:
    - If a hash is in the file but not the DB: Insert it.
    - If a hash is in the DB but not the file: Mark it as "orphaned" or delete it.

### 🧠 Milestone 3: The Brain (FSRS Algorithm)
**Goal:** Determine exactly when you need to see a card again.
- **Port FSRS:** Implement the core FSRS math. The core formula for new stability S after a successful review is:
  `$$S'(S, R, D) = S \cdot (1 + a \cdot D^{-b} \cdot S^c \cdot (e^{d \cdot (1-R)} - 1))$$`
  (Note: You can use simplified constants for the first version).
- **The Scheduler:** Write a function `GetNextDue(card)` that returns a list of hashes sorted by their due date.

### 🌐 Milestone 4: The Web Bridge (Backend API)
**Goal:** Make the data accessible over a network.
- **HTTP Server:** Set up `net/http` or `chi`.
- **Endpoints:**
    - `GET /api/next`: Returns the next due card.
    - `POST /api/review`: Accepts the card ID and the grade (1-4) and updates the DB.
    - `POST /api/sync`: Manual trigger to re-scan the Markdown files.
- **Static Embedding:** Use `//go:embed` to include your HTML/CSS/JS inside the Go binary.

### 📱 Milestone 5: The Interface (Mobile-Friendly UI)
**Goal:** A "thumb-friendly" study experience using server-rendered HTML.
- **Tech Choice:** Use **HTMX** to handle interactivity by fetching HTML fragments from the Go backend. This avoids a complex JS framework.
- **The "Deck" View:** A simple counter showing "Due Today: X".
- **The "Study" View:**
    - Card front (Question).
    - "Show Answer" button.
    - Card back (Answer) + 4 rating buttons (Again, Hard, Good, Easy).
- **Responsive Design:** Use a simple CSS framework (like Pico.css or Tailwind CSS) to ensure the buttons are large and usable on mobile.
- **PWA Manifest:** Add a `manifest.json` so you can "Install" the site on your phone and hide the browser address bar.

### ☁️ Milestone 6: Remote & Git Integration
**Goal:** Point at public repos and sync from anywhere.
- **Git Consumer:** Integrate `go-git` to clone/pull from public URLs.
- **Background Worker:** Set up a "Ticker" in Go that pulls from all Git sources every 30 minutes.
- **Source Manager UI:** A screen to add/remove local paths or GitHub URLs.

### 🔒 Milestone 7: Hardening & Deployment
**Goal:** Host it securely on your machine for remote access.
- **Simple Auth:** Implement a "Secret Key" or basic password login via Middleware.
- **Dockerization:** Create a `Dockerfile` that bundles the Go app and provides a volume for the SQLite DB and Markdown files.
- **Reverse Proxy Setup:** Instructions for using Caddy to get an automatic HTTPS certificate (e.g., `https://knolhash.yourdomain.com`).
