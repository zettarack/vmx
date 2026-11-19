package show

import (
	"bytes"
	"image"
	"testing"
)

// The wordmark opens the show and is every fifteenth part; between, each
// round of the rotation plays every family once; no family plays twice
// running, across rounds too, for the first hours.
func TestTheScheduleKeepsItsPromises(t *testing.T) {
	last := ""
	round := map[string]int{}
	for p := int64(0); p < 600; p++ {
		name := PartName(p)
		// Fifteen, written here: the promise, not the constant that keeps it.
		if (p%15 == 0) != (name == "vmx") {
			t.Fatalf("part %d plays %s", p, name)
		}
		if name == last {
			t.Fatalf("part %d plays %s twice running", p, name)
		}
		last = name
		if name == "vmx" {
			continue
		}
		round[name]++
		if k := p - p/logoEvery - 1; (k+1)%int64(len(rotation)) == 0 {
			if len(round) != len(rotation) {
				t.Fatalf("the round ending at part %d plays %d families: %v", p, len(round), round)
			}
			round = map[string]int{}
		}
	}
}

func TestFamiliesAndNextOf(t *testing.T) {
	f := Families()
	if len(f) != 10 || f[0] != "vmx" {
		t.Fatalf("families: %v", f)
	}
	seen := map[string]bool{}
	for _, n := range f {
		if seen[n] {
			t.Fatalf("%s twice", n)
		}
		seen[n] = true
	}
	for p := int64(0); p < 120; p++ {
		for _, n := range f {
			q := NextOf(p, n, 20)
			if q <= p || PartName(q) != n {
				t.Fatalf("after part %d the next %s is part %d (%s)", p, n, q, PartName(q))
			}
		}
	}
	if NextOf(0, "nope", 40) != -1 {
		t.Error("a family that is none was found")
	}
}

func frameAt(t float64) Frame { return Frame{T: t, Cols: 4, Rows: 3, W: 24, H: 18} }

// A tile's picture is the show's alone: the same tile at the same moment
// is the same picture from a show that has drawn nothing before and from
// one that has drawn other parts and moments, so no tile can drift from
// the others whatever it was doing.
func TestATilesPictureIsTheShowsAlone(t *testing.T) {
	used := new(Show)
	for _, at := range []float64{5, 61, 200, 905, 33, 1530, 2400.5} {
		for place := 0; place < 12; place += 5 {
			a := new(Show).Render(place, frameAt(at), "classic")
			used.Render((place+1)%12, frameAt(at+7), "blue")
			b := used.Render(place, frameAt(at), "classic")
			if !bytes.Equal(a.Pix, b.Pix) {
				t.Fatalf("tile %d at %v: a fresh show and a used one differ", place, at)
			}
		}
	}
}

func litPixels(img *image.RGBA) int {
	n := 0
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i]|img.Pix[i+1]|img.Pix[i+2] > 24 {
			n++
		}
	}
	return n
}

// Every family draws something on most of its tiles, at its start, half
// way and at its end, in every look; a clone's place below the picture
// draws too; and a frame with no shape is a blank picture, not a panic.
func TestEveryFamilyDraws(t *testing.T) {
	for p := int64(0); p < 12; p++ {
		for _, look := range []string{"blue", "green", "classic"} {
			for _, in := range []float64{12, 30, 58} {
				f := frameAt(float64(p)*PartLen + in)
				drawn := 0
				for place := 0; place < 12; place++ {
					if litPixels(new(Show).Render(place, f, look)) > 0 {
						drawn++
					}
				}
				if drawn < 6 {
					t.Errorf("part %d (%s) at %v s in %s: %d of 12 tiles drew anything", p, PartName(p), in, look, drawn)
				}
			}
		}
		if img := new(Show).Render(13, frameAt(float64(p)*PartLen+30), "blue"); img.Bounds().Dx() != 24 {
			t.Errorf("part %d: a clone's place drew %v", p, img.Bounds())
		}
	}
	for _, f := range []Frame{{}, {T: 5, Cols: 0, Rows: 3, W: 24, H: 18}, {T: 5, Cols: 4, Rows: 3, W: 0, H: 18}} {
		if img := new(Show).Render(0, f, "blue"); img == nil {
			t.Errorf("%+v: no picture", f)
		}
	}
	if img := new(Show).Render(-1, frameAt(5), "blue"); litPixels(img) != 0 {
		t.Error("a place below 0 drew")
	}
}

// When a part ends, the top left tile changes first and the bottom right
// last; a cut changes every tile at once.
func TestAChangeCrossesTheWallAndACutDoesNot(t *testing.T) {
	at := PartLen + 0.3
	first := new(Show).Render(0, frameAt(at), "classic")
	last := new(Show).Render(11, frameAt(at), "classic")
	newFirst := new(Show).Render(0, Frame{T: at, Cols: 4, Rows: 3, W: 24, H: 18, CutPart: 1}, "classic")
	newLast := new(Show).Render(11, Frame{T: at, Cols: 4, Rows: 3, W: 24, H: 18, CutPart: 1}, "classic")
	if !bytes.Equal(first.Pix, newFirst.Pix) {
		t.Error("the top left tile has not changed part 0.3 s after the change")
	}
	if bytes.Equal(last.Pix, newLast.Pix) {
		t.Error("the bottom right tile changed part as soon as the top left")
	}
}

// The looks: blue and green are one ramp each by the light's brightness,
// black through their colour to white; classic is the light's own colour.
func TestTheLooksRamps(t *testing.T) {
	for _, c := range []struct {
		look string
		in   vec3
		want [3]uint8
	}{
		{"blue", gray(0.5), [3]uint8{16, 64, 180}},
		{"green", gray(0.5), [3]uint8{64, 180, 16}},
		{"blue", gray(0), [3]uint8{0, 0, 0}},
		{"green", gray(1), [3]uint8{255, 255, 255}},
		{"classic", v3(0.2, 0.4, 0.6), [3]uint8{51, 102, 153}},
	} {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		paint(img, &view{w: 1, h: 1, px: []vec3{c.in}, display: true}, c.look, 0)
		if got := [3]uint8{img.Pix[0], img.Pix[1], img.Pix[2]}; got != c.want || img.Pix[3] != 255 {
			t.Errorf("%s of %v: %v, want %v", c.look, c.in, got, c.want)
		}
	}
}

// Every character the scrollers say is in their font (a space is the
// gap), so no fact loses a letter on the wall.
func TestEveryFactIsInTheFont(t *testing.T) {
	for _, f := range facts {
		for i := 0; i < len(f); i++ {
			if _, ok := font[f[i]]; !ok && f[i] != ' ' {
				t.Errorf("%q: %q is not in the font", f, f[i])
			}
		}
	}
	if s := scroll(7); len(s) < 100 {
		t.Errorf("a scroller's line: %q", s)
	}
}

// The wordmark and its tagline have a glyph for every character, and
// draw: no letter of either is blank.
func TestTheWordmarkAndTaglineAreInTheFont(t *testing.T) {
	for _, s := range []string{word, tagline} {
		for i := 0; i < len(s); i++ {
			if glyphFor(s[i]) == blank {
				t.Errorf("%q: %q has no glyph", s, s[i])
			}
		}
	}
}
