package worldcup

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
)

// scoreRe matches "N–N", "N-N", "N − N" (various dash/minus forms).
var scoreRe = regexp.MustCompile(`(\d+)\s*[–\-−]\s*(\d+)`)

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

	knownTeams := entryTeamSet()
	teams := parseGroupTables(doc)
	// Primary: footballbox divs (used by 2026 WC page); fallback: wikitables.
	matches := parseFootballBoxes(doc, knownTeams)
	if len(matches) == 0 {
		matches = parseMatches(doc, knownTeams)
	}
	applyThirdPlaceRankings(doc, teams)
	applyKnockoutEliminations(doc, teams)

	slog.Info("worldcup: scraped tournament data", "teams", len(teams), "matches", len(matches))
	return TournamentData{
		Teams:     teams,
		Matches:   matches,
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

// applyThirdPlaceRankings parses the "Ranking of third-placed teams" wikitable
// and updates team state based on their rank among the 12 third-placed teams:
//   - ranks 1–8  (advancing to knockouts): ThirdPlaceGroupRank = rank
//   - ranks 9–12 (eliminated):             FinalPlace = rank+24  (→ positions 33–36, scoring 33–36 pts)
//
// The table is identified by having a "Grp" column, which group tables lack.
// Rows where the team name cannot be resolved to a known entry team are skipped
// (e.g. "Third place Group E" when that group hasn't finished yet).
func applyThirdPlaceRankings(doc *html.Node, teams map[string]TeamState) {
	knownTeams := entryTeamSet()
	for _, table := range findWikitables(doc) {
		rows := tableRows(table)
		if len(rows) < 2 {
			continue
		}
		headers := cellTexts(rows[0])
		if indexOfHeader(headers, "Grp") < 0 {
			continue
		}
		teamIdx := indexOfHeader(headers, "Team")
		if teamIdx < 0 {
			teamIdx = 2
		}
		for _, row := range rows[1:] {
			cells := cellTexts(row)
			if len(cells) <= teamIdx {
				continue
			}
			pos, err := strconv.Atoi(strings.TrimSpace(cells[0]))
			if err != nil || pos < 1 || pos > 12 {
				continue
			}
			name := normalizeTeamName(cells[teamIdx])
			if name == "" || !knownTeams[name] {
				continue
			}
			state := teams[name]
			if pos <= 8 {
				state.ThirdPlaceGroupRank = pos
				state.FinalPlace = 0
			} else {
				state.FinalPlace = pos + 24
				state.ThirdPlaceGroupRank = 0
			}
			teams[name] = state
		}
		break
	}
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

// cellInfo holds text and class for one table cell.
type cellInfo struct {
	text  string
	class string
}

// cellsInfo returns text + class for every <td>/<th> in a row.
func cellsInfo(row *html.Node) []cellInfo {
	var out []cellInfo
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || (c.Data != "td" && c.Data != "th") {
			continue
		}
		cls := ""
		for _, a := range c.Attr {
			if a.Key == "class" {
				cls = a.Val
				break
			}
		}
		out = append(out, cellInfo{
			text:  strings.TrimSpace(extractText(c)),
			class: cls,
		})
	}
	return out
}

// parseMatches scans all non-group wikitables for match rows and returns them
// sorted chronologically. Rows are identified by a score cell ("N–N") or
// Wikipedia's fscore CSS class; team names come from fhome/faway classes or
// the cells adjacent to the score. Only matches involving at least one entry
// team are returned.
func parseMatches(doc *html.Node, knownTeams map[string]bool) []Match {
	seen := make(map[string]bool)
	var matches []Match

	for _, table := range findWikitables(doc) {
		rows := tableRows(table)
		if isGroupTable(rows) {
			continue
		}

		var currentDate time.Time

		for _, row := range rows {
			cells := cellsInfo(row)

			// Single-cell rows are often date headers.
			if len(cells) == 1 {
				if t, ok := tryParseDate(cells[0].text); ok {
					currentDate = t
				}
				continue
			}

			// Find score cell: prefer class "fscore", fall back to regex.
			scoreIdx := -1
			for i, c := range cells {
				if strings.Contains(c.class, "fscore") {
					scoreIdx = i
					break
				}
			}
			if scoreIdx < 0 {
				for i, c := range cells {
					if scoreRe.MatchString(c.text) {
						scoreIdx = i
						break
					}
				}
			}
			if scoreIdx < 0 {
				continue
			}

			// Find team names: prefer fhome/faway classes, fall back to adjacent cells.
			homeText, awayText := "", ""
			for _, c := range cells {
				if strings.Contains(c.class, "fhome") && homeText == "" {
					homeText = normalizeTeamName(c.text)
				}
				if strings.Contains(c.class, "faway") && awayText == "" {
					awayText = normalizeTeamName(c.text)
				}
			}
			if homeText == "" && scoreIdx > 0 {
				homeText = normalizeTeamName(cells[scoreIdx-1].text)
			}
			if awayText == "" && scoreIdx < len(cells)-1 {
				awayText = normalizeTeamName(cells[scoreIdx+1].text)
			}

			// Skip if neither team is a sweepstake entry team.
			if !knownTeams[homeText] && !knownTeams[awayText] {
				continue
			}

			// Deduplicate (same match can appear in multiple tables).
			key := homeText + "|||" + awayText
			alt := awayText + "|||" + homeText
			if seen[key] || seen[alt] {
				continue
			}
			seen[key] = true

			m := Match{
				HomeTeam: homeText,
				AwayTeam: awayText,
				KickOff:  currentDate,
			}
			scoreText := cells[scoreIdx].text
			if sub := scoreRe.FindStringSubmatch(scoreText); sub != nil {
				m.HomeScore, _ = strconv.Atoi(sub[1])
				m.AwayScore, _ = strconv.Atoi(sub[2])
				m.Played = true
			}

			matches = append(matches, m)
		}
	}

	// Sort chronologically; zero-time matches go to the end.
	sort.Slice(matches, func(i, j int) bool {
		a, b := matches[i].KickOff, matches[j].KickOff
		if a.IsZero() && b.IsZero() {
			return matches[i].HomeTeam < matches[j].HomeTeam
		}
		if a.IsZero() {
			return false
		}
		if b.IsZero() {
			return true
		}
		return a.Before(b)
	})

	return matches
}

// tryParseDate attempts to parse common Wikipedia date formats.
func tryParseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, f := range []string{"2 January 2006", "January 2, 2006", "2 Jan 2006", "Jan 2, 2006"} {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseFootballBoxes extracts matches from div.footballbox elements, which is
// how Wikipedia presents matches on the 2026 FIFA World Cup page.
func parseFootballBoxes(doc *html.Node, knownTeams map[string]bool) []Match {
	boxes := findNodesByClass(doc, "div", "footballbox")
	seen := make(map[string]bool)
	var matches []Match

	for _, box := range boxes {
		kickOff := extractKickOff(box)

		homeNode := findNodeByClass(box, "th", "fhome")
		awayNode := findNodeByClass(box, "th", "faway")
		scoreNode := findNodeByClass(box, "th", "fscore")
		if homeNode == nil || awayNode == nil || scoreNode == nil {
			continue
		}

		homeText := normalizeTeamName(firstAnchorText(homeNode))
		awayText := normalizeTeamName(firstAnchorText(awayNode))
		scoreText := strings.TrimSpace(extractText(scoreNode))

		if homeText == "" || awayText == "" {
			continue
		}
		if !knownTeams[homeText] && !knownTeams[awayText] {
			continue
		}

		key := homeText + "|||" + awayText
		if seen[key] || seen[awayText+"|||"+homeText] {
			continue
		}
		seen[key] = true

		m := Match{HomeTeam: homeText, AwayTeam: awayText, KickOff: kickOff}
		if sub := scoreRe.FindStringSubmatch(scoreText); sub != nil {
			m.HomeScore, _ = strconv.Atoi(sub[1])
			m.AwayScore, _ = strconv.Atoi(sub[2])
			m.Played = true
		}
		matches = append(matches, m)
	}

	sort.Slice(matches, func(i, j int) bool {
		a, b := matches[i].KickOff, matches[j].KickOff
		if a.IsZero() && b.IsZero() {
			return matches[i].HomeTeam < matches[j].HomeTeam
		}
		if a.IsZero() {
			return false
		}
		if b.IsZero() {
			return true
		}
		return a.Before(b)
	})
	return matches
}

// findNodesByClass returns all elements of the given tag whose class list
// contains class as a whole word. tag="" matches any element.
func findNodesByClass(root *html.Node, tag, class string) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (tag == "" || n.Data == tag) {
			for _, a := range n.Attr {
				if a.Key == "class" {
					for _, c := range strings.Fields(a.Val) {
						if c == class {
							out = append(out, n)
							break
						}
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

// findNodeByClass returns the first element matching tag + class.
func findNodeByClass(root *html.Node, tag, class string) *html.Node {
	nodes := findNodesByClass(root, tag, class)
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

// findNodeByClassSubstr returns the first element of tag whose class attribute
// contains classSubstr as a substring (for multi-value class chains).
func findNodeByClassSubstr(root *html.Node, tag, classSubstr string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && (tag == "" || n.Data == tag) {
			for _, a := range n.Attr {
				if a.Key == "class" && strings.Contains(a.Val, classSubstr) {
					found = n
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

// firstAnchorText returns the text of the first <a> element found within n.
func firstAnchorText(n *html.Node) string {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if found != nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == "a" {
			found = node
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	if found == nil {
		return ""
	}
	return strings.TrimSpace(extractText(found))
}

// extractKickOff builds a UTC time from the footballbox's dtstart span (date)
// and ftime div (local time + UTC offset). Returns zero time if parsing fails.
func extractKickOff(box *html.Node) time.Time {
	var date time.Time
	if ds := findNodeByClassSubstr(box, "span", "dtstart"); ds != nil {
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(extractText(ds))); err == nil {
			date = t
		}
	}
	if date.IsZero() {
		return time.Time{}
	}

	ftimeNode := findNodeByClass(box, "div", "ftime")
	if ftimeNode == nil {
		return date
	}

	// UTC offset from the anchor text, e.g. "UTC−6".
	offsetHours := 0
	if h, ok := parseUTCOffset(firstAnchorText(ftimeNode)); ok {
		offsetHours = h
	}

	// Time text: strip the "UTC…" portion from the ftime text.
	ftimeText := strings.TrimSpace(extractText(ftimeNode))
	if idx := strings.Index(ftimeText, "UTC"); idx >= 0 {
		ftimeText = strings.TrimSpace(ftimeText[:idx])
	}
	// Normalise non-breaking spaces.
	ftimeText = strings.Map(func(r rune) rune {
		if r == ' ' {
			return ' '
		}
		return r
	}, ftimeText)

	hour, min, ok := parseLocalTime(ftimeText)
	if !ok {
		return date
	}

	// Build local datetime then shift to UTC.
	local := time.Date(date.Year(), date.Month(), date.Day(), hour, min, 0, 0, time.UTC)
	return local.Add(time.Duration(-offsetHours) * time.Hour)
}

// parseUTCOffset parses strings like "UTC−6", "UTC+1", "UTC−04:00" into hours.
func parseUTCOffset(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "UTC") {
		return 0, false
	}
	rest := strings.TrimPrefix(s, "UTC")
	if rest == "" {
		return 0, true
	}
	// Normalise various minus/dash characters to ASCII hyphen.
	rest = strings.NewReplacer("−", "-", "−", "-", "–", "-").Replace(rest)
	sign := 1
	if strings.HasPrefix(rest, "-") {
		sign = -1
		rest = rest[1:]
	} else if strings.HasPrefix(rest, "+") {
		rest = rest[1:]
	}
	h, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(rest, ":", 2)[0]))
	if err != nil {
		return 0, false
	}
	return sign * h, true
}

// parseLocalTime parses "1:00 p.m." / "8:00 p.m." / "12:00 p.m." into 24-hour hour+min.
func parseLocalTime(s string) (hour, min int, ok bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	isPM := strings.Contains(s, "p.m.") || strings.Contains(s, "pm")
	s = strings.NewReplacer("p.m.", "", "a.m.", "", "pm", "", "am", "").Replace(s)
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, ":", 2)
	if len(parts) < 2 {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if isPM && h != 12 {
		h += 12
	} else if !isPM && h == 12 {
		h = 0
	}
	return h, m, true
}
