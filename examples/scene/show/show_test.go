package show

import (
	"bytes"
	"image"
	"math"
	"testing"
)

// The intro opens the show and is every third part; between, each
// round of the rotation plays every family once; no family plays twice
// running, across rounds too, for the first hours.
func TestTheScheduleKeepsItsPromises(t *testing.T) {
	last := ""
	round := map[string]int{}
	for p := int64(0); p < 600; p++ {
		name := PartName(p)
		// Every third, written here: the promise, not the constant that
		// keeps it.
		if (p%3 == 0) != (name == "vmx") {
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
	if len(f) != 11 || f[0] != "vmx" {
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
			q := NextOf(p, n, 30)
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
	// Fifteen parts: five intros and the whole of the first round; each
	// early in its part and in its last ten seconds.
	seen := map[string]bool{}
	for p := int64(0); p < 15; p++ {
		seen[PartName(p)] = true
		for _, look := range []string{"blue", "green", "classic"} {
			for _, in := range []float64{12, 30, 58, LenOf(p) - 10} {
				f := frameAt(StartOf(p) + in)
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
		if img := new(Show).Render(13, frameAt(StartOf(p)+30), "blue"); img.Bounds().Dx() != 24 {
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
	if len(seen) != len(Families()) {
		t.Errorf("fifteen parts played %d families of %d", len(seen), len(Families()))
	}
}

// When a part ends, the top left tile changes first and the bottom right
// last; a cut changes every tile at once.
func TestAChangeCrossesTheWallAndACutDoesNot(t *testing.T) {
	at := StartOf(1) + 0.3
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
	// A tile whose turn has not come draws the part before as it goes on,
	// by that part's own clock: into an intro from a scene, and into a
	// scene from an intro, which are not the same length.
	for _, p := range []int64{1, 3} {
		f := frameAt(StartOf(p) + 0.3)
		before := new(Show).draw(image.NewRGBA(image.Rect(0, 0, 24, 18)), 3, 2, f, p-1, LenOf(p-1)+0.3, 0, "classic")
		if img := new(Show).Render(11, f, "classic"); !bytes.Equal(img.Pix, before.Pix) {
			t.Errorf("0.3 s into part %d the bottom right tile is not part %d going on", p, p-1)
		}
	}
}

// Each extra lays something over the picture: the whole picture with it
// is not the picture without it; a name that is no extra lays nothing.
func TestEachExtraDraws(t *testing.T) {
	whole := func(f Frame) []byte {
		var all []byte
		for place := 0; place < f.Cols*f.Rows; place++ {
			all = append(all, new(Show).Render(place, f, "classic").Pix...)
		}
		return all
	}
	f := Frame{T: StartOf(1) + 30, Cols: 4, Rows: 3, W: 80, H: 60}
	plain := whole(f)
	for _, x := range Extras {
		f.Extras = []string{x}
		if bytes.Equal(whole(f), plain) {
			t.Errorf("%s drew nothing", x)
		}
	}
	f.Extras = []string{"bloom", "", "Stars"}
	if !bytes.Equal(whole(f), plain) {
		t.Error("a name that is no extra drew")
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

// The wordmark and its tagline have a glyph for every character (a space
// is the gap), so neither loses a letter.
func TestTheWordmarkAndTaglineAreInTheFont(t *testing.T) {
	for _, s := range []string{word, tagline} {
		for i := 0; i < len(s); i++ {
			if s[i] != ' ' && glyphFor(s[i]) == blank {
				t.Errorf("%q: %q has no glyph", s, s[i])
			}
		}
	}
}

// The schedule's clock: a scene plays seven minutes and the intro one; every part starts where the one before it ends; and the part at
// any moment is the one whose span holds it.
func TestThePartsTakeTheirTimes(t *testing.T) {
	if LenOf(0) != 60 || LenOf(1) != 420 || LenOf(2) != 420 || LenOf(3) != 60 {
		t.Fatalf("lengths: %v %v %v %v", LenOf(0), LenOf(1), LenOf(2), LenOf(3))
	}
	for p := int64(0); p < 60; p++ {
		if StartOf(p+1) != StartOf(p)+LenOf(p) {
			t.Fatalf("part %d starts at %v, part %d at %v, %v long", p+1, StartOf(p+1), p, StartOf(p), LenOf(p))
		}
		for _, in := range []float64{0, 0.5, LenOf(p) / 2, LenOf(p) - 0.01} {
			if q, at := PartAt(StartOf(p) + in); q != p || math.Abs(at-in) > 1e-6 {
				t.Fatalf("%v into part %d: part %d, %v in", in, p, q, at)
			}
		}
	}
	if q, at := PartAt(-5); q != 0 || at != 0 {
		t.Errorf("before the start: part %d at %v", q, at)
	}
}

// The black hole's sky has no seams: rays from one eye, each passing a
// little wider of the hole than the last, leave in ways that turn smoothly
// one to the next, from the edge of the ring of light round the hole's
// shadow to far past the disk. Where the following handed over to the rule
// at a fixed width, or where a ray's count of steps changed, the way out
// jumped, and the stars behind the hole showed rings of doubled stars
// (2026-11-18). A jump is a step in how the turn changes from ray to ray;
// the bending's own curve, steep near the shadow, changes it smoothly. The
// rays lie in the disk's own plane, so no gas stops them.
func TestTheBlackHolesSkyHasNoSeams(t *testing.T) {
	b := newBlackhole(7, &view{}).(*blackhole)
	b.px = 1 / (1.05 * 600)
	eye := v3(0, 0, -21)
	var ang []float64
	for w := 2.9; w < 20; w += 0.002 {
		// The ray aimed to pass w wide of the middle, were it not bent.
		s := w / 21
		_, _, out, gone := b.follow(eye, v3(s, 0, math.Sqrt(1-s*s)), 0)
		if !gone {
			t.Fatalf("a ray %.3f wide fell in", w)
		}
		ang = append(ang, math.Atan2(out.x, out.z))
	}
	worst, at := 0.0, 0
	for i := 3; i < len(ang); i++ {
		third := ang[i] - 3*ang[i-1] + 3*ang[i-2] - ang[i-3]
		if math.Abs(third) > worst {
			worst, at = math.Abs(third), i
		}
	}
	// A pixel of the wall is 1/630 of a radian: a step under a twentieth
	// of one does not show.
	if worst > 1.0/630/20 {
		t.Errorf("the way out steps by %.2g rad at %.3f wide", worst, 2.9+0.002*float64(at))
	}
}

// A cell of the cube that a bolt is fired at is aimed at while the bolt
// flies, flashes when it lands, is gone for its hold, grows back over
// growFor and is whole again, glowing; and the count of cells grown back
// since the part began goes up as it is whole. The shot is one whose cell
// no other shot touches from a story before it to a story after.
func TestTheCubesCellsGoAndGrowBack(t *testing.T) {
	c := newCube(11, &view{pw: 1280, ph: 600}).(*cube)
	var s shot
	found := false
	var shots []shot
	for at := storyFor * 2; at < 300 && len(shots) < 40; at += storyFor {
		c.state(at)
		shots = append(shots, c.shots...)
	}
	for _, a := range shots {
		alone := true
		for _, at := range []float64{a.land, a.land + storyFor} {
			c.state(at)
			for _, b := range c.shots {
				if b.cell == a.cell && b.land != a.land {
					alone = false
				}
			}
		}
		if alone {
			s, found = a, true
			break
		}
	}
	if !found {
		t.Fatal("no shot alone on its cell before 300 s")
	}
	end := s.land + hitFor + s.hold + growFor
	for _, w := range []struct {
		at    float64
		state int
	}{
		{s.fire + 0.05, cellAimed},
		{s.land + 0.05, cellHit},
		{s.land + hitFor + 0.05, cellGone},
		{s.land + hitFor + s.hold - 0.05, cellGone},
		{s.land + hitFor + s.hold + 0.05, cellGrowing},
		{end + 0.05, cellOK},
		{s.fire - 0.05, cellOK},
	} {
		c.state(w.at)
		if got := c.cells[s.cell].state; got != w.state {
			t.Errorf("at %.2f s (the bolt fired %.2f, landed %.2f, held %.2f) the cell is %d, want %d", w.at, s.fire, s.land, s.hold, got, w.state)
		}
	}
	c.state(end + 0.05)
	if k := c.cells[s.cell].k; k < 0.9 {
		t.Errorf("just grown back, the cell glows %.2f", k)
	}
	if before, after := c.state(end-0.02), c.state(end+0.02); after < before+1 {
		t.Errorf("grown back: %d before the cell is whole, %d after", before, after)
	}
}
