package fpl

import "sort"

// Matchup compares two managers' teams for one H2H fixture: who captained
// whom, the players only one of them has (where the match is won or lost)
// and the players they share.
type Matchup struct {
	A, B   MatchupSide
	Shared []PlayerScore // in both starting line-ups
}

// MatchupSide is one manager's side of a matchup.
type MatchupSide struct {
	Captain      PlayerScore
	Chip         string // raw FPL chip name
	TransferCost int
	Only         []PlayerScore // starters the other manager doesn't have
	Bench        []PlayerScore // players who didn't count (none under bench boost)
	TransfersIn  []int         // element ids
	TransfersOut []int
}

// PlayerScore is a player and what they scored for a manager (points ×
// multiplier). Points is zero when the gameweek hasn't started.
type PlayerScore struct {
	Element    int
	Points     int
	Multiplier int
}

// BuildMatchup compares two teams. points may be nil for a gameweek that
// hasn't started; transfers are each manager's transfers for this
// gameweek.
func BuildMatchup(a, b Picks, points map[int]int, transfersA, transfersB []Transfer) Matchup {
	startersA, startersB := starters(a), starters(b)
	score := func(el, mult int) PlayerScore {
		return PlayerScore{Element: el, Points: points[el] * mult, Multiplier: mult}
	}
	side := func(p Picks, mine, theirs map[int]int, transfers []Transfer) MatchupSide {
		s := MatchupSide{Chip: p.Chip, TransferCost: p.TransferCost}
		if c := effectiveCaptain(p); c != 0 {
			s.Captain = score(c, mine[c])
		}
		for el, mult := range mine {
			if _, shared := theirs[el]; !shared {
				s.Only = append(s.Only, score(el, mult))
			}
		}
		sortScores(s.Only)
		for _, pl := range p.Players {
			if pl.Multiplier == 0 {
				s.Bench = append(s.Bench, PlayerScore{Element: pl.Element, Points: points[pl.Element]})
			}
		}
		for _, t := range transfers {
			s.TransfersIn = append(s.TransfersIn, t.In)
			s.TransfersOut = append(s.TransfersOut, t.Out)
		}
		return s
	}
	m := Matchup{A: side(a, startersA, startersB, transfersA), B: side(b, startersB, startersA, transfersB)}
	for el := range startersA {
		if _, ok := startersB[el]; ok {
			m.Shared = append(m.Shared, score(el, 1))
		}
	}
	sortScores(m.Shared)
	return m
}

// starters maps each player who counts towards the score (the starting XI,
// or all 15 under bench boost) to their multiplier.
func starters(p Picks) map[int]int {
	out := map[int]int{}
	for _, pl := range p.Players {
		if pl.Multiplier > 0 {
			out[pl.Element] = pl.Multiplier
		}
	}
	return out
}

// sortScores orders by points, highest first, then by element id so the
// order is stable before a gameweek starts.
func sortScores(s []PlayerScore) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Points != s[j].Points {
			return s[i].Points > s[j].Points
		}
		return s[i].Element < s[j].Element
	})
}

// TransfersFor filters a manager's transfers to one gameweek.
func TransfersFor(all []Transfer, event int) []Transfer {
	var out []Transfer
	for _, t := range all {
		if t.Event == event {
			out = append(out, t)
		}
	}
	return out
}
