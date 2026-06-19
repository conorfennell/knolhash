# knolhash — Claude context

## What this project is

A **Go web app** that serves two things:
1. A flashcard/spaced-repetition system (the original product — not touched recently)
2. A **World Cup 2026 sweepstake leaderboard** at `/worldcup` — the active work

The sweepstake has 18 entries (real people), each with 4 national teams. Points are scored golf-style (lower = better). Data is scraped from Wikipedia every 15 minutes.

## Running locally

```bash
go run ./cmd/knolhash
# → http://localhost:8080/worldcup
```

Requires `knolhash.db` (SQLite). The worldcup route works without it since data comes from Wikipedia, not the DB.

## World Cup scoring rules

Golf scoring — lower total points wins. See `worldcup-rules.md` for the full table.

| Placement | Points |
|---|---|
| Champion | 1 |
| Runner-up | 2 |
| 3rd place | 3 |
| 4th place | 4 |
| QF loser | 6 |
| R16 loser | 12 |
| R32 loser | 25 |
| 3rd-place group rank 9–12 (eliminated) | 33–36 |
| 4th in group | 42 |

3rd-place teams that advance to the R32 get a bonus: `PlacementPoints + ThirdPlaceGroupBonus(rank)` where rank 1–8 adds 1–8 pts.

**While teams are still active**, estimated points are used as a floor:
- 1st/2nd/3rd in group → ~25pts
- 4th in group → ~42pts

`~` prefix means provisional/estimated throughout the UI.

**Provisional vs definitive elimination:**
- A team is `Provisional=true` when the 3rd-place ranking table has fewer than 12 resolved teams (not all groups done). These show `~Out` and `~Xpts`.
- Fully eliminated teams (FinalPlace set, Provisional=false) show `Out` and no `~`.

## Key files

| File | Purpose |
|---|---|
| `internal/worldcup/entries.go` | All 18 entries, `TeamState` struct, team name mapping |
| `internal/worldcup/scraper.go` | Wikipedia scraper — group tables, 3rd-place rankings, footballbox match parser |
| `internal/worldcup/scoring.go` | `ScoreEntry`, `ScoreAll`, `PlacementPoints`, `EstimatedGroupPoints`, prize calcs |
| `internal/web/worldcup.go` | Main leaderboard handler + entry page handler (`/worldcup/entry/<slug>`) |
| `internal/web/server.go` | Template funcs: `teamClass`, `flagClass`, `teamPosLabel`, `teamPts`, `entrySlug`, `isEntryTeam`, match entry helpers |
| `internal/web/templates/worldcup.html` | Main leaderboard page |
| `internal/web/templates/worldcup_entry.html` | Personal entry page |
| `cmd/knolhash/main.go` | Entry point — has `_ "time/tzdata"` import for Alpine Docker timezone fix |
| `worldcup-rules.md` | Human-readable rules doc for the sweepstake |

## What's been built (current state as of 14 Jun 2026)

### Main leaderboard (`/worldcup`)
- Hero banner with pot size (€90)
- Upcoming + recent match cards with kickoff times in Irish time (Europe/Dublin)
- 4 prize tables: Least Points, Most Points, Most Goals, World Cup Winner
- Per-entry breakdown grid (18 cards, 2-col) — each shows all 4 teams with pos/goals/pts
- Flag emoji treatment: half-greyscale for provisional out, full greyscale for definitive out (no strikethrough)
- Entry names are links to personal pages

### Personal entry page (`/worldcup/entry/<slug>`)
- URL slug = lowercase + hyphens (e.g. `kevin-2`, `paul-football-friends`)
- Hero: rank of 18, total pts, goals
- 2×2 team cards (compact: pos / goals / pts)
- Unified upcoming fixtures list in chronological order — entry's teams shown in bold dark
- Prize standings table with rank and gap to leader in each category

### Scraper
- Parses Wikipedia's group wikitables for standings
- Parses `div.footballbox` elements for match data + kickoff times with UTC offsets
- Parses "Ranking of third-placed teams" wikitable (identified by "Grp" column)
- Sets `Provisional=true` on 3rd-place ranked teams when fewer than 12 groups have finished

### Known behaviour
- Teams with `ThirdPlaceGroupRank > 0` and `FinalPlace == 0` (advancing 3rd-place teams still in tournament) use `EstimatedGroupPoints` (25pts) as their floor — the rank bonus only applies post-elimination
- 4th-place teams with `Played == 3` get `FinalPlace = 40` (representative 37–48 value, scores 42pts)
- Prizes: Least Points = €5 + 50% variable pool; Overall Winner = 50% variable pool; Most Points = €5; Most Goals = €5

## Docker / deployment

Uses Alpine + `Dockerfile`. The `_ "time/tzdata"` blank import in `main.go` embeds the timezone DB — without it, `time.LoadLocation("Europe/Dublin")` silently returns UTC on Alpine.

## Prizes (20 entries × €5 = €100 pot)
- **Least Points** (best): €5 + 42.50 = €47.50
- **Overall Winner** (champion's entry): €42.50
- **Most Points** (worst): €5
- **Most Goals**: €5
