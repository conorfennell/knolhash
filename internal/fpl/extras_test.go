package fpl

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBiggestMargin(t *testing.T) {
	d := league([]int{1, 2, 3, 4},
		week(1, [4]int{1, 80, 2, 40}, [4]int{3, 60, 4, 50}), // 40
		week(2, [4]int{0, 99, 1, 10}, [4]int{3, 90, 4, 50}), // AVERAGE ignored; 40 ties GW1
		week(3, [4]int{2, 20, 4, 95}, [4]int{1, 50, 3, 50}), // 75, but unsettled
	)
	d.Events[2].DataChecked = false
	nr, ok := BiggestMargin(d)
	if !ok || nr.Event != 1 || nr.Margin != 40 || nr.Winner.Entry != 1 || nr.Loser.Entry != 2 {
		t.Errorf("naming rights = %+v, want GW1 entry 1 over 2 by 40 (earlier match keeps a tie)", nr)
	}

	d.Events[2].DataChecked = true
	if nr, _ := BiggestMargin(d); nr.Event != 3 || nr.Margin != 75 || nr.Winner.Entry != 4 {
		t.Errorf("naming rights = %+v, want GW3 entry 4 by 75", nr)
	}
}

func TestWeeklySummaryRecordedGameweek(t *testing.T) {
	srv := fakeFPL(t)
	data, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).FetchLeague(LeagueID)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := WeeklySummary(data, BuildLedger(data), 8)
	if !ok {
		t.Fatal("no summary for settled GW8")
	}
	for _, want := range []string{
		"Gameweek 8\n",
		"Luke won gameweek 8 and won €10.",
		"Kevin and Paul lost gameweek 8 and lost €5 each.",
		"currently holds naming rights to",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "−") {
		t.Errorf("summary uses a Unicode minus; WhatsApp text should use '-':\n%s", got)
	}
	if _, ok := WeeklySummary(data, BuildLedger(data), 9); ok {
		t.Error("summary produced for unplayed GW9")
	}
}

func TestWeeklySummaryTiedTop(t *testing.T) {
	d := league([]int{1, 2, 3, 4}, week(1, [4]int{1, 90, 2, 40}, [4]int{3, 90, 4, 30}))
	got, _ := WeeklySummary(d, BuildLedger(d), 1)
	if !strings.Contains(got, "A and C won gameweek 1 and split €10.") {
		t.Errorf("tied top line wrong:\n%s", got)
	}
	if !strings.Contains(got, "A   €10\n") || !strings.Contains(got, "B   -€10\n") {
		t.Errorf("standings lines wrong:\n%s", got)
	}
}

func TestWeeklySummaryThreeWayTie(t *testing.T) {
	d := league([]int{1, 2, 3, 4, 5, 6}, week(1, [4]int{1, 80, 2, 10}, [4]int{3, 80, 4, 20}, [4]int{5, 80, 6, 50}))
	got, _ := WeeklySummary(d, BuildLedger(d), 1)
	if !strings.Contains(got, "A, C and E won gameweek 1 and split €10.") {
		t.Errorf("three-way tie line wrong:\n%s", got)
	}
}

func TestBiggestMarginNoneSettled(t *testing.T) {
	d := league([]int{1, 2}, week(1, [4]int{1, 80, 2, 40}))
	d.Events[0].DataChecked = false
	if _, ok := BiggestMargin(d); ok {
		t.Error("naming rights from an unsettled gameweek")
	}
	if _, ok := WeeklySummary(d, BuildLedger(d), 1); ok {
		t.Error("summary for an unsettled gameweek")
	}
}

func TestJoinNames(t *testing.T) {
	for in, want := range map[string]string{"": "", "A": "A", "A,B": "A and B", "A,B,C": "A, B and C"} {
		var names []string
		if in != "" {
			names = strings.Split(in, ",")
		}
		if got := joinNames(names); got != want {
			t.Errorf("joinNames(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConcerns(t *testing.T) {
	pct := func(n int) *int { return &n }
	players := map[int]Player{
		1: {ID: 1, Name: "Fit", Status: "a"},
		2: {ID: 2, Name: "Knock", Status: "d", Chance: pct(75)},
		3: {ID: 3, Name: "Crocked", Status: "i", Chance: pct(0)},
		4: {ID: 4, Name: "BenchBan", Status: "s"},
		5: {ID: 5, Name: "Returning", Status: "a", Chance: pct(75)},
	}
	p := Picks{Players: []Pick{{1, 2}, {2, 1}, {3, 1}, {5, 1}, {4, 0}}}
	got := Concerns(p, players)
	var names []string
	for _, c := range got {
		names = append(names, c.Player.Name)
	}
	if strings.Join(names, ",") != "Crocked,Knock,Returning,BenchBan" {
		t.Errorf("concerns order = %v", names)
	}
	if !got[3].Bench || got[0].Bench {
		t.Errorf("bench flags wrong: %+v", got)
	}

	p.Chip = "bboost"
	if c := Concerns(p, players); c[len(c)-1].Bench {
		t.Error("bench player counted as bench under bench boost")
	}
}

func TestLiveCachePoll(t *testing.T) {
	var liveCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/entry/1/history/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"chips":[{"name":"wildcard","event":2}]}`))
	})
	mux.HandleFunc("/entry/1/event/5/picks/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"active_chip":"3xc","entry_history":{"event_transfers_cost":4},"picks":[{"element":10,"multiplier":3},{"element":11,"multiplier":1}]}`))
	})
	mux.HandleFunc("/event/5/live/", func(w http.ResponseWriter, r *http.Request) {
		liveCalls++
		if liveCalls == 2 {
			http.Error(w, "blip", http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"elements":[{"id":10,"stats":{"total_points":5}},{"id":11,"stats":{"total_points":2}}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}

	league := &Cache{lastAttempt: time.Now(), data: LeagueData{
		Managers: []Manager{{Entry: 1, DisplayName: "Kevin"}},
		Events:   []Event{{ID: 5, IsCurrent: true, Deadline: time.Now().Add(-time.Hour)}},
	}}
	c := NewLiveCache(client, league)

	c.poll()
	d := c.Get()
	if d.Event != 5 || d.Scores[1] != 5*3+2-4 {
		t.Errorf("live = event %d scores %v, want event 5 score 13", d.Event, d.Scores)
	}
	if d.Chip(1, 5) != "3xc" || d.Chip(1, 2) != "wildcard" {
		t.Errorf("chips = %v", d.Chips)
	}
	if d.PicksEvent != 5 || len(d.Picks[1].Players) != 2 {
		t.Errorf("picks not published: %+v", d)
	}

	// A failed live fetch keeps the last good scores.
	c.poll()
	if d := c.Get(); d.Event != 5 || d.Scores[1] != 13 {
		t.Errorf("after failed poll: event %d scores %v, want previous scores kept", d.Event, d.Scores)
	}

	// Once the gameweek finishes, live scores stop but the team is kept for
	// availability.
	league.data.Events[0].Finished = true
	c.poll()
	if d := c.Get(); d.Event != 0 || d.Scores != nil || d.PicksEvent != 5 {
		t.Errorf("after finish: %+v", d)
	}
}
