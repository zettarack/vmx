package show

import "math"

// tunnel is a flight down a winding tube, raymarched: the camera keeps to
// its middle and turns with it, so the tube closes round and its far end
// is always round a bend. Its walls are panels of metal with their seams,
// dark metal with rings of light going by, a lattice of hexagonal plates
// with their seams lit, or rock; a lamp with the rider lights what is
// near, two lights run on ahead, and the rest is dark.
type tunnel struct {
	kind           int
	speed, radius  float64
	ax, ay, fx, fy float64 // the path's sway, across and up
	pal            palette
	seed           uint64
	segs           float64 // panels round the tube
	light          vec3    // the lights' colour
}

func newTunnel(seed uint64, v *view) scene {
	tu := &tunnel{kind: int(mix64(seed) % 4), speed: 3.5 + 2*h01(seed, 1), radius: 1.0,
		ax: 1.2 + 0.8*h01(seed, 2), ay: 0.6 + 0.6*h01(seed, 3), fx: 0.13 + 0.06*h01(seed, 4), fy: 0.09 + 0.05*h01(seed, 5),
		pal: palFor(mix64(seed + 7)), seed: seed, segs: float64(12 + 4*int(h01(seed, 6)*3))}
	tu.light = tu.pal.at(0.55).add(gray(0.3))
	return tu
}

// mid is the tube's middle at depth z.
func (tu *tunnel) mid(z float64) (float64, float64) {
	return tu.ax*math.Sin(z*tu.fx) + 0.35*tu.ax*math.Sin(z*tu.fx*2.3+1.3), tu.ay * math.Cos(z*tu.fy)
}

// wall is how far p is from the tube's wall, from inside: the tube's
// radius less p's distance from its middle, the wall's own relief added.
func (tu *tunnel) wall(p vec3) float64 {
	cx, cy := tu.mid(p.z)
	dx, dy := p.x-cx, p.y-cy
	d := tu.radius - math.Hypot(dx, dy)
	switch tu.kind {
	case 0: // the panels stand a little proud of their seams
		u := (math.Atan2(dy, dx)/(2*math.Pi) + 0.5) * tu.segs
		d += 0.025 * (smooth(0, 0.06, fract(u)) * smooth(1, 0.94, fract(u)) * smooth(0, 0.06, fract(p.z*0.5)) * smooth(1, 0.94, fract(p.z*0.5)))
	case 1: // ribs every two units
		d += 0.08 * math.Pow(math.Max(0, math.Cos(p.z*math.Pi)), 16)
	case 3: // rock: the wall pushed in and out, no seam round the tube
		d += 0.16 * fbm(dx*1.7+p.z*1.1, dy*1.7-p.z*0.9, 2, tu.seed)
	}
	return d
}

func (tu *tunnel) frame(v *view, t float64) {
	z := t * tu.speed
	ex, ey := tu.mid(z)
	eye := v3(ex, ey, z)
	tx, ty := tu.mid(z + 2.5)
	f := v3(tx, ty, z+2.5).sub(eye).norm()
	r := f.cross(v3(0, 1, 0)).norm()
	u := r.cross(f)
	// Two lights run on ahead of the rider.
	var lights [2]vec3
	for i := range lights {
		lz := z + 5 + 7*float64(i) + 2*math.Sin(t*0.7+float64(i)*2)
		lx, ly := tu.mid(lz)
		lights[i] = v3(lx, ly+0.3*math.Sin(t*1.1+float64(i)), lz)
	}
	v.each(func(x, y float64) vec3 {
		rd := f.scale(1.25).add(r.scale(-x)).add(u.scale(y)).norm()
		// The march: a step the wall's distance at most, shorter in rock,
		// whose distance is not exact; past far, the fog has it all.
		const far = 24.0
		step := 0.85
		if tu.kind == 3 {
			step = 0.6
		}
		d := 0.05
		hit := false
		for i := 0; i < 64 && d < far; i++ {
			h := tu.wall(eye.add(rd.scale(d)))
			if h < 0.002*d {
				hit = true
				break
			}
			d += h * step
		}
		// Past far, the fog alone. A march that ran out of steps short of
		// it is a ray along the wall, the wall just there: shaded where it
		// stopped (a far end in jags of black, 2026-11-19, where it was not).
		fog := v3(0.004, 0.005, 0.008)
		if !hit && d >= far {
			return fog
		}
		p := eye.add(rd.scale(d))
		const e = 0.004
		a := tu.wall(p.add(v3(e, -e, -e)))
		b := tu.wall(p.add(v3(-e, -e, e)))
		c0 := tu.wall(p.add(v3(-e, e, -e)))
		c1 := tu.wall(p.add(v3(e, e, e)))
		n := v3(a-b-c0+c1, -a-b+c0+c1, -a+b-c0+c1).norm()
		alb, emit := tu.surface(p)
		// The rider's lamp, and the lights ahead: each falling off with
		// its distance, with a shine on the metal.
		toEye := eye.sub(p)
		de := toEye.length()
		l := toEye.scale(1 / de)
		c := alb.scale(1.3 * math.Max(n.dot(l), 0) / (1 + de*de*0.04))
		c = c.add(gray(0.35 * math.Pow(math.Max(n.dot(l), 0), 40) / (1 + de*de*0.04)))
		for _, lp := range lights {
			lv := lp.sub(p)
			dl := lv.length()
			c = c.add(alb.mul(tu.light).scale(2.2 * math.Max(n.dot(lv.scale(1/dl)), 0) / (1 + dl*dl*0.35)))
		}
		c = c.add(emit)
		return mix(c, fog, 1-math.Exp(-d*0.13))
	})
}

// surface is the wall's own colour at p, and the light it gives.
func (tu *tunnel) surface(p vec3) (vec3, vec3) {
	cx, cy := tu.mid(p.z)
	dx, dy := p.x-cx, p.y-cy
	u := (math.Atan2(dy, dx)/(2*math.Pi) + 0.5) * tu.segs
	switch tu.kind {
	case 0: // panels, each its own tone, a strip of light on a few
		iu, iz := math.Floor(u), math.Floor(p.z*0.5)
		cell := uint64(int64(iu))*0x9e3779b1 ^ uint64(int64(iz))*0x51ed27 ^ tu.seed
		alb := tu.pal.at(0.12 + 0.1*h01(cell, 1)).scale(0.35 + 0.25*h01(cell, 2))
		var emit vec3
		if h01(cell, 3) < 0.1 {
			emit = tu.light.scale(1.6 * math.Exp(-sq((fract(p.z*0.5)-0.5)/0.05)))
		}
		return alb, emit
	case 1: // dark metal, a ring of light on every fourth rib
		alb := gray(0.18 + 0.05*perlin(u*2, p.z*1.5, tu.seed))
		ring := math.Exp(-sq((fract(p.z/8+0.5) - 0.5) * 8 / 0.2))
		return alb, tu.light.scale(2.5 * ring)
	case 2: // a honeycomb of plates, their seams lit
		lx, ly, ix, iy := hexCell(u, p.z)
		// The plate's number wraps round the tube with it, or the one that
		// straddles where the angle turns over has two colours.
		cell := uint64(int64(math.Mod(ix*2, tu.segs*2)))*0x9e3779b1 ^ uint64(int64(iy*2))*0x51ed27
		return tu.pal.at(0.2 + 0.12*h01(cell, tu.seed)).scale(0.45), tu.light.scale(1.1 * smooth(0.43, 0.49, hexDist(lx, ly)))
	default: // rock, in bands of its own
		n := fbm(dx*3+p.z*1.4, dy*3-p.z*0.8, 3, tu.seed+3) + 0.5*fbm(dy*7-p.z*2.5, dx*7, 2, tu.seed+4)
		return mix(v3(0.13, 0.1, 0.08), v3(0.42, 0.37, 0.31), sat(0.5+0.5*n)), vec3{}
	}
}

// hexCell is the honeycomb's cell that (x, y) is in, spacing 1 across:
// the point from the cell's middle, and the middle.
func hexCell(x, y float64) (lx, ly, cx, cy float64) {
	const s = 1.7320508
	// Two lattices of middles, one offset by half a cell each way; the
	// nearer middle of the two is the cell's.
	ax, ay := math.Floor(x)+0.5, math.Floor(y/s)+0.5
	bx, by := math.Floor(x-0.5)+0.5, math.Floor((y-1)/s)+0.5
	h1x, h1y := x-ax, y-ay*s
	h2x, h2y := x-(bx+0.5), y-(by+0.5)*s
	if h1x*h1x+h1y*h1y < h2x*h2x+h2y*h2y {
		return h1x, h1y, ax, ay
	}
	return h2x, h2y, bx + 0.5, by + 0.5
}

// hexDist is how far a point is from its cell's middle, in the hexagon's
// own measure: 0 there, 0.5 at the cell's edge.
func hexDist(x, y float64) float64 {
	ax, ay := math.Abs(x), math.Abs(y)
	return math.Max(ax*0.5+ay*0.8660254, ax)
}
