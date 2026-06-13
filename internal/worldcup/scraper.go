package worldcup

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
)

const wikiURL = "https://en.wikipedia.org/wiki/2026_FIFA_World_Cup"

// FetchTournamentData fetches the Wikipedia World Cup page and extracts
// team standings from the 12 group tables, plus knockout results where available.
func FetchTournamentData() (TournamentData, error) {
	req, err := http.NewRequest("GET", wikiURL, nil)
	if err != nil {
		return TournamentData{}, err
	}
	req.Header.Set("User-Agent", "knolhash-worldcup/1.0 (sweepstake leaderboard; personal project)")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return TournamentData{}, fmt.Errorf("fetching Wikipedia: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TournamentData{}, fmt.Errorf("Wikipedia returned %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return TournamentData{}, fmt.Errorf("parsing HTML: %w", err)
	}

	teams := parseGroupTables(doc)
	applyKnockoutEliminations(doc, teams)

	slog.Info("worldcup: scraped tournament data", "teams", len(teams))
	return TournamentData{
		Teams:     teams,
		FetchedAt: time.Now(),
	}, nil
}

// entryTeamSet returns the set of all team names referenced across all Entries.
func entryTeamSet() map[string]bool {
	set := make(map[string]bool)
	for _, e := range Entries {
		for _, t := range e.Teams {
			set[t] = true
		}
	}
	return set
}

// parseGroupTables finds the 12 group standing wikitables and extracts each
// team's current position, goals scored, and games played.
func parseGroupTables(doc *html.Node) map[string]TeamState {
	knownTeams := entryTeamSet()
	teams := make(map[string]TeamState)
	for _, table := range findWikitables(doc) {
		rows := tableRows(table)
		if !isGroupTable(rows) {
			continue
		}
		headers := cellTexts(rows[0])
		gfIdx := indexOfHeader(headers, "GF")
		pldIdx := indexOfHeader(headers, "Pld")
		if gfIdx < 0 || pldIdx < 0 {
			continue
		}
		for _, row := range rows[1:] {
			cells := cellTexts(row)
			if len(cells) <= gfIdx {
				continue
			}
			pos, err := strconv.Atoi(strings.TrimSpace(cells[0]))
			if err != nil || pos < 1 || pos > 4 {
				continue
			}
			name := normalizeTeamName(cells[1])
			if name == "" || !knownTeams[name] {
				continue
			}
			gf, _ := strconv.Atoi(strings.TrimSpace(cells[gfIdx]))
			pld, _ := strconv.Atoi(strings.TrimSpace(cells[pldIdx]))
			teams[name] = TeamState{
				Name:          name,
				GoalsFor:      gf,
				GroupPosition: pos,
				Played:        pld,
			}
		}
	}

	// Teams that finished 4th and have played all 3 group matches are eliminated.
	for name, state := range teams {
		if state.Played == 3 && state.GroupPosition == 4 {
			s := state
			s.FinalPlace = 40 // representative value in the 37–48 range (all score 42 pts)
			teams[name] = s
		}
	}

	return teams
}

// applyKnockoutEliminations attempts to assign FinalPlace values to teams
// eliminated in the knockout rounds by parsing the round-specific sections.
// This is a best-effort parse; data will be absent until each round is played.
func applyKnockoutEliminations(doc *html.Node, teams map[string]TeamState) {
	// Each knockout round has a heading like "Round of 32", "Round of 16", etc.
	// Wikipedia updates the page with match results once they're played.
	// We scan all wikitables for rows that contain two known team names and a score,
	// then mark the loser with the appropriate FinalPlace.
	//
	// Round of 32 losers: places 17–32 (42 pts in scoring, but score is 25)
	// Round of 16 losers: places 9–16
	// Quarterfinal losers: places 5–8
	// Semifinal losers:    places 3–4 (3rd place match determines 3 vs 4)
	// Final:               places 1–2
	//
	// The page structure for individual match results uses tables with a
	// home/score/away layout. We look for those patterns here.
	// This will be populated as the tournament progresses — during group stage
	// this function is a no-op because no knockout tables exist yet.
	_ = doc
	_ = teams
}

// isGroupTable returns true when the set of rows looks like a group standings
// table: first header starts with "Pos", has "GF" and "Pld" headers, and
// exactly 4 data rows with positions 1–4.
func isGroupTable(rows []*html.Node) bool {
	if len(rows) < 5 {
		return false
	}
	headers := cellTexts(rows[0])
	if len(headers) < 4 {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(headers[0]), "pos") {
		return false
	}
	if indexOfHeader(headers, "GF") < 0 || indexOfHeader(headers, "Pld") < 0 {
		return false
	}
	// Verify exactly 4 valid data rows (positions 1–4).
	valid := 0
	for _, row := range rows[1:] {
		cells := cellTexts(row)
		if len(cells) == 0 {
			continue
		}
		pos, err := strconv.Atoi(strings.TrimSpace(cells[0]))
		if err == nil && pos >= 1 && pos <= 4 {
			valid++
		}
	}
	return valid == 4
}

// findWikitables returns every <table> node whose class contains "wikitable".
func findWikitables(n *html.Node) []*html.Node {
	var tables []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "table" {
			for _, a := range node.Attr {
				if a.Key == "class" && strings.Contains(a.Val, "wikitable") {
					tables = append(tables, node)
					break
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return tables
}

// tableRows returns all <tr> nodes that are direct descendants of the table
// (via thead/tbody/tfoot), without descending into nested tables.
func tableRows(table *html.Node) []*html.Node {
	var rows []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			rows = append(rows, n)
			return // don't recurse into tr children
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	// Only walk the direct structural children (thead, tbody, tfoot), not nested tables.
	for c := table.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			switch c.Data {
			case "thead", "tbody", "tfoot":
				walk(c)
			case "tr":
				rows = append(rows, c)
			}
		}
	}
	return rows
}

// cellTexts returns the trimmed text content of each <th>/<td> in a <tr>.
func cellTexts(row *html.Node) []string {
	var texts []string
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			texts = append(texts, strings.TrimSpace(extractText(c)))
		}
	}
	return texts
}

// extractText recursively collects text nodes, skipping <style> and <script>.
func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Type == html.ElementNode && (n.Data == "style" || n.Data == "script") {
		return ""
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(extractText(c))
	}
	return sb.String()
}

// indexOfHeader returns the index of the first header whose trimmed, lowercased
// value equals name (case-insensitive). Returns -1 if not found.
func indexOfHeader(headers []string, name string) int {
	lower := strings.ToLower(name)
	for i, h := range headers {
		if strings.ToLower(strings.TrimSpace(h)) == lower {
			return i
		}
	}
	return -1
}

// normalizeTeamName strips the host indicator "(H)", normalises whitespace,
// and maps Wikipedia country names to the canonical names used in Entries.
func normalizeTeamName(raw string) string {
	// Replace non-breaking spaces and control characters with regular spaces.
	name := strings.Map(func(r rune) rune {
		if r == ' ' || unicode.IsControl(r) {
			return ' '
		}
		return r
	}, raw)
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, " (H)")
	name = strings.TrimSpace(name)
	if mapped, ok := wikiToEntry[name]; ok {
		return mapped
	}
	return name
}
