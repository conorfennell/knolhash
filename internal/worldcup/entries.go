package worldcup

import "time"

// Entry is one sweepstake entry: a name and four chosen teams.
type Entry struct {
	Name  string
	Teams [4]string
}

// TeamState holds tournament progress for one team.
type TeamState struct {
	Name                string
	GoalsFor            int  // goals scored across all tournament matches
	GroupPosition       int  // current position within their group (1–4), 0 = unknown
	Played              int  // group-stage games played
	FinalPlace          int  // 1–48 when eliminated/finished; 0 = still active
	ThirdPlaceGroupRank int  // 1–8 for 3rd-placed teams that advance to knockouts; 0 otherwise
	Provisional         bool // true when placement/rank is based on incomplete group stage data
}

// Match represents one tournament fixture.
type Match struct {
	HomeTeam  string
	AwayTeam  string
	KickOff   time.Time // UTC; zero if unknown
	HomeScore int
	AwayScore int
	Played    bool
	Group     string
	Venue     string // e.g. "NRG Stadium, Houston"
}

// TournamentData is the full scraped state of the tournament.
type TournamentData struct {
	Teams     map[string]TeamState // keyed by canonical team name
	Matches   []Match
	FetchedAt time.Time
}

// EntryResult holds computed scores for one entry.
type EntryResult struct {
	Entry       Entry
	TotalPoints int
	TotalGoals  int
	TeamStates  [4]TeamState
}

// PrizeResult pairs a winner name with an amount.
type PrizeResult struct {
	EntryName string
	Amount    float64
}

// PrizeSummary holds the four prize categories.
type PrizeSummary struct {
	MostPoints    PrizeResult
	MostGoals     PrizeResult
	LeastPoints   PrizeResult
	OverallWinner PrizeResult
}

// TotalPot is the prize pool: 20 entries × €5.
const TotalPot = 100

// wikiToEntry maps Wikipedia team names to the canonical names used in Entries.
// Only entries that differ need to be listed here.
var wikiToEntry = map[string]string{
	"Czech Republic":          "Czechia",
	"Bosnia and Herzegovina":  "Bosnia",
	"Curaçao":            "Curacao", // Curaçao
	"Cape Verde":              "Cabo Verde",
	"DR Congo":                "Congo DR",
	"United States":           "USA",
}

// Entries is the complete, locked list of 20 sweepstake entries.
var Entries = []Entry{
	{Name: "James P", Teams: [4]string{"Spain", "Senegal", "Egypt", "Paraguay"}},
	{Name: "Paul", Teams: [4]string{"Switzerland", "Colombia", "Australia", "Tunisia"}},
	{Name: "Paul Football Friends", Teams: [4]string{"France", "Canada", "Sweden", "Uzbekistan"}},
	{Name: "Rory", Teams: [4]string{"Brazil", "Austria", "Czechia", "Cabo Verde"}},
	{Name: "James V", Teams: [4]string{"Argentina", "Morocco", "Saudi Arabia", "New Zealand"}},
	{Name: "Conor", Teams: [4]string{"England", "South Korea", "Norway", "Jordan"}},
	{Name: "Oisin", Teams: [4]string{"Belgium", "Japan", "Congo DR", "South Africa"}},
	{Name: "Oisin 2", Teams: [4]string{"Portugal", "Ecuador", "Algeria", "Ghana"}},
	{Name: "Eoghan", Teams: [4]string{"Mexico", "Croatia", "Ivory Coast", "Paraguay"}},
	{Name: "Eoghan 2", Teams: [4]string{"Germany", "Uruguay", "Scotland", "Bosnia"}},
	{Name: "Shermain", Teams: [4]string{"Mexico", "Japan", "Norway", "New Zealand"}},
	{Name: "Kevin", Teams: [4]string{"Netherlands", "Morocco", "Egypt", "Bosnia"}},
	{Name: "Kevin 2", Teams: [4]string{"Spain", "Turkey", "Ivory Coast", "Iraq"}},
	{Name: "Josie", Teams: [4]string{"Spain", "Morocco", "Norway", "Paraguay"}},
	{Name: "Josie 2", Teams: [4]string{"Mexico", "Turkey", "Qatar", "Curacao"}},
	{Name: "Paul 2", Teams: [4]string{"USA", "Iran", "Panama", "Haiti"}},
	{Name: "Niamh", Teams: [4]string{"France", "Croatia", "Norway", "Paraguay"}},
	{Name: "Steve", Teams: [4]string{"France", "Morocco", "Scotland", "Paraguay"}},
	{Name: "Ann", Teams: [4]string{"Spain", "South Korea", "Norway", "Uzbekistan"}},
	{Name: "Seamus", Teams: [4]string{"Haiti", "Norway", "Argentina", "Colombia"}},
}
