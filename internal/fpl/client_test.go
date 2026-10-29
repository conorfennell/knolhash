package fpl

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// fakeFPL serves the recorded responses in testdata. Gameweeks without a
// recorded file return no matches.
func fakeFPL(t *testing.T) *httptest.Server {
	t.Helper()
	serve := func(w http.ResponseWriter, file string) {
		b, err := os.ReadFile("testdata/" + file)
		if err != nil {
			w.Write([]byte(`{"has_next":false,"results":[]}`))
			return
		}
		w.Write(b)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/bootstrap-static/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		serve(w, "bootstrap.json")
	})
	mux.HandleFunc("/leagues-h2h/428253/standings/", func(w http.ResponseWriter, r *http.Request) {
		serve(w, "standings.json")
	})
	mux.HandleFunc("/leagues-h2h-matches/league/428253/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "1" {
			serve(w, "missing")
			return
		}
		serve(w, "matches_event"+r.URL.Query().Get("event")+".json")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchLeague(t *testing.T) {
	srv := fakeFPL(t)
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	data, err := c.FetchLeague(LeagueID)
	if err != nil {
		t.Fatal(err)
	}

	if data.LeagueName != "Taking the trophy off of James" {
		t.Errorf("league name = %q", data.LeagueName)
	}
	if len(data.Standings) != 12 {
		t.Fatalf("standings = %d rows, want 12", len(data.Standings))
	}
	if len(data.Managers) != 11 {
		t.Errorf("managers = %d, want 11 (AVERAGE is not a manager)", len(data.Managers))
	}

	var avg *Standing
	for i := range data.Standings {
		if data.Standings[i].Side.IsAverage() {
			avg = &data.Standings[i]
		}
	}
	if avg == nil || avg.Side.Name != "AVERAGE" {
		t.Errorf("AVERAGE row missing or mislabelled: %+v", avg)
	}

	names := map[string]bool{}
	for _, m := range data.Managers {
		names[m.DisplayName] = true
	}
	for _, want := range []string{"James V", "James P", "Kevin", "Eoghan"} {
		if !names[want] {
			t.Errorf("missing display name %q in %v", want, names)
		}
	}

	cur, ok := data.CurrentEvent()
	if !ok || cur.ID != 8 || !cur.Finished {
		t.Errorf("current event = %+v", cur)
	}
	next, ok := data.NextEvent()
	if !ok || next.ID != 9 {
		t.Errorf("next event = %+v", next)
	}

	gw1 := data.MatchesFor(1)
	if len(gw1) != 6 {
		t.Fatalf("GW1 matches = %d, want 6", len(gw1))
	}
	if !gw1[0].Home.IsAverage() || gw1[0].Home.Points != 50 || gw1[0].Away.Name != "James P" {
		t.Errorf("GW1 first match = %+v", gw1[0])
	}
	if p := data.Players[16]; p.Name != "Saka" || p.Team != "ARS" || !p.Out() || p.StatusLabel() != "Injured" {
		t.Errorf("player 16 = %+v", p)
	}
	if p := data.Players[1]; p.Unavailable() || p.Chance != nil {
		t.Errorf("player 1 = %+v, want fit with unknown chance", p)
	}
	if len(data.MatchesFor(9)) != 6 {
		t.Errorf("GW9 fixtures = %d, want 6", len(data.MatchesFor(9)))
	}
}

func TestFetchLeagueFollowsPages(t *testing.T) {
	page2 := `{"has_next":false,"results":[{"id":99,"event":1,"entry_1_entry":111,"entry_1_name":"Late FC","entry_1_player_name":"Orla Byrne","entry_1_points":60,"entry_2_entry":2196128,"entry_2_name":"Kevin's Team","entry_2_player_name":"Kevin O'Flaherty","entry_2_points":40}]}`
	page1, err := os.ReadFile("testdata/matches_event1.json")
	if err != nil {
		t.Fatal(err)
	}
	page1 = []byte(strings.Replace(string(page1), `"has_next":false`, `"has_next":true`, 1))
	mux := http.NewServeMux()
	mux.HandleFunc("/bootstrap-static/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"events":[{"id":1,"name":"Gameweek 1","is_current":true,"finished":true,"data_checked":true}]}`))
	})
	mux.HandleFunc("/leagues-h2h/428253/standings/", func(w http.ResponseWriter, r *http.Request) {
		b, _ := os.ReadFile("testdata/standings.json")
		w.Write(b)
	})
	mux.HandleFunc("/leagues-h2h-matches/league/428253/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			w.Write(page1)
			return
		}
		w.Write([]byte(page2))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	data, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).FetchLeague(LeagueID)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(data.MatchesFor(1)); got != 7 {
		t.Fatalf("GW1 matches = %d, want 7 across two pages", got)
	}
	// A manager who has left the league is still named from match data.
	if last := data.Matches[len(data.Matches)-1]; last.Home.Name != "Orla" || last.Home.TeamName != "Late FC" {
		t.Errorf("departed manager side = %+v", last.Home)
	}
}

func TestFetchLeagueErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if _, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).FetchLeague(LeagueID); err == nil {
		t.Fatal("want error for 503")
	}
}

func TestDisplayNames(t *testing.T) {
	got := displayNames(map[int]string{
		1: "james vaughan", 2: "James Power", // same first name → initial
		3: "Éamon Ryan",                                    // accented first letter
		4: "Seán Murphy", 5: "Seán Moran", 6: "Seán Moore", // initials clash → full name
		7: "Cher", 8: "Kevin O'Flaherty",
	})
	want := map[int]string{
		1: "James V", 2: "James P", 3: "Éamon",
		4: "Seán Murphy", 5: "Seán Moran", 6: "Seán Moore",
		7: "Cher", 8: "Kevin",
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("displayNames[%d] = %q, want %q", id, got[id], w)
		}
	}
}

func TestEventStatus(t *testing.T) {
	cases := []struct {
		e    Event
		want string
	}{
		{Event{IsCurrent: true}, "live"},
		{Event{IsCurrent: true, Finished: true}, "unconfirmed"},
		{Event{Finished: true, DataChecked: true}, "settled"},
		{Event{IsNext: true}, "upcoming"},
	}
	for _, c := range cases {
		if got := c.e.Status(); got != c.want {
			t.Errorf("%+v.Status() = %q, want %q", c.e, got, c.want)
		}
	}
}
