package show

import "math"

// live is the wordmark's lit green (#5fff00) as light, for the classic look.
var live = v3(0.13, 1.1, 0.0)

// field is bits going through the picture the way zxp's pool wall draws
// a pool's I/O: a digit to a column, each at a pace of its own, down for a
// write and up for a read, fading in and out at the picture's ends; the
// columns in layers, the far ones small, dim and slow. With a trail, each
// head leads a run of digits that fade behind it and change as they go.
type field struct {
	seed    uint64
	layers  []layer
	trail   int
	up      float64 // the share of columns that read: their bits rise
	breathe float64 // how far a column's light swells and fades
	wave    float64 // a burst's crossing speed, columns a second; 0: none
}

type layer struct {
	s     int     // a digit's cell, in pixels
	gap   int     // a column's width, in pixels
	speed float64 // picture heights a second
	light float64
	per   int // digits in a column at once
}

// draw adds the field's light to the tile at t, scaled by gain.
func (f *field) draw(v *view, t, gain float64) {
	for li := range f.layers {
		L := &f.layers[li]
		gw, gh := 5*L.s, 7*L.s
		span := float64(v.ph + gh)
		first := max((v.x0-L.gap)/L.gap, 0)
		last := (v.x0 + v.w) / L.gap
		for k := first; k <= last; k++ {
			x := k*L.gap + (L.gap-gw)/2
			if x+gw <= v.x0 || x >= v.x0+v.w {
				continue
			}
			ck := uint64(k)*0x9e3779b1 + uint64(li)<<40
			swell := 1.0
			if f.breathe > 0 {
				s := smooth(-0.35, 0.55, noise1(float64(k)*0.23+t*0.11, f.seed+uint64(li)))
				if f.wave > 0 {
					// A burst crossing the wall: the columns it passes light up.
					front := math.Mod(t*f.wave, float64(v.pw/L.gap)+40) - 20
					s = math.Max(s*0.35, math.Exp(-sq((float64(k)-front)/6)))
				}
				swell = 1 - f.breathe + f.breathe*s
			}
			rise := h01(ck, f.seed+11) < f.up
			for b := 0; b < L.per; b++ {
				cb := ck*31 + uint64(b)
				pace := 0.75 + 0.5*h01(cb, f.seed+1)
				run := t*L.speed*pace + h01(cb, f.seed+2)
				n := math.Floor(run)
				u := run - n
				top := -float64(gh) + u*span
				step := -(gh + L.s)
				if rise {
					top = float64(v.ph) - u*span
					step = -step
				}
				l := L.light * gain * swell * smooth(0, 0.08, u) * smooth(1, 0.9, u)
				if l < 0.004 {
					continue
				}
				d := mix64(cb^uint64(n)*0x51ed27) & 1
				head := live.scale(l * 0.8).add(gray(l * 0.5))
				v.stamp(&digits[d], x, int(top), L.s, head)
				for i := 1; i <= f.trail; i++ {
					tl := l * math.Pow(1-float64(i)/float64(f.trail+1), 1.5) * 0.75
					// A trail's digits change as it runs, each at its own beat.
					dd := mix64(cb+uint64(i)*977+uint64(t*(4+3*h01(cb, uint64(i))))) & 1
					v.stamp(&digits[dd], x, int(top)+i*step, L.s, live.scale(tl))
				}
			}
		}
	}
}

// bits is the field alone, with a floor of light lines it falls into: the
// pool wall's I/O as a scene, in one of three ways (zxp's own, rain, a
// storm of bursts).
type bits struct {
	f     *field
	floor bool
	seed  uint64
	line  string
}

func newBits(seed uint64, v *view) scene {
	b := &bits{seed: seed, floor: h01(seed, 1) < 0.6, line: scroll(seed)}
	switch mix64(seed) % 3 {
	case 0: // zxp's own: sparse columns both ways, three depths
		b.f = &field{seed: seed, up: 0.4, breathe: 0.65, layers: []layer{
			{s: 1, gap: 11, speed: 0.07, light: 0.22, per: 2},
			{s: 2, gap: 19, speed: 0.13, light: 0.5, per: 1},
			{s: 3, gap: 31, speed: 0.21, light: 1.0, per: 1},
		}}
	case 1: // rain: dense columns, each head trailing its run
		b.f = &field{seed: seed, trail: 9 + int(h01(seed, 2)*7), breathe: 0.35, layers: []layer{
			{s: 1, gap: 8, speed: 0.22, light: 0.28, per: 2},
			{s: 2, gap: 15, speed: 0.36, light: 0.85, per: 1},
		}}
	default: // a storm: bursts of I/O cross the wall
		b.f = &field{seed: seed, up: 0.5, breathe: 0.9, wave: 9 + 6*h01(seed, 3), layers: []layer{
			{s: 1, gap: 10, speed: 0.1, light: 0.3, per: 2},
			{s: 2, gap: 17, speed: 0.2, light: 0.75, per: 2},
		}}
	}
	return b
}

func (b *bits) frame(v *view, t float64) {
	const horizon = -0.18
	v.each(func(x, y float64) vec3 {
		c := v3(0.002, 0.006, 0.004).scale(1 + 2*smooth(0.5, -0.5, y))
		if !b.floor || y > horizon {
			return c
		}
		// A floor of light lines going away, the bits' ground.
		z := 0.12 / (horizon - y)
		gx, gz := x*z*6, z*6-t*0.6
		lx := math.Exp(-sq(fract(gx+0.5)-0.5) * 900)
		lz := math.Exp(-sq(fract(gz)-0.5) * 900)
		fog := math.Exp(-z * 0.35)
		return c.add(live.scale(0.25 * (lx + lz) * fog))
	})
	b.f.draw(v, t, 1)
	if b.floor {
		// Along the far edge of the floor, quiet: what is true here.
		s := max(1, v.ph/300)
		v.scroller(b.line, int(float64(v.ph)*(0.5-horizon))-7*s-4, s, 70, 3, t, live.scale(0.32))
	}
}
