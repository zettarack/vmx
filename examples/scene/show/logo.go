package show

import "math"

// logo is the wordmark made of bits: the bits fall out of the dark, each
// lit cell of VMX catches one (the bottom rows first), and the word holds
// while light runs across it and its digits flip, zettarack.com typed out
// under it, and at the end its bits fall away into the rain. It says
// nothing more: a scroller of what vmx does, then cards of it on the
// tiles, were each taken out (2026-11-19: "the text on the tiles with the
// features really sucks"). A word with a t has the drip from its foot, as
// the zettarack wordmark does.
type logo struct {
	cs, ds int // a cell's pixels; a digit's own cell in it
	ox, oy int // the wordmark's top left in the picture
	cells  []lit
	drips  []drip
	rain   *field
	seed   uint64
	// The tagline's left, its top and its font's cell.
	tx, ty, ts int
}

type lit struct {
	gx, gy int     // the cell's top left in the picture
	arrive float64 // when its bit lands
	fall   float64 // how long it falls
	from   float64 // how far above the picture it falls from, pixels
	leave  float64 // when it falls away
	salt   uint64
}

type drip struct {
	gx, gy     int // where it hangs: under the foot of a t
	every, off float64
}

func newLogo(seed uint64, v *view) scene {
	across := len(word)*6 - 1
	cs := max(4, min(v.pw*84/100/across, v.ph*34/100/7))
	// A digit fills most of its cell's height.
	lg := &logo{cs: cs, ds: max(1, int(float64(cs)*0.72/7+0.5)), ox: (v.pw - across*cs) / 2, oy: v.ph*42/100 - 7*cs/2, seed: seed}
	lg.ts = max(2, cs/7)
	lg.tx, lg.ty = (v.pw-(len(tagline)*6*lg.ts-lg.ts))/2, lg.oy+7*cs+cs*6/5
	for li := 0; li < len(word); li++ {
		g := glyphFor(word[li])
		for r := 0; r < 7; r++ {
			for c := 0; c < 5; c++ {
				if !g.on(c, r) {
					continue
				}
				salt := mix64(seed ^ uint64(li*64+r*8+c+1))
				lg.cells = append(lg.cells, lit{
					gx: lg.ox + (li*6+c)*cs, gy: lg.oy + r*cs,
					arrive: 2.4 + float64(6-r)*0.62 + float64(li)*0.11 + 0.9*h01(salt, 1),
					fall:   1.0 + 0.7*h01(salt, 2),
					from:   float64(cs) * (2 + 12*h01(salt, 3)),
					leave:  IntroLen - 9.5 + float64(6-r)*0.22 + float64(li)*0.07 + 0.9*h01(salt, 4),
					salt:   salt,
				})
			}
		}
		if word[li] == 't' {
			lg.drips = append(lg.drips, drip{
				gx: lg.ox + (li*6+3)*cs + cs/2, gy: lg.oy + 7*cs,
				every: 5.5 + 2*h01(seed, uint64(li)), off: 5 * h01(seed, uint64(li+9)),
			})
		}
	}
	lg.rain = &field{seed: seed, up: 0.3, breathe: 0.5, layers: []layer{
		{s: 1, gap: 12, speed: 0.06, light: 0.12, per: 2},
		{s: 2, gap: 23, speed: 0.11, light: 0.2, per: 1},
	}}
	return lg
}

func (lg *logo) frame(v *view, t float64) {
	// The ground: a dim liquid of light, barely moving, as the desktop's
	// own wallpaper.
	v.each(func(x, y float64) vec3 {
		n := fbm(x*1.3+t*0.015, y*1.3-t*0.01, 3, lg.seed)
		return v3(0.004, 0.009, 0.007).scale(1.2 + n)
	})
	// The rain thins while the word holds, and comes back as it falls.
	lg.rain.draw(v, t, 0.45+0.55*smooth(9, 2, t)+0.55*smooth(IntroLen-11, IntroLen-6, t))
	cs, ds := lg.cs, lg.ds
	dw, dh := 5*ds, 7*ds
	mid := float64(lg.oy) + 3.5*float64(cs)
	width := float64((len(word)*6 - 1) * cs)
	for i := range lg.cells {
		c := &lg.cells[i]
		start := c.arrive - c.fall
		if t < start {
			continue
		}
		y := float64(c.gy)
		light := 1.0
		switch {
		case t < c.arrive:
			// Falling into its cell, faster as it goes.
			u := (t - start) / c.fall
			y = lerp(-c.from, float64(c.gy), u*u)
			light = 0.85
		case t >= c.leave:
			// Falling away.
			d := t - c.leave
			y = float64(c.gy) + 700*d*d
			light = math.Max(0, 1-d*0.9)
			if y > float64(v.ph) || light <= 0 {
				continue
			}
		}
		if !v.overlaps(c.gx-cs, int(y)-cs, 3*cs, 3*cs) {
			continue
		}
		held := t >= c.arrive && t < c.leave
		if held {
			light += 2.2 * math.Exp(-(t-c.arrive)*7) // the landing's flash
			// The light running across the word, every seven seconds.
			if t > 9 {
				run := math.Mod(t-9, 7) / 7
				at := lerp(-0.2, 1.2, run)*width + float64(lg.ox)
				cx := float64(c.gx) + float64(cs)/2 + (float64(c.gy)-mid)*0.6
				light += 1.4 * math.Exp(-sq((cx-at)/(1.6*float64(cs))))
			}
		}
		// Its digit: changing as it falls; held, a flip now and then.
		d := c.salt & 1
		if !held {
			d = uint64(t*9+float64(c.salt%7)) & 1
		} else if h01(c.salt, uint64(t/2.3+float64(c.salt%5))) < 0.12 {
			d ^= 1
		}
		gy := int(y)
		cx, cy := c.gx+cs/2, gy+cs/2
		col := live.scale(light)
		if held {
			v.box(c.gx+1, gy+1, cs-2, cs-2, col.scale(0.16))
		}
		v.glow(float64(cx), float64(cy), float64(cs)*1.3, col.scale(0.10))
		v.stamp(&digits[d], cx-dw/2, cy-dh/2, ds, col.add(gray(0.25*light)))
	}
	// Under the word, once it has landed, the site typed out, a cursor
	// blinking after it; it goes with the word. It is the picture's, as
	// every word a scene writes is: the page's Text off leaves it.
	if t > 8.5 && t < IntroLen-9.5 {
		fade := smooth(IntroLen-9.5, IntroLen-10.5, t)
		n := min(len(tagline), int((t-8.5)/0.12)+1)
		for i := 0; i < n; i++ {
			g := glyphFor(tagline[i])
			v.stamp(&g, lg.tx+i*6*lg.ts, lg.ty, lg.ts, live.scale(0.9*fade))
		}
		if n < len(tagline) || math.Mod(t, 1) < 0.5 {
			v.box(lg.tx+n*6*lg.ts, lg.ty, 5*lg.ts, 7*lg.ts, live.scale(0.5*fade))
		}
	}
	// The drips from the two t's: one forms under the foot, hangs, and
	// falls to the bottom, faint.
	for _, d := range lg.drips {
		if t < 12 || t > IntroLen-10 {
			continue
		}
		u := math.Mod(t-12+d.off, d.every)
		x := float64(d.gx)
		switch {
		case u < 1.4: // forming
			k := u / 1.4
			v.glow(x, float64(d.gy)+float64(cs)*0.3*k, float64(cs)*0.35*k+1, live.scale(0.5*k))
		default: // falling
			f := u - 1.4
			y := float64(d.gy) + float64(cs)*0.3 + 600*f*f
			if y < float64(v.ph) {
				v.glow(x, y, float64(cs)*0.3, live.scale(0.45*math.Max(0, 1-f)))
				v.glow(x, y-float64(cs)*0.5, float64(cs)*0.2, live.scale(0.2*math.Max(0, 1-f)))
			}
		}
	}
}
