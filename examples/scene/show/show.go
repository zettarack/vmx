// Package show is the demoscene wall's picture: an hour of parts, each a
// scene of one family drawn with a seed of its own, all of it from the
// wall's clock alone, so forty tiles that each draw their own square agree
// on the whole picture without a word between them.
//
// A scene plays seven minutes. The vmx intro, the wordmark landing out of
// falling bits with zettarack.com typed under it, opens the show and comes
// back after every two scenes, a minute each time (2026-11-19: three
// minutes, while it scrolled what vmx does in one line); between, the
// families of
// the rotation each play once a round, in an order of the round's own and with a seed of their own: a
// landscape at dawn in one round, a canyon by moonlight in the next. When a
// part ends, every tile changes at a moment of its own, in an order the
// next part picks (moment): its screen shuts to a line of light and opens
// on the new part, so the change is forty machines changing and not one
// picture. A part the controls move to changes every tile at once.
//
// A scene is light; the tile's look makes it colour: blue and green, the
// wall's two versions, each one ramp from black through its colour to white
// by the light's brightness, so a rollout reads tile by tile whatever
// plays; classic, the scene's own colours.
//
// Part of the vmx examples (MIT, see examples/LICENSE).
package show

import (
	"image"
	"math"
)

// SceneLen is how long a scene plays, IntroLen the vmx intro, in seconds.
const (
	SceneLen = 420.0
	IntroLen = 60.0
)

// logoEvery: the intro is every third part, the first among them.
const logoEvery = 3

// cycle is an intro and the scenes after it.
const cycle = IntroLen + (logoEvery-1)*SceneLen

// LenOf is how long part p plays.
func LenOf(p int64) float64 {
	if p%logoEvery == 0 {
		return IntroLen
	}
	return SceneLen
}

// StartOf is when part p starts, on the show's clock.
func StartOf(p int64) float64 {
	if p <= 0 {
		return 0
	}
	c, r := p/logoEvery, p%logoEvery
	s := float64(c) * cycle
	if r > 0 {
		s += IntroLen + float64(r-1)*SceneLen
	}
	return s
}

// PartAt is the part playing at t, and how far into it t is.
func PartAt(t float64) (int64, float64) {
	t = math.Max(t, 0)
	c := math.Floor(t / cycle)
	r := t - c*cycle
	p := int64(c) * logoEvery
	if r < IntroLen {
		return p, r
	}
	r -= IntroLen
	k := int64(r / SceneLen)
	return p + 1 + k, r - float64(k)*SceneLen
}

// wipe is how long a change of part takes to cross the wall; flashFor is
// how long a tile's flash lasts when a cut changes it; closeFor and openFor
// how long a tile's screen takes to close to a line of light on the part
// ending and to open again on the next.
const (
	wipe     = 3.0
	flashFor = 0.45
	closeFor = 0.35
	openFor  = 0.5
)

// moment is when place (col, row) makes part p's change, 0 to 1 of the
// wipe. Each part has a choreography of its own, which every tile works out
// for itself from the part's number, so the forty change in step without a
// word between them: a ripple from one tile, every tile its own moment, a
// sweep across, a sweep down, the board's two colours one after the other,
// the middle out. A clone's place below the picture keeps the moment of
// the row it repeats. (2026-11-19, the operator: "most of the charm of this
// is 40 windows working together .. the transitions have to reflect that";
// one diagonal wipe read as one picture changing.)
func moment(p int64, col, row, cols, rows int) float64 {
	cols, rows = max(cols, 1), max(rows, 1)
	row %= rows
	s := mix64(uint64(p)*0x9e3779b97f4a7c15 + 0x7a3d)
	c, r := float64(col), float64(row)
	cw, rh := float64(max(cols-1, 1)), float64(max(rows-1, 1))
	switch s % 6 {
	case 0:
		oc, or := float64((s>>8)%uint64(cols)), float64((s>>20)%uint64(rows))
		far := math.Max(math.Max(math.Hypot(oc, or), math.Hypot(cw-oc, or)), math.Max(math.Hypot(oc, rh-or), math.Hypot(cw-oc, rh-or)))
		return math.Hypot(c-oc, r-or) / math.Max(far, 1)
	case 1:
		return float64(mix64(s+uint64(row*cols+col)+1)%1000) / 999
	case 2:
		return c / cw
	case 3:
		return r / rh
	case 4:
		return 0.5*float64((col+row)%2) + 0.5*c/cw
	default:
		return math.Hypot(c-cw/2, r-rh/2) / math.Max(math.Hypot(cw/2, rh/2), 1)
	}
}

// Frame is what a tile knows of a moment: the show's clock, in seconds
// since the show began, and the picture's shape. CutPart is the part the
// wall's controls last moved the show to the start of: that change does
// not wait for each tile's turn (the part before it is not what was
// playing), the flash alone still crossing the wall. 0, the zero value,
// is no cut: part 0 has no part before it. Extras are the page's extras
// on (Extras' names), drawn over the scene; any other name is passed over.
type Frame struct {
	T          float64
	Cols, Rows int
	W, H       int
	CutPart    int64
	Extras     []string
}

// Looks are the colours a tile can draw in: blue (the wall's first
// version), green (its second) and classic (the scenes' own colours).
var Looks = map[string]bool{"blue": true, "green": true, "classic": true}

// Show draws one tile's frames. It keeps what a part made at its first
// frame (a landscape's map, a field's columns) for the frames after it.
type Show struct {
	part       int64
	cols, rows int
	w, h       int
	sc         scene
}

// scene is one part: frame draws the tile at t seconds into the part.
type scene interface {
	frame(v *view, t float64)
}

// family is a kind of scene, built for a seed and the picture's shape.
type family struct {
	name  string
	build func(seed uint64, v *view) scene
}

var wordmark = family{"vmx", newLogo}

// rotation is the families between two wordmarks. Each round plays them
// all, in an order of its own.
var rotation = []family{
	{"grid", newGrid},
	{"land", newLand},
	{"day", newDay},
	{"space", newSpace},
	{"shapes", newShapes},
	{"mandel", newMandel},
	{"julia", newJulia},
	{"blackhole", newBlackhole},
	{"plasma", newPlasma},
	{"cube", newCube},
	{"stone", newStone},
	{"wood", newWood},
	{"gems", newGems},
}

// PartName is the name of the family part p plays, for a log or a page.
func PartName(p int64) string {
	f, _ := partOf(p)
	return f.name
}

// Families are the families' names, the wordmark's first: what the wall's
// page offers to go to.
func Families() []string {
	n := []string{wordmark.name}
	for _, f := range rotation {
		n = append(n, f.name)
	}
	return n
}

// Reach is how many parts ahead NextOf must look to find any family: two
// rounds of the rotation (a family first in one and last in the next) with
// an intro after every two scenes, and a margin. Fixed counts were sized
// for ten families and missed some when the show grew (2026-11-19).
func Reach() int { return 3*len(rotation) + 6 }

// NextOf is the first part after p that plays the family named name, or
// -1 when none does within limit parts; Reach() finds any of them.
func NextOf(p int64, name string, limit int) int64 {
	for q := p + 1; q <= p+int64(limit); q++ {
		if PartName(q) == name {
			return q
		}
	}
	return -1
}

// partOf is part p's family and seed: the wordmark every logoEvery parts,
// and between, round after round of the rotation, each round in an order
// of its own that never starts with the family the round before ended on.
func partOf(p int64) (family, uint64) {
	if p < 0 {
		p = 0
	}
	seed := mix64(uint64(p)*0x9e3779b97f4a7c15 + 0x2545f4914f6cdd1d)
	if p%logoEvery == 0 {
		return wordmark, seed
	}
	k := p - p/logoEvery - 1
	n := int64(len(rotation))
	return rotation[roundOrder(k / n)[k%n]], seed
}

func roundOrder(r int64) []int {
	o := shuffled(r)
	if r > 0 {
		before := shuffled(r - 1)
		if o[0] == before[len(before)-1] {
			o[0], o[1] = o[1], o[0]
		}
	}
	return o
}

func shuffled(r int64) []int {
	o := make([]int, len(rotation))
	for i := range o {
		o[i] = i
	}
	s := mix64(uint64(r) + 0x632be59bd9b4e019)
	for i := len(o) - 1; i > 0; i-- {
		s = mix64(s + 0x9e3779b97f4a7c15)
		j := int(s % uint64(i+1))
		o[i], o[j] = o[j], o[i]
	}
	return o
}

// Render draws tile place of the picture at f, in look.
func (s *Show) Render(place int, f Frame, look string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, max(f.W, 1), max(f.H, 1)))
	if f.Cols < 1 || f.Rows < 1 || f.W < 1 || f.H < 1 || place < 0 {
		return img
	}
	col, row := place%f.Cols, place/f.Cols
	t := math.Max(f.T, 0)
	p, local := PartAt(t)
	// The tile's own moment of change, in this part's choreography.
	at := moment(p, col, row, f.Cols, f.Rows) * wipe
	if p > 0 && f.CutPart == p {
		// A cut: every tile changes at once, a flash alone crossing the wall.
		flash := 0.0
		if local >= at && local-at < flashFor {
			flash = 1 - (local-at)/flashFor
		}
		return s.draw(img, col, row, f, p, local, flash, look)
	}
	if p > 0 && local < at {
		// Its moment not come: the part before goes on, its screen closing
		// to a line in the last closeFor before the moment.
		s.draw(img, col, row, f, p-1, local+LenOf(p-1), 0, look)
		if at-local < closeFor {
			shut(img, 1-(at-local)/closeFor)
		}
		return img
	}
	s.draw(img, col, row, f, p, local, 0, look)
	if p > 0 && local-at < openFor {
		shut(img, 1-(local-at)/openFor)
	}
	return img
}

// shut closes a tile's screen toward a line across its middle: phase 0
// open, 1 shut, the whole picture squeezed into the band left open, and a
// line of light across the middle as it closes. A window switching, one
// machine at a time.
func shut(img *image.RGBA, phase float64) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if phase <= 0 || w < 1 || h < 1 {
		return
	}
	e := math.Min(phase, 1)
	e = e * e * (3 - 2*e)
	band, mid := (1-e)*float64(h), float64(h)/2
	src := append([]uint8(nil), img.Pix...)
	for y := 0; y < h; y++ {
		d := float64(y) + 0.5 - mid
		in := band >= 1 && math.Abs(d) <= band/2
		sy := 0
		if in {
			sy = min(max(int(mid+d*float64(h)/band), 0), h-1)
		}
		for x := 0; x < w; x++ {
			o, so := y*img.Stride+x*4, sy*img.Stride+x*4
			if !in {
				img.Pix[o], img.Pix[o+1], img.Pix[o+2] = 0, 0, 0
				continue
			}
			img.Pix[o], img.Pix[o+1], img.Pix[o+2] = src[so], src[so+1], src[so+2]
		}
	}
	if e <= 0.5 {
		return
	}
	l := (e - 0.5) / 0.5
	for _, y := range []int{int(mid) - 1, int(mid)} {
		if y < 0 || y >= h {
			continue
		}
		for x := 0; x < w; x++ {
			k := l * math.Sqrt(math.Max(0, 1-math.Abs(float64(x)+0.5-float64(w)/2)/(float64(w)/2)))
			o := y*img.Stride + x*4
			for c := 0; c < 3; c++ {
				img.Pix[o+c] = uint8(math.Min(255, float64(img.Pix[o+c])+k*(255-float64(img.Pix[o+c]))))
			}
		}
	}
}

// draw draws part p at local seconds into it on the tile at (col, row) of
// the picture into img, the frame's extras over it, a change's flash
// added.
func (s *Show) draw(img *image.RGBA, col, row int, f Frame, p int64, local, flash float64, look string) *image.RGBA {
	v := &view{x0: col * f.W, y0: row * f.H, w: f.W, h: f.H, pw: f.Cols * f.W, ph: f.Rows * f.H}
	if s.sc == nil || s.part != p || s.cols != f.Cols || s.rows != f.Rows || s.w != f.W || s.h != f.H {
		fam, seed := partOf(p)
		s.sc = fam.build(seed, v)
		s.part, s.cols, s.rows, s.w, s.h = p, f.Cols, f.Rows, f.W, f.H
	}
	v.px = make([]vec3, f.W*f.H)
	s.sc.frame(v, local)
	extras(v, f.Extras, math.Max(f.T, 0))
	paint(img, v, look, 0.9*flash*flash)
	return img
}

// paint turns the tile's light into its look's colours, a change's flash
// added: blue and green by the light's brightness alone, each through its
// own ramp, black through its colour to white; classic, the light's own.
func paint(img *image.RGBA, v *view, look string, flash float64) {
	for i, c := range v.px {
		if !v.display {
			c = tone(c)
		}
		if flash > 0 {
			c = c.add(gray(flash))
		}
		var r, g, b float64
		switch look {
		case "blue", "green":
			l := sat(0.2126*c.x + 0.7152*c.y + 0.0722*c.z)
			lo, mid, hi := l*l*l*l, l*l, math.Sqrt(l)
			if look == "blue" {
				r, g, b = lo, mid, hi
			} else {
				r, g, b = mid, hi, lo
			}
		default:
			r, g, b = c.x, c.y, c.z
		}
		o := img.Pix[i*4 : i*4+4 : i*4+4]
		o[0], o[1], o[2], o[3] = u8(r), u8(g), u8(b), 255
	}
}

func u8(x float64) uint8 { return uint8(sat(x)*255 + 0.5) }

// tone is light on a screen: a filmic shoulder for what is brighter than
// white, then the screen's gamma (2, near enough, and cheap).
func tone(c vec3) vec3 { return vec3{tone1(c.x), tone1(c.y), tone1(c.z)} }

func tone1(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return math.Sqrt(sat(x * (2.51*x + 0.03) / (x*(2.43*x+0.59) + 0.14)))
}

// view is one tile's window on the picture: where the tile sits in it, the
// picture's pixels (its own rows: a clone's place is below them), and the
// tile's light, a row after another.
type view struct {
	x0, y0, w, h int
	pw, ph       int
	px           []vec3
	// display says px is already the screen's colour, not light: the
	// palettes of the classic effects.
	display bool
}

// aspect is the picture's width in heights.
func (v *view) aspect() float64 { return float64(v.pw) / float64(v.ph) }

// each sets every pixel of the tile to f of its place in the picture: x
// across from the middle and y up from it, both in heights of the picture.
func (v *view) each(f func(x, y float64) vec3) {
	inv := 1 / float64(v.ph)
	cx, cy := float64(v.pw)/2, float64(v.ph)/2
	for j := 0; j < v.h; j++ {
		y := (cy - float64(v.y0+j) - 0.5) * inv
		row := v.px[j*v.w : (j+1)*v.w]
		for i := range row {
			row[i] = f((float64(v.x0+i)+0.5-cx)*inv, y)
		}
	}
}

// add adds light c to the picture's pixel (gx, gy) when it is the tile's.
func (v *view) add(gx, gy int, c vec3) {
	x, y := gx-v.x0, gy-v.y0
	if x < 0 || y < 0 || x >= v.w || y >= v.h {
		return
	}
	v.px[y*v.w+x] = v.px[y*v.w+x].add(c)
}

// overlaps says whether the picture's box (gx, gy, w, h) has a pixel of
// the tile.
func (v *view) overlaps(gx, gy, w, h int) bool {
	return gx < v.x0+v.w && gx+w > v.x0 && gy < v.y0+v.h && gy+h > v.y0
}

// box adds light c over the picture's box (gx, gy, w, h), clipped to the
// tile.
func (v *view) box(gx, gy, w, h int, c vec3) {
	x1, y1 := max(gx, v.x0), max(gy, v.y0)
	x2, y2 := min(gx+w, v.x0+v.w), min(gy+h, v.y0+v.h)
	for y := y1; y < y2; y++ {
		row := v.px[(y-v.y0)*v.w:]
		for x := x1; x < x2; x++ {
			row[x-v.x0] = row[x-v.x0].add(c)
		}
	}
}

// glow adds a soft light around the picture's point (gx, gy), falling to
// nothing r pixels away.
func (v *view) glow(gx, gy, r float64, c vec3) {
	x1, y1 := max(int(gx-r), v.x0), max(int(gy-r), v.y0)
	x2, y2 := min(int(gx+r)+1, v.x0+v.w), min(int(gy+r)+1, v.y0+v.h)
	inv := 1 / (r * r)
	for y := y1; y < y2; y++ {
		dy := float64(y) + 0.5 - gy
		row := v.px[(y-v.y0)*v.w:]
		for x := x1; x < x2; x++ {
			dx := float64(x) + 0.5 - gx
			if k := 1 - (dx*dx+dy*dy)*inv; k > 0 {
				row[x-v.x0] = row[x-v.x0].add(c.scale(k * k))
			}
		}
	}
}

// stamp adds glyph g with its top left at the picture's pixel (gx, gy),
// each of its cells s pixels square, in light c.
func (v *view) stamp(g *glyph, gx, gy, s int, c vec3) {
	if !v.overlaps(gx, gy, 5*s, 7*s) {
		return
	}
	for r := 0; r < 7; r++ {
		for k := 0; k < 5; k++ {
			if g[r][k] == '#' {
				v.box(gx+k*s, gy+r*s, s, s, c)
			}
		}
	}
}

// Extras are what a page can lay over any scene, as the desktop's
// screensaver lays layers over its picture: stars, rasters (copper bars
// rolling), scanlines, a vignette and grain. Every tile draws them over its
// own square, from the picture's coordinates, so they meet at the tiles'
// edges as the scene does.
var Extras = []string{"stars", "rasters", "scanlines", "vignette", "grain"}

// extras lays the extras named in on over the tile's picture at t.
func extras(v *view, on []string, t float64) {
	if len(on) == 0 {
		return
	}
	has := map[string]bool{}
	for _, n := range on {
		has[n] = true
	}
	stars, rasters, scan, vig, grain := has["stars"], has["rasters"], has["scanlines"], has["vignette"], has["grain"]
	scale := 1.0
	if v.display {
		// A palette's colours are the screen's already: an extra's light is
		// laid on at the screen's own strength.
		scale = 0.6
	}
	cx, cy := float64(v.pw)/2, float64(v.ph)/2
	// The copper bars: four, each its own colour, rolling up and down.
	var mid [4]float64
	var tint [4]vec3
	for bar := range mid {
		fb := float64(bar)
		mid[bar] = cy + cy*0.85*math.Sin(t*0.9+fb*0.8)
		tint[bar] = palette{gray(0.5), gray(0.5), v3(1, 1, 1), v3(fb*0.25, 0.33+fb*0.25, 0.67+fb*0.25)}.at(0).scale(0.45 * scale)
	}
	for j := 0; j < v.h; j++ {
		gy := v.y0 + j
		for i := 0; i < v.w; i++ {
			gx := v.x0 + i
			k := j*v.w + i
			c := v.px[k]
			if stars {
				h := lattice(int64(gx/2), int64((float64(gy)+t*6)/2), 0x5a7)
				if h&511 < 2 && gx%2 == 0 {
					b := float64(h>>9&255) / 255
					c = c.add(gray((0.4 + 1.6*b*b) * (0.6 + 0.4*math.Sin(t*(2+3*b)+float64(h>>20&63))) * scale))
				}
			}
			if rasters {
				for bar := range mid {
					if d := (float64(gy) - mid[bar]) / (float64(v.ph) * 0.03); d > -1 && d < 1 {
						c = c.add(tint[bar].scale(math.Cos(d * math.Pi / 2)))
					}
				}
			}
			if scan && gy%2 == 1 {
				c = c.scale(0.55)
			}
			if vig {
				dx, dy := (float64(gx)-cx)/cx, (float64(gy)-cy)/cy
				c = c.scale(1 - 0.55*sat(0.6*(dx*dx+dy*dy)-0.15))
			}
			if grain {
				g := h01(uint64(gx)*0x9e37+uint64(gy), uint64(t*24))
				c = c.add(gray((g - 0.5) * 0.08 * scale))
			}
			v.px[k] = c
		}
	}
}
