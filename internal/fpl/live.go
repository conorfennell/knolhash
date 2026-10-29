package fpl

import "fmt"

// Picks is a manager's team for a gameweek. Multipliers already encode the
// captain (×2), triple captain (×3), bench (×0) and bench boost (bench ×1).
type Picks struct {
	Entry        int
	Event        int
	Chip         string // raw FPL chip name, "" when none played
	TransferCost int
	Players      []Pick
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
	} `json:"entry_history"`
	Picks []struct {
		Element    int `json:"element"`
		Multiplier int `json:"multiplier"`
	} `json:"picks"`
}

type apiLive struct {
	Elements []struct {
		ID    int `json:"id"`
		Stats struct {
			TotalPoints int `json:"total_points"`
		} `json:"stats"`
	} `json:"elements"`
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
	p := Picks{Entry: entry, Event: event, TransferCost: raw.EntryHistory.EventTransfersCost}
	if raw.ActiveChip != nil {
		p.Chip = *raw.ActiveChip
	}
	for _, pk := range raw.Picks {
		p.Players = append(p.Players, Pick{Element: pk.Element, Multiplier: pk.Multiplier})
	}
	return p, nil
}

// FetchLivePoints fetches every player's points so far in a gameweek.
func (c *Client) FetchLivePoints(event int) (map[int]int, error) {
	var raw apiLive
	if err := c.getJSON(fmt.Sprintf("/event/%d/live/", event), &raw); err != nil {
		return nil, err
	}
	out := make(map[int]int, len(raw.Elements))
	for _, e := range raw.Elements {
		out[e.ID] = e.Stats.TotalPoints
	}
	return out, nil
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
