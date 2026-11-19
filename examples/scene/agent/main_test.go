package main

import (
	"encoding/json"
	"testing"

	"scene/show"
)

// A wall that sends the show's clock has the tile draw that moment; one
// from before the page's controls sends none, and the tile draws the
// wall's own time, as it always did.
func TestTheTileDrawsTheShowsClockWhenTheWallSendsOne(t *testing.T) {
	for _, c := range []struct {
		body string
		at   float64
		cut  int64
	}{
		{`{"t":12.5,"cols":8,"rows":5,"w":160,"h":120}`, 12.5, 0},
		{`{"t":12.5,"show":605.25,"cut_part":10,"cols":8,"rows":5,"w":160,"h":120}`, 605.25, 10},
		{`{"t":12.5,"show":0,"cols":8,"rows":5,"w":160,"h":120}`, 0, 0},
	} {
		var f frame
		if err := json.Unmarshal([]byte(c.body), &f); err != nil {
			t.Fatalf("%s: %v", c.body, err)
		}
		if f.at() != c.at || f.CutPart != c.cut {
			t.Errorf("%s: draws %v, cut %d; want %v, cut %d", c.body, f.at(), f.CutPart, c.at, c.cut)
		}
	}
}

// LOOK takes the three looks and nothing else, spelled exactly.
func TestLookTakesTheThreeLooksAlone(t *testing.T) {
	for _, l := range []string{"blue", "green", "classic"} {
		if !show.Looks[l] {
			t.Errorf("LOOK=%s is refused", l)
		}
	}
	for _, l := range []string{"", "ember", "Blue", "blue ", "red"} {
		if show.Looks[l] {
			t.Errorf("LOOK=%q is taken", l)
		}
	}
}

// A tile draws a picture of the wall's tile size, whatever part plays.
func TestRenderDrawsTheTilesSize(t *testing.T) {
	sh := new(show.Show)
	for _, at := range []float64{0, 61, 3600} {
		img := render(sh, 3, frame{T: at, Cols: 8, Rows: 5, W: 40, H: 30}, "blue")
		if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 30 {
			t.Errorf("at %v: %v, want 40x30", at, b)
		}
	}
}
