package main

import (
	"image/color"
	"testing"
)

// Each look's colours at the palette's four turns (s = 0.5, 1, 0.5, 0): blue
// and green are one ramp, black through their colour to white, in two
// orders of the channels; a look the program does not know draws as
// classic, which main refuses before it draws (LOOK is checked against
// looks).
func TestEachLookHasItsOwnColours(t *testing.T) {
	for _, c := range []struct {
		look string
		p    float64
		want color.RGBA
	}{
		{"blue", 0, color.RGBA{15, 63, 180, 255}},
		{"blue", 0.25, color.RGBA{255, 255, 255, 255}},
		{"blue", 0.75, color.RGBA{0, 0, 0, 255}},
		{"green", 0, color.RGBA{63, 180, 15, 255}},
		{"green", 0.25, color.RGBA{255, 255, 255, 255}},
		{"green", 0.75, color.RGBA{0, 0, 0, 255}},
		{"classic", 0, color.RGBA{127, 239, 15, 255}},
		{"", 0, color.RGBA{127, 239, 15, 255}},
	} {
		if got := shade(c.look, c.p); got != c.want {
			t.Errorf("%q at %.2f: %v, want %v", c.look, c.p, got, c.want)
		}
	}
	for _, l := range []string{"blue", "green", "classic"} {
		if !looks[l] {
			t.Errorf("LOOK=%s is refused", l)
		}
	}
	for _, l := range []string{"", "ember", "Blue", "blue ", "red"} {
		if looks[l] {
			t.Errorf("LOOK=%q is taken", l)
		}
	}
}
