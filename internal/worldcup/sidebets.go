package worldcup

import "time"

// IndividualBet is one person's wager on a specific team in a match.
type IndividualBet struct {
	EntryName string
	Pick      string // team name they're backing
	Amount    float64
}

// SideBet defines a match and the bets placed on it.
type SideBet struct {
	HomeTeam string
	AwayTeam string
	Bets     []IndividualBet
}

// ResolvedSideBet is a SideBet with the match result filled in.
type ResolvedSideBet struct {
	HomeTeam  string
	AwayTeam  string
	HomeScore int
	AwayScore int
	Played    bool
	Winner    string    // canonical team name, or "" for draw / not played
	KickOff   time.Time // zero if unknown
	Bets      []IndividualBet
}

// SideBets is the complete list of side bets for the tournament.
var SideBets = []SideBet{
	{
		HomeTeam: "Scotland",
		AwayTeam: "Brazil",
		Bets: []IndividualBet{
			{EntryName: "Conor", Pick: "Scotland", Amount: 5},
			{EntryName: "Kevin", Pick: "Brazil", Amount: 5},
		},
	},
}

// ResolveSideBets matches each SideBet against the scraped match list.
func ResolveSideBets(bets []SideBet, matches []Match) []ResolvedSideBet {
	out := make([]ResolvedSideBet, 0, len(bets))
	for _, sb := range bets {
		r := ResolvedSideBet{
			HomeTeam: sb.HomeTeam,
			AwayTeam: sb.AwayTeam,
			Bets:     sb.Bets,
		}
		for _, m := range matches {
			home := canonicalTeam(m.HomeTeam)
			away := canonicalTeam(m.AwayTeam)
			sbHome := canonicalTeam(sb.HomeTeam)
			sbAway := canonicalTeam(sb.AwayTeam)
			if (home == sbHome && away == sbAway) || (home == sbAway && away == sbHome) {
				r.KickOff = m.KickOff
				if m.Played {
					r.Played = true
					// normalise scores to sb ordering
					if home == sbHome {
						r.HomeScore = m.HomeScore
						r.AwayScore = m.AwayScore
					} else {
						r.HomeScore = m.AwayScore
						r.AwayScore = m.HomeScore
					}
					if r.HomeScore > r.AwayScore {
						r.Winner = sb.HomeTeam
					} else if r.AwayScore > r.HomeScore {
						r.Winner = sb.AwayTeam
					}
				}
				break
			}
		}
		out = append(out, r)
	}
	return out
}

// canonicalTeam lower-cases and strips spaces for loose matching.
func canonicalTeam(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out = append(out, c)
	}
	return string(out)
}
