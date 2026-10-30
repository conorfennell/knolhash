package fpl

import "fmt"

// Picks is a manager's team for a gameweek. Multipliers already encode the
// captain (×2), triple captain (×3), bench (×0) and bench boost (bench ×1).
type Picks struct {
	Entry         int
	Event         int
	Chip          string // raw FPL chip name, "" when none played
	TransferCost  int
	PointsOnBench int // as reported by FPL once the gameweek is scored
	Captain       int // element id
	Vice          int // element id
	Players       []Pick
}

// Pick is one player in a manager's team.
type Pick struct {
	Element    int
	Multiplier int
}

// LiveScore is a manager's gameweek score from current player points, net
// of transfer hits. It ignores auto-subs, so it is provisional until the
// gameweek is settled.
func LiveScore(p Picks, points map[int]int) int {
	total := 0
	for _, pl := range p.Players {
		total += points[pl.Element] * pl.Multiplier
	}
	return total - p.TransferCost
}

// ChipLabel is the short badge for an FPL chip name.
func ChipLabel(chip string) string {
	switch chip {
	case "":
		return ""
	case "3xc":
		return "TC"
	case "bboost":
		return "BB"
	case "freehit":
		return "FH"
	case "wildcard":
		return "WC"
	case "manager":
		return "AM"
	}
	return chip
}

type apiPicks struct {
	ActiveChip   *string `json:"active_chip"`
	EntryHistory struct {
		EventTransfersCost int `json:"event_transfers_cost"`
		PointsOnBench      int `json:"points_on_bench"`
	} `json:"entry_history"`
	Picks []struct {
		Element    int  `json:"element"`
		Multiplier int  `json:"multiplier"`
		IsCaptain  bool `json:"is_captain"`
		IsVice     bool `json:"is_vice_captain"`
	} `json:"picks"`
}

type apiLive struct {
	Elements []struct {
		ID    int `json:"id"`
		Stats struct {
			TotalPoints     int `json:"total_points"`
			Minutes         int `json:"minutes"`
			Goals           int `json:"goals_scored"`
			Assists         int `json:"assists"`
			CleanSheets     int `json:"clean_sheets"`
			GoalsConceded   int `json:"goals_conceded"`
			OwnGoals        int `json:"own_goals"`
			PenaltiesSaved  int `json:"penalties_saved"`
			PenaltiesMissed int `json:"penalties_missed"`
			YellowCards     int `json:"yellow_cards"`
			RedCards        int `json:"red_cards"`
			Saves           int `json:"saves"`
			Bonus           int `json:"bonus"`
		} `json:"stats"`
	} `json:"elements"`
}

// PlayerStats is what a player did in a gameweek (summed over a double
// gameweek).
type PlayerStats struct {
	Points          int
	Minutes         int
	Goals           int
	Assists         int
	CleanSheets     int
	GoalsConceded   int
	OwnGoals        int
	PenaltiesSaved  int
	PenaltiesMissed int
	YellowCards     int
	RedCards        int
	Saves           int
	Bonus           int
}

// FetchLiveStats fetches every player's stats so far in a gameweek.
func (c *Client) FetchLiveStats(event int) (map[int]PlayerStats, error) {
	var raw apiLive
	if err := c.getJSON(fmt.Sprintf("/event/%d/live/", event), &raw); err != nil {
		return nil, err
	}
	out := make(map[int]PlayerStats, len(raw.Elements))
	for _, e := range raw.Elements {
		s := e.Stats
		out[e.ID] = PlayerStats{
			Points: s.TotalPoints, Minutes: s.Minutes, Goals: s.Goals, Assists: s.Assists,
			CleanSheets: s.CleanSheets, GoalsConceded: s.GoalsConceded, OwnGoals: s.OwnGoals,
			PenaltiesSaved: s.PenaltiesSaved, PenaltiesMissed: s.PenaltiesMissed,
			YellowCards: s.YellowCards, RedCards: s.RedCards, Saves: s.Saves, Bonus: s.Bonus,
		}
	}
	return out, nil
}

// PointsOf extracts each player's points from their stats.
func PointsOf(stats map[int]PlayerStats) map[int]int {
	out := make(map[int]int, len(stats))
	for id, s := range stats {
		out[id] = s.Points
	}
	return out
}

type apiHistory struct {
	Chips []struct {
		Name  string `json:"name"`
		Event int    `json:"event"`
	} `json:"chips"`
}

// FetchPicks fetches a manager's team for a gameweek.
func (c *Client) FetchPicks(entry, event int) (Picks, error) {
	var raw apiPicks
	if err := c.getJSON(fmt.Sprintf("/entry/%d/event/%d/picks/", entry, event), &raw); err != nil {
		return Picks{}, err
	}
	p := Picks{Entry: entry, Event: event, TransferCost: raw.EntryHistory.EventTransfersCost, PointsOnBench: raw.EntryHistory.PointsOnBench}
	if raw.ActiveChip != nil {
		p.Chip = *raw.ActiveChip
	}
	for _, pk := range raw.Picks {
		p.Players = append(p.Players, Pick{Element: pk.Element, Multiplier: pk.Multiplier})
		if pk.IsCaptain {
			p.Captain = pk.Element
		}
		if pk.IsVice {
			p.Vice = pk.Element
		}
	}
	return p, nil
}

// FetchLivePoints fetches every player's points so far in a gameweek.
func (c *Client) FetchLivePoints(event int) (map[int]int, error) {
	stats, err := c.FetchLiveStats(event)
	if err != nil {
		return nil, err
	}
	return PointsOf(stats), nil
}

// FetchChips fetches the chips a manager has played, as event → chip name.
func (c *Client) FetchChips(entry int) (map[int]string, error) {
	var raw apiHistory
	if err := c.getJSON(fmt.Sprintf("/entry/%d/history/", entry), &raw); err != nil {
		return nil, err
	}
	out := map[int]string{}
	for _, ch := range raw.Chips {
		out[ch.Event] = ch.Name
	}
	return out, nil
}

// Transfer is one player swap a manager made for a gameweek. FPL only shows
// a gameweek's transfers once its deadline has passed.
type Transfer struct {
	Event int
	In    int // element id
	Out   int // element id
}

type apiTransfer struct {
	Event      int `json:"event"`
	ElementIn  int `json:"element_in"`
	ElementOut int `json:"element_out"`
}

// FetchTransfers fetches every transfer a manager has made this season.
func (c *Client) FetchTransfers(entry int) ([]Transfer, error) {
	var raw []apiTransfer
	if err := c.getJSON(fmt.Sprintf("/entry/%d/transfers/", entry), &raw); err != nil {
		return nil, err
	}
	out := make([]Transfer, 0, len(raw))
	for _, t := range raw {
		out = append(out, Transfer{Event: t.Event, In: t.ElementIn, Out: t.ElementOut})
	}
	return out, nil
}
