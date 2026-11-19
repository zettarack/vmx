package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
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

// The extras the wall sends reach the tile's frame: the show's names, each
// once, or none; any other name, a name twice or a value of another type
// refuses the frame, with the name in the error.
func TestTheWallsExtrasAreTheShowsOrTheFrameIsRefused(t *testing.T) {
	shape := `"t":3,"cols":8,"rows":5,"w":160,"h":120`
	for _, c := range []struct {
		extras string
		want   []string
		bad    string
	}{
		{``, nil, ""},
		{`,"extras":null`, nil, ""},
		{`,"extras":[]`, []string{}, ""},
		{`,"extras":["grain"]`, []string{"grain"}, ""},
		{`,"extras":["stars","rasters","scanlines","vignette","grain"]`, []string{"stars", "rasters", "scanlines", "vignette", "grain"}, ""},
		{`,"extras":["bloom"]`, nil, `"bloom"`},
		{`,"extras":["Grain"]`, nil, `"Grain"`},
		{`,"extras":["grain "]`, nil, `"grain "`},
		{`,"extras":[""]`, nil, `""`},
		{`,"extras":["grain","grain"]`, nil, "twice"},
		{`,"extras":["n\u00e4be"]`, nil, "n\u00e4be"},
		{`,"extras":"grain"`, nil, "cannot unmarshal"},
		{`,"extras":[3]`, nil, "cannot unmarshal"},
	} {
		body := "{" + shape + c.extras + "}"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		f, err := ask(srv.Client(), srv.URL)
		srv.Close()
		if c.bad == "" {
			if err != nil {
				t.Errorf("%s: refused: %v", body, err)
			} else if !slices.Equal(f.Extras, c.want) {
				t.Errorf("%s: extras %q, want %q", body, f.Extras, c.want)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: taken, extras %q", body, f.Extras)
		} else if !strings.Contains(err.Error(), c.bad) {
			t.Errorf("%s: %v, want it to say %s", body, err, c.bad)
		}
	}
}

// An extra the frame names is drawn: the scanlines darken the picture's
// odd rows, and the frame without them leaves them as the scene drew them.
func TestTheFramesExtrasAreDrawn(t *testing.T) {
	f := frame{T: 30, Cols: 4, Rows: 3, W: 40, H: 30}
	plain := render(new(show.Show), 5, f, "classic")
	f.Extras = []string{"scanlines"}
	lined := render(new(show.Show), 5, f, "classic")
	darker, same := 0, 0
	for y := 1; y < 30; y += 2 {
		for x := 0; x < 40; x++ {
			a, b := plain.RGBAAt(x, y), lined.RGBAAt(x, y)
			switch {
			case int(b.R)+int(b.G)+int(b.B) < int(a.R)+int(a.G)+int(a.B):
				darker++
			case a == b:
				same++
			}
		}
	}
	if darker < 200 {
		t.Errorf("scanlines darkened %d pixels of the odd rows (%d the same)", darker, same)
	}
}
