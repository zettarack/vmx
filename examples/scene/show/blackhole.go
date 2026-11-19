package show

import "math"

// blackhole is a black hole and the disk of gas going round it, drawn the
// way its light comes: every ray from the eye is bent by the hole on its
// way, so the disk's far side shows over the hole's top and under its foot
// as well as behind it, the stars and the band of the galaxy behind are
// drawn round its edge into rings, and a ray that comes too near falls in
// and is black. The gas is hottest, brightest and fastest at its inner
// edge, and the side of it coming toward the eye is the brighter and the
// bluer. On some parts jets stand out of the poles. Over a part the eye
// goes some of the way round the hole, rising over the disk and coming in
// toward it by the part's middle, and going back out.
//
// The units are the hole's own: nothing that comes inside a radius of 1
// leaves, light can go round it at 1.5, and gas can go round it no nearer
// than 3, where the disk begins. A ray's path is followed by its bending
// alone, the rule that gives those three radii (in Schwarzschild's
// geometry, a ray's path bends as a body's would under a pull of 3/2 h^2
// over the fourth power of its distance, h its angular momentum).
type blackhole struct {
	seed      uint64
	in, out   float64 // the disk's inner and outer radius
	far, near float64 // the eye's distance at the part's ends and middle
	low, high float64 // its height over the disk, in radians, at the ends and middle
	az0, turn float64 // where round the hole it starts, and how far it goes in a part
	roll      float64
	spin      float64 // the disk's way round: 1 or -1
	hot, cool vec3    // the gas's colour at its inner edge and toward its outer
	jet       float64 // the jets' brightness, 0 for none
	band      vec3    // the galaxy's plane, by its normal
	neb       palette
	px        float64 // a pixel's angle, at the picture's middle
}

// gases are the colours a disk can burn in: its inner edge, its outer.
var gases = [][2]vec3{
	{v3(1, 0.93, 0.82), v3(1, 0.42, 0.1)},     // white to orange
	{v3(0.85, 0.92, 1), v3(0.3, 0.45, 1)},     // white to blue
	{v3(1, 0.95, 0.72), v3(0.85, 0.5, 0.12)},  // gold
	{v3(1, 0.88, 0.95), v3(0.75, 0.22, 0.55)}, // rose
}

func newBlackhole(seed uint64, v *view) scene {
	b := &blackhole{seed: seed}
	b.in = 3 + 0.4*h01(seed, 1)
	b.out = 9 + 5*h01(seed, 2)
	b.far = 20 + 6*h01(seed, 3)
	b.near = 14 + 4*h01(seed, 4)
	b.low = 0.05 + 0.08*h01(seed, 5)
	b.high = 0.3 + 0.35*h01(seed, 6)
	b.az0 = 2 * math.Pi * h01(seed, 7)
	b.turn = (0.6 + 0.8*h01(seed, 8)) * math.Pi
	if h01(seed, 9) < 0.5 {
		b.turn = -b.turn
	}
	b.roll = 0.5 * (h01(seed, 10) - 0.5)
	b.spin = 1
	if h01(seed, 11) < 0.5 {
		b.spin = -1
	}
	g := gases[int(mix64(seed+12)%uint64(len(gases)))]
	b.hot, b.cool = g[0], g[1]
	if h01(seed, 13) < 0.35 {
		b.jet = 0.5 + 0.5*h01(seed, 14)
	}
	b.band = v3(2*h01(seed, 15)-1, 2*h01(seed, 16)-1, 2*h01(seed, 17)-1).add(v3(0, 0.3, 0)).norm()
	b.neb = palFor(mix64(seed + 18))
	return b
}

// eye is where the eye is at t into the part, and the picture's right, up
// and ahead from it.
func (b *blackhole) eye(t float64) (e, r, u, f vec3) {
	w := 0.5 - 0.5*math.Cos(2*math.Pi*t/SceneLen) // 0 at the part's ends, 1 in its middle
	d := lerp(b.far, b.near, w)
	el := lerp(b.low, b.high, w)
	az := b.az0 + b.turn*t/SceneLen
	e = v3(math.Cos(el)*math.Cos(az), math.Sin(el), math.Cos(el)*math.Sin(az)).scale(d)
	f = e.scale(-1).norm()
	r = f.cross(v3(0, 1, 0)).norm()
	u = r.cross(f)
	sn, cs := math.Sincos(b.roll + 0.04*math.Sin(t*0.05))
	return e, r.scale(cs).add(u.scale(sn)), u.scale(cs).sub(r.scale(sn)), f
}

func (b *blackhole) frame(v *view, t float64) {
	e, r, u, f := b.eye(t)
	b.px = 1 / (1.05 * float64(v.ph))
	v.each(func(x, y float64) vec3 {
		d := r.scale(x).add(u.scale(y)).add(f.scale(1.05)).norm()
		return b.trace(e, d, t).add(b.jets(e, d))
	})
}

// trace is the light that comes to the eye at p along d.
func (b *blackhole) trace(p, d vec3, t float64) vec3 {
	col, clear, out, gone := b.follow(p, d, t)
	if gone {
		col = col.add(b.sky(out).scale(clear))
	}
	return col
}

// pull is how the hole bends a ray at p whose angular momentum, squared,
// is h2: toward the middle, 3/2 h2 over the distance to the fourth; and
// the distance.
func pull(p vec3, h2 float64) (vec3, float64) {
	r2 := p.dot(p)
	r := math.Sqrt(r2)
	return p.scale(-1.5 * h2 / (r2 * r2 * r)), r
}

// follow follows the ray from p along d back to where its light came
// from: the disk, each time the ray crosses it (the gas there, over what
// of it is clear), and the light gathered, how much of what lies beyond
// still shows, and, when the ray gets away rather than falling in, the way
// it leaves.
//
// Every ray is followed from the eye until it is on its way out three
// times as far as the disk's edge, and only then bent the rest of its way
// by rule, where the rest is a few thousandths of what it was. Rays that
// passed wide were once bent by a rule alone from the eye, a rule for
// another pull, followed with a first-order step and handed to the rule at
// the disk's edge: where the rule's error met the following, the way out
// jumped by half a pixel and more, and the stars behind the hole showed
// rings of doubled stars (2026-11-18). The step is velocity Verlet now,
// the rule is the same pull's (see bend), and the hand-over is far out.
func (b *blackhole) follow(p, d vec3, t float64) (col vec3, clear float64, out vec3, gone bool) {
	h2 := p.cross(d).dot(p.cross(d)) // the ray's angular momentum, squared: kept all the way
	clear = 1.0
	went := 0.0 // how far the ray has come, for how much of the disk a pixel takes in
	acc, r := pull(p, h2)
	for i := 0; i < 240; i++ {
		if r < 1 {
			return col, clear, vec3{}, false
		}
		if r > b.out*3 && p.dot(d) > 0 {
			// On its way out, far past the disk: nothing more to meet.
			dn := d.norm()
			return col, clear, bend(p, dn, p.cross(dn).length()), true
		}
		// A step of a fourteenth of the distance to the middle (a fifth,
		// past the disk), however fast the path's measure runs: the bending
		// is sharpest near the hole.
		step := (0.07 + 0.13*smooth(b.out, b.out*2, r)) * r
		dt := step / d.length()
		went += step
		half := d.add(acc.scale(dt / 2))
		p2 := p.add(half.scale(dt))
		acc2, r2 := pull(p2, h2)
		d2 := half.add(acc2.scale(dt / 2))
		if (p.y > 0) != (p2.y > 0) {
			k := p.y / (p.y - p2.y)
			q := p.add(p2.sub(p).scale(k))
			if rr := math.Hypot(q.x, q.z); rr > b.in*0.9 && rr < b.out*1.1 {
				// A pixel's width on the disk across its rings: wider the
				// farther the ray has come and the flatter it meets the disk.
				foot := went * b.px / math.Max(math.Abs(d2.y)/d2.length(), 0.05)
				c, a := b.gas(q, rr, d2, foot, t)
				col = col.add(c.scale(clear))
				clear *= 1 - a
				if clear < 0.02 {
					return col, clear, vec3{}, false
				}
			}
		}
		p, d, acc, r = p2, d2, acc2, r2
	}
	// Still going round after all those steps: as near the edge as makes
	// no difference, and into the hole.
	return col, clear, vec3{}, false
}

// bend is the way a ray at p going d (of length 1) leaves for good: turned
// toward the hole by what the hole has still to bend it. Under pull, a ray
// passing wide of the hole by w is bent 2/w + 15 pi/(16 w^2) in all (the
// first two terms, in the hole's units), half on each half of its way
// past; along a straight line the share of it still to come, from a point
// whose way out makes cosine c with the line from the middle, is
// 1 - c(3 - c^2)/2 of a half, the integral of pull's 1/r^5 across the
// line (the first term's own share; the second is shared alike).
func bend(p, d vec3, wide float64) vec3 {
	if wide < 1e-9 {
		return d
	}
	c := p.dot(d) / p.length()
	rest := 1 - c*(3-c*c)/2 // 2 the whole way past, 1 from its nearest, 0 when gone
	a := math.Min(rest*(1/wide+15*math.Pi/(32*wide*wide)), 1.2)
	in := p.sub(d.scale(p.dot(d))).scale(-1).norm() // across the ray, toward the hole
	sn, cs := math.Sincos(a)
	return d.scale(cs).add(in.scale(sn))
}

// flowFor is how long the gas's clumps are drawn out before they are drawn
// afresh (see gas).
const flowFor = 24.0

// gas is the light the disk sends back along a ray that crosses it at q,
// rr from the middle, going d, where a pixel takes in foot of the disk
// across its rings; and how much of what lies behind it the gas there
// hides.
func (b *blackhole) gas(q vec3, rr float64, d vec3, foot, t float64) (vec3, float64) {
	edge := smooth(b.in*0.92, b.in*1.12, rr) * smooth(b.out*1.08, b.out*0.55, rr)
	if edge <= 0 {
		return vec3{}, 0
	}
	// keep is how much of a pattern that changes rate times a unit of
	// radius a pixel can show: all of it while a feature is two pixels
	// wide, none under one, so the rings do not shimmer where the disk is
	// seen nearly edge on.
	keep := func(rate float64) float64 { return sat(1.5 - 2*rate*foot) }
	// The lanes: rings of thicker and thinner gas, by the radius alone.
	lanes, amp, sum := 0.0, 1.0, 0.0
	x, rate := math.Log(rr)*14, 14/rr
	for o := 0; o < 3; o++ {
		lanes += amp * keep(rate) * noise1(x, b.seed+20+uint64(o))
		sum += amp
		amp, x, rate = amp*0.5, x*2.07, rate*2.07
	}
	lanes /= sum
	// The clumps go round with the gas, each ring at its own speed, the
	// inner ones fastest, so they draw out into streaks. Followed for ever
	// the drawing out makes them finer than a pixel in a minute (the first
	// cut shimmered by the part's middle, 2026-11-18), so they are drawn
	// twice, each drawn out for no more than flowFor seconds before it is
	// drawn afresh at a turn of its own, and the two are crossfaded, each at
	// its strongest when the other starts again.
	w := b.spin * 0.8 * math.Pow(b.in/rr, 1.5)
	phi := math.Atan2(q.z, q.x)
	l := math.Log(rr) * 7
	clumps := 0.0
	for k := 0; k < 2; k++ {
		c := t/flowFor + 0.5*float64(k)
		f := fract(c)
		n := uint64(int64(math.Floor(c)))*2 + uint64(k)
		a := phi - w*f*flowFor + 2*math.Pi*h01(b.seed, n)
		u := fract(a/(2*math.Pi)) * 16
		v := perlinP(u, l, 16, b.seed+n*0x9e37) + 0.5*keep(14/rr)*perlinP(u*2, l*2, 32, b.seed+n*0x9e37+7)
		clumps += (1 - math.Abs(2*f-1)) * v
	}
	dens := sat(0.5 + 0.8*lanes + 1.1*clumps)
	heat := math.Pow(b.in/rr, 1.25) // 1 at the inner edge
	// The gas's speed across the ray (toward the eye or away), and the
	// climb out of the hole's pull: light from the side coming toward the
	// eye arrives brighter and bluer, the other dimmer and redder.
	beta := math.Sqrt(0.5 / (rr - 1))
	toward := -v3(-q.z, 0, q.x).scale(b.spin / rr).dot(d.norm())
	g := math.Sqrt(1-beta*beta) / (1 - beta*toward) * math.Sqrt(1-1/rr)
	c := mix(b.cool, b.hot, sat(heat*g))
	light := (0.04 + 6*math.Pow(heat, 2.4)) * (0.3 + 0.9*dens) * math.Min(g*g*g, 4)
	alpha := sat(0.25+0.75*dens) * edge * 0.92
	return c.scale(light * alpha), alpha
}

// jets are the two beams out of the hole's poles, where a part has them,
// faint and blue-white, seen where the ray passes nearest the hole's axis.
// They are not bent: what is nearest the hole, where the bending is, is
// left out of them.
func (b *blackhole) jets(e, d vec3) vec3 {
	if b.jet == 0 {
		return vec3{}
	}
	across := d.x*d.x + d.z*d.z
	if across < 1e-9 {
		return vec3{}
	}
	s := -(e.x*d.x + e.z*d.z) / across
	if s <= 0 {
		return vec3{}
	}
	y := math.Abs(e.y + s*d.y)
	dist := math.Hypot(e.x+s*d.x, e.z+s*d.z)
	w := 0.22 + 0.07*y
	k := b.jet * math.Exp(-dist*dist/(w*w)) * smooth(2, 4.5, y) / (1 + 0.004*y*y)
	return v3(0.55, 0.72, 1).scale(k)
}

// sky is what lies behind, the way d (of length 1) looks: stars, one to a
// cell of a cube round the eye where there is one, and the band of the
// galaxy with its dust, faint.
func (b *blackhole) sky(d vec3) vec3 {
	ax, ay, az := math.Abs(d.x), math.Abs(d.y), math.Abs(d.z)
	var face uint64
	var s, w float64
	switch {
	case ax >= ay && ax >= az:
		face, s, w = 0, d.y/ax, d.z/ax
		if d.x < 0 {
			face = 1
		}
	case ay >= az:
		face, s, w = 2, d.x/ay, d.z/ay
		if d.y < 0 {
			face = 3
		}
	default:
		face, s, w = 4, d.x/az, d.y/az
		if d.z < 0 {
			face = 5
		}
	}
	const cells = 500.0
	h := lattice(int64(math.Floor((s+1)*cells)), int64(math.Floor((w+1)*cells)), b.seed+face*0x51ed)
	g := d.dot(b.band)
	inBand := math.Exp(-g * g / 0.035)
	col := vec3{}
	if h&255 < 3 || inBand > 0.3 && h&255 < 8 {
		k := float64(h>>8&255) / 255
		col = v3(0.8+0.2*k, 0.85, 1-0.2*k).scale(0.12 + 2.2*k*k*k)
	}
	if inBand > 0.01 {
		n := fbm(d.x*3+d.z*1.7, d.y*3-d.z*1.3, 4, b.seed+40)
		dust := smooth(-0.1, 0.5, fbm(d.x*7-d.y*2, d.z*7+d.y*2, 3, b.seed+41))
		col = col.add(b.neb.at(0.3 + 0.3*n).scale(0.09 * inBand * (0.6 + 0.6*n) * (1 - 0.75*dust)))
	}
	return col
}
