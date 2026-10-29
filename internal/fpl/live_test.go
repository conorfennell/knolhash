package fpl

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func fakeLiveFPL(t *testing.T) *Client {
	t.Helper()
	mux := http.NewServeMux()
	file := func(path, name string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			b, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			w.Write(b)
		})
	}
	file("/event/8/live/", "live_event8.json")
	file("/entry/5134711/event/8/picks/", "picks_5134711_event8.json")
	file("/entry/2196128/event/8/picks/", "picks_2196128_event8.json")
	mux.HandleFunc("/entry/2196128/history/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"chips":[{"name":"bboost","event":3},{"name":"3xc","event":6}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, HTTP: srv.Client()}
}

// Recorded GW8 picks plus GW8 live points reproduce the final H2H scores,
// including a −8 transfer hit.
func TestLiveScoreMatchesRecordedGameweek(t *testing.T) {
	c := fakeLiveFPL(t)
	points, err := c.FetchLivePoints(8)
	if err != nil {
		t.Fatal(err)
	}
	for entry, want := range map[int]int{5134711: 39, 2196128: 11} {
		p, err := c.FetchPicks(entry, 8)
		if err != nil {
			t.Fatal(err)
		}
		if got := LiveScore(p, points); got != want {
			t.Errorf("entry %d live score = %d, want %d (hits %d)", entry, got, want, p.TransferCost)
		}
	}
}

func TestLiveScoreMultipliers(t *testing.T) {
	points := map[int]int{1: 10, 2: 6, 3: 2, 4: 8}
	cases := []struct {
		name string
		p    Picks
		want int
	}{
		{"captain doubles", Picks{Players: []Pick{{1, 2}, {2, 1}, {3, 1}, {4, 0}}}, 20 + 6 + 2},
		{"triple captain", Picks{Chip: "3xc", Players: []Pick{{1, 3}, {2, 1}, {3, 1}, {4, 0}}}, 30 + 6 + 2},
		{"bench boost counts the bench", Picks{Chip: "bboost", Players: []Pick{{1, 2}, {2, 1}, {3, 1}, {4, 1}}}, 20 + 6 + 2 + 8},
		{"transfer hit", Picks{TransferCost: 4, Players: []Pick{{1, 2}, {2, 1}}}, 26 - 4},
		{"unknown player scores nothing yet", Picks{Players: []Pick{{99, 2}}}, 0},
	}
	for _, c := range cases {
		if got := LiveScore(c.p, points); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestFetchChips(t *testing.T) {
	chips, err := fakeLiveFPL(t).FetchChips(2196128)
	if err != nil {
		t.Fatal(err)
	}
	if ChipLabel(chips[3]) != "BB" || ChipLabel(chips[6]) != "TC" || chips[1] != "" {
		t.Errorf("chips = %v", chips)
	}
}
