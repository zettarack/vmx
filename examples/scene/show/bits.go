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
