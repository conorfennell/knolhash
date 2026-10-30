package fpl

import "testing"

func TestBuildMatchup(t *testing.T) {
	// A: captain 1 (×2), 2, 3, bench 9. B: captain 2 (×2), 1, 4, bench 3.
	a := Picks{Captain: 1, Players: []Pick{{1, 2}, {2, 1}, {3, 1}, {9, 0}}, TransferCost: 4}
	b := Picks{Captain: 2, Chip: "3xc", Players: []Pick{{2, 3}, {1, 1}, {4, 1}, {3, 0}}}
	points := map[int]int{1: 10, 2: 6, 3: 2, 4: 8, 9: 15}
	m := BuildMatchup(a, b, points, []Transfer{{Event: 5, In: 3, Out: 7}}, nil)

	if m.A.Captain != (PlayerScore{Element: 1, Points: 20, Multiplier: 2}) || m.B.Captain != (PlayerScore{Element: 2, Points: 18, Multiplier: 3}) {
		t.Errorf("captains = %+v / %+v", m.A.Captain, m.B.Captain)
	}
	// 3 is on B's bench, so it's a differential for A; 9 is on A's bench and doesn't count.
	if len(m.A.Only) != 1 || m.A.Only[0].Element != 3 || m.A.Only[0].Points != 2 {
		t.Errorf("only A = %+v, want player 3", m.A.Only)
	}
	if len(m.B.Only) != 1 || m.B.Only[0].Element != 4 || m.B.Only[0].Points != 8 {
		t.Errorf("only B = %+v, want player 4", m.B.Only)
	}
	if len(m.Shared) != 2 || m.Shared[0].Element != 1 || m.Shared[1].Element != 2 {
		t.Errorf("shared = %+v, want 1 then 2 by points", m.Shared)
	}
	if len(m.A.Bench) != 1 || m.A.Bench[0] != (PlayerScore{Element: 9, Points: 15}) || len(m.B.Bench) != 1 || m.B.Bench[0].Element != 3 {
		t.Errorf("benches = %+v / %+v", m.A.Bench, m.B.Bench)
	}
	if m.A.TransferCost != 4 || len(m.A.TransfersIn) != 1 || m.A.TransfersIn[0] != 3 || m.A.TransfersOut[0] != 7 || m.B.Chip != "3xc" {
		t.Errorf("sides = %+v / %+v", m.A, m.B)
	}
}

func TestBuildMatchupBenchBoostCountsBench(t *testing.T) {
	a := Picks{Chip: "bboost", Players: []Pick{{1, 2}, {9, 1}}}
	b := Picks{Players: []Pick{{1, 2}, {9, 0}}}
	m := BuildMatchup(a, b, nil, nil, nil)
	if len(m.A.Only) != 1 || m.A.Only[0].Element != 9 {
		t.Errorf("bench-boosted bench player should be A's differential: %+v", m.A.Only)
	}
}

func TestTransfersFor(t *testing.T) {
	got := TransfersFor([]Transfer{{Event: 3, In: 1}, {Event: 8, In: 2}, {Event: 8, In: 3}}, 8)
	if len(got) != 2 || got[0].In != 2 || got[1].In != 3 {
		t.Errorf("GW8 transfers = %+v", got)
	}
}
