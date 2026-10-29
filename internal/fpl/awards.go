package fpl

import (
	"fmt"
	"sort"
)

// Award is a weekly Hall of Fame or Hall of Shame award.
type Award struct {
	Icon    string
	Title   string
	Shame   bool
	Winners []AwardWinner // more than one on a tie
}

// AwardWinner is a manager who won an award, with why.
type AwardWinner struct {
	Entry  int
	Detail string
}

// GameweekHistory is what the awards need for one settled gameweek: every
// manager's final team and every player's points.
type GameweekHistory struct {
	Event  int
	Picks  map[int]Picks // entry → team
	Points map[int]int   // element id → points
}

// GameweekAwards works out the awards for a settled gameweek. Matches give
// the final scores; AVERAGE never wins anything.
func GameweekAwards(matches []Match, h GameweekHistory, players map[int]Player) []Award {
	var awards []Award
	add := func(a Award) {
		if len(a.Winners) > 0 {
			awards = append(awards, a)
		}
	}
	name := func(element int) string {
		if p, ok := players[element]; ok {
			return p.Name
		}
		return "?"
	}

	type result struct {
		entry, score, margin int // margin > 0 won, < 0 lost
	}
	var results []result
	scores := map[int]int{}
	for _, m := range matches {
		for _, pair := range [2][2]Side{{m.Home, m.Away}, {m.Away, m.Home}} {
			me, them := pair[0], pair[1]
			if me.IsAverage() {
				continue
			}
			results = append(results, result{me.Entry, me.Points, me.Points - them.Points})
			scores[me.Entry] = me.Points
		}
	}

	// 🚀 / 🧯 Chip of the week: best and worst score among chip players.
	type chipScore struct {
		entry, score int
		chip         string
	}
	var chips []chipScore
	for entry, p := range h.Picks {
		if s, ok := scores[entry]; ok && p.Chip != "" {
			chips = append(chips, chipScore{entry, s, ChipLabel(p.Chip)})
		}
	}
	chipScoreOf := func(c chipScore) int { return c.score }
	chipWinner := func(c chipScore) AwardWinner {
		return AwardWinner{c.entry, fmt.Sprintf("%s, %d pts", c.chip, c.score)}
	}
	add(Award{Icon: "🚀", Title: "Chip of the Week", Winners: pick(chips, chipScoreOf, true, chipWinner)})
	// Only a flop if someone else's chip did better.
	if best, worst := pick(chips, chipScoreOf, true, chipScoreOf), pick(chips, chipScoreOf, false, chipScoreOf); len(chips) > 1 && worst[0] < best[0] {
		add(Award{Icon: "🧯", Title: "Chip Flop", Shame: true, Winners: pick(chips, chipScoreOf, false, chipWinner)})
	}

	// 🍀 Jammy win: lowest winning score. 💔 Robbed: highest losing score.
	var wins, losses []result
	for _, r := range results {
		switch {
		case r.margin > 0:
			wins = append(wins, r)
		case r.margin < 0:
			losses = append(losses, r)
		}
	}
	add(Award{Icon: "🍀", Title: "Jammy Win", Winners: pick(wins,
		func(r result) int { return r.score }, false,
		func(r result) AwardWinner { return AwardWinner{r.entry, fmt.Sprintf("won with %d", r.score)} })})
	add(Award{Icon: "💔", Title: "Robbed", Shame: true, Winners: pick(losses,
		func(r result) int { return r.score }, true,
		func(r result) AwardWinner { return AwardWinner{r.entry, fmt.Sprintf("lost with %d", r.score)} })})

	// 🪑 Bench of shame: most points left on the bench (not under bench boost).
	type bench struct{ entry, pts int }
	var benches []bench
	for entry, p := range h.Picks {
		if _, ok := scores[entry]; ok && p.Chip != "bboost" && p.PointsOnBench > 0 {
			benches = append(benches, bench{entry, p.PointsOnBench})
		}
	}
	add(Award{Icon: "🪑", Title: "Bench of Shame", Shame: true, Winners: pick(benches,
		func(b bench) int { return b.pts }, true,
		func(b bench) AwardWinner { return AwardWinner{b.entry, fmt.Sprintf("%d pts on the bench", b.pts)} })})

	// 🫠 Captain calamity: lowest captain return, with a dig if the vice
	// would have done better.
	type captain struct{ entry, element, pts, vice, vicePts int }
	var captains []captain
	for entry, p := range h.Picks {
		if _, ok := scores[entry]; !ok {
			continue
		}
		c := effectiveCaptain(p)
		if c == 0 {
			continue
		}
		vice := p.Vice
		if c == p.Vice {
			vice = 0 // the vice already stood in
		}
		captains = append(captains, captain{entry, c, h.Points[c], vice, h.Points[vice]})
	}
	add(Award{Icon: "🫠", Title: "Captain Calamity", Shame: true, Winners: pick(captains,
		func(c captain) int { return c.pts }, false,
		func(c captain) AwardWinner {
			d := fmt.Sprintf("%s (C) %d pts", name(c.element), c.pts)
			if c.vice != 0 && c.vicePts > c.pts {
				d += fmt.Sprintf(", vice %s got %d", name(c.vice), c.vicePts)
			}
			return AwardWinner{c.entry, d}
		})})

	// 💸 Hit that didn't pay: lost by no more than the transfer hit.
	type hit struct{ entry, cost, by int }
	var hits []hit
	for _, r := range results {
		if p, ok := h.Picks[r.entry]; ok && r.margin < 0 && p.TransferCost > 0 && -r.margin <= p.TransferCost {
			hits = append(hits, hit{r.entry, p.TransferCost, -r.margin})
		}
	}
	add(Award{Icon: "💸", Title: "Hit That Didn't Pay", Shame: true, Winners: pick(hits,
		func(x hit) int { return x.cost }, true,
		func(x hit) AwardWinner {
			return AwardWinner{x.entry, fmt.Sprintf("took a −%d hit, lost by %d", x.cost, x.by)}
		})})

	return awards
}

// effectiveCaptain is the player whose points were multiplied: the vice if
// FPL moved the armband because the captain didn't play.
func effectiveCaptain(p Picks) int {
	best, mult := 0, 1
	for _, pl := range p.Players {
		if pl.Multiplier > mult {
			best, mult = pl.Element, pl.Multiplier
		}
	}
	if best == 0 {
		return p.Captain
	}
	return best
}

// pick returns every item tied on the highest (or lowest) key, mapped
// through out and sorted by entry for stable output.
func pick[T, R any](items []T, key func(T) int, highest bool, out func(T) R) []R {
	if len(items) == 0 {
		return nil
	}
	best := key(items[0])
	for _, it := range items[1:] {
		if k := key(it); (highest && k > best) || (!highest && k < best) {
			best = k
		}
	}
	var chosen []T
	for _, it := range items {
		if key(it) == best {
			chosen = append(chosen, it)
		}
	}
	res := make([]R, len(chosen))
	for i, it := range chosen {
		res[i] = out(it)
	}
	if ws, ok := any(res).([]AwardWinner); ok {
		sort.Slice(ws, func(i, j int) bool { return ws[i].Entry < ws[j].Entry })
	}
	return res
}
