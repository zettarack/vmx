// Package show is the demoscene wall's picture: an hour of parts, each a
// scene of one family drawn with a seed of its own, all of it from the
// wall's clock alone, so forty tiles that each draw their own square agree
// on the whole picture without a word between them.
//
// A part plays a minute. The wordmark opens the show and comes back every
// fifteenth part; between, the families of the rotation each play once a
// round, in an order of the round's own and with a seed of their own: a
// landscape at dawn in one round, a canyon by moonlight in the next. A tile
// changes part at its own moment, the top left first, with a flash, so the
// change crosses the wall from machine to machine.
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

// PartLen is how long a part plays, in seconds.
const PartLen = 60.0

// logoEvery: the wordmark is every fifteenth part, the first among them.
const logoEvery = 15

// wipe is how long a change of part takes to cross the wall; flashFor is
// how long a tile's flash lasts when it changes.
const (
	wipe     = 2.4
	flashFor = 0.45
)

// Frame is what a tile knows of a moment: the show's clock, in seconds
// since the show began, and the picture's shape. CutPart is the part the
// wall's controls last moved the show to the start of: that change does
// not wait for each tile's turn (the part before it is not what was
// playing), the flash alone still crossing the wall. 0, the zero value,
// is no cut: part 0 has no part before it. Quiet is the page's text
// turned off: the scrollers say nothing.
type Frame struct {
	T          float64
	Cols, Rows int
	W, H       int
	CutPart    int64
	Quiet      bool
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
	{"bits", newBits},
	{"land", newLand},
	{"day", newDay},
	{"space", newSpace},
	{"shapes", newShapes},
	{"mandel", newMandel},
	{"julia", newJulia},
	{"tunnel", newTunnel},
	{"plasma", newPlasma},
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

// NextOf is the first part after p that plays the family named name, or
// -1 when none does within limit parts (a round plays every family, so
// twenty finds any of them).
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
	p := int64(t / PartLen)
	local := t - float64(p)*PartLen
	// The tile's own moment of change: the top left first, the bottom
	// right last, a clone's place below the picture with the last row.
	at := float64(col+min(row, f.Rows-1)) / float64(max(f.Cols+f.Rows-2, 1)) * wipe
	flash := 0.0
	if p > 0 && local < at && f.CutPart != p {
		p, local = p-1, local+PartLen
	} else if p > 0 && local >= at && local-at < flashFor {
		flash = 1 - (local-at)/flashFor
	}
	v := &view{x0: col * f.W, y0: row * f.H, w: f.W, h: f.H, pw: f.Cols * f.W, ph: f.Rows * f.H, quiet: f.Quiet}
	if s.sc == nil || s.part != p || s.cols != f.Cols || s.rows != f.Rows || s.w != f.W || s.h != f.H {
		fam, seed := partOf(p)
		s.sc = fam.build(seed, v)
		s.part, s.cols, s.rows, s.w, s.h = p, f.Cols, f.Rows, f.W, f.H
	}
	v.px = make([]vec3, f.W*f.H)
	s.sc.frame(v, local)
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
	// quiet: the page's text is off, and the scrollers say nothing.
	quiet bool
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
