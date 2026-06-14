# Trusted Sources — Vision & Build Plan

## What it is

A personal "trusted sources" directory at `knolhash.com/trusted/cfennell`. Each person publishes the sources they actually trust, organised by topic. The value: "I trust Conor's judgment on X — show me what he reads."

This is not a social feed or aggregator. The unit of trust is the **source**, not the person. People can browse someone else's list and borrow sources into their own.

## Decisions made (14 Jun 2026)

| Decision | Choice | Reasoning |
|---|---|---|
| Route | `/trusted/:handle` | Each person owns their own page |
| First page | `/trusted/cfennell` | Start with just Conor's list |
| Approach | Static — sources hardcoded in Go | Ship fast, validate format before adding auth/DB |
| Data model | topic + source type + URL | Simple, enough to be useful |
| Taxonomy | Free-form topics, no enforcement | Each person organises their own way; no cross-person browsing yet |
| Multi-user | Deferred — add second person when ready | `/trusted/[handle]` pattern already supports it |
| Adopt mechanic | Deferred | Not needed until there's a second person to adopt from |

## Data model

```go
type Source struct {
    Label string // display name
    URL   string
    Type  string // "youtube-channel", "newsletter", "newspaper", "journalist", "twitter", "podcast", "research", etc.
}

type TopicSources struct {
    Topic   string
    Sources []Source
}

type PersonSources struct {
    Handle string
    Name   string
    Topics []TopicSources
}
```

## cfennell's seed sources

### Clean Energy
- Just Have a Think — https://www.youtube.com/@JustHaveaThink (youtube-channel)
- BloombergNEF — https://about.bnef.com/ (research)

### Ukraine
- *(add when building)*

## Build plan

### Phase 1 — Static MVP
- [ ] Create `internal/web/trusted.go` — `PersonSources` struct + hardcoded cfennell data + handler
- [ ] Create `internal/web/templates/trusted.html` — topic groups, source type label, link
- [ ] Register `GET /trusted/{handle}` route in `internal/web/server.go`
- [ ] Test locally at `http://localhost:8080/trusted/cfennell`
- [ ] Deploy and share link

### Phase 2 — Second person
- [ ] Add second person's sources as another hardcoded `PersonSources` entry
- [ ] Verify `/trusted/[handle]` renders correctly for them
- [ ] Optional: `/trusted` index page listing all people

### Phase 3 — Adopt mechanic (deferred)
- [ ] Design the copy-source flow — visitor clicks "add to my list" on a source
- [ ] Needs identity/session — revisit once static has been validated

## Tech pattern

Follow the worldcup precedent exactly:
- `internal/web/trusted.go` — handler + static data (mirrors `worldcup.go`)
- `internal/web/templates/trusted.html` — Pico.css, keep it clean and mobile-readable
- Route registered in `server.go`

## Open items to watch

- **No "why I trust this" field yet** — may want a one-liner annotation per source
- **Mobile first** — visitors will open the shared link on their phone; template must be responsive
- **Interview onboarding flow** — parked; revisit if manually editing Go structs feels painful
- **Cross-person topic browsing** — good long-term direction; needs controlled vocab first
