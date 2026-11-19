package show

import "math"

// vector is a flight over a plain drawn the way the old vector screens
// drew: lines of light on black, a glow round each, every edge of every
// shape seen. A helicopter of the scene's own, a utility helicopter after
// the UH-1, flies a winding course low over the plain, nose a little down
// in forward flight, rolling into each turn, climbing and sinking; the
// camera chases it, banking with it. Faint crosses mark the ground; blocks,
// pyramids and towers stand on it; a range of angular mountains runs round
// the horizon with a crescent moon over it.
type vector struct {
	seed  uint64
	speed float64 // the helicopter's, units a second
	ridge []float64
	moon  float64 // the moon's bearing
	ink   vec3
}

const (
	vecFocal = 1.1 // picture heights from the eye to the screen
	vecCell  = 9.0 // the plain's cells, each with a shape or none
	vecStep  = 0.05
)

func newVector(seed uint64, v *view) scene {
	vc := &vector{seed: seed, speed: 9 + 4*h01(seed, 1), moon: 2 * math.Pi * h01(seed, 3)}
	// The phosphor's colour: green mostly, red or white now and then.
	vc.ink = v3(0.3, 1.0, 0.42)
	switch h := h01(seed, 4); {
	case h < 0.15:
		vc.ink = v3(1.0, 0.35, 0.25)
	case h < 0.25:
		vc.ink = v3(0.8, 0.9, 1.0)
	}
	// The range: angular, a straight line from one point to the next, a
	// few peaks well above the rest.
	n := 64
	for i := 0; i < n; i++ {
		a := float64(i) / float64(n)
		h := 0.04 + 0.07*sat(0.5+0.6*fbm1(a*6, 3, seed+5)) + 0.025*h01(seed, uint64(200+i))
		if h01(seed, uint64(100+i)) < 0.08 {
			h += 0.06 + 0.07*h01(seed, uint64(300+i))
		}
		vc.ridge = append(vc.ridge, h)
	}
	return vc
}

// heading is the helicopter's course at t: a slow winding, two swings of
// their own lengths laid one on the other.
func (vc *vector) heading(t float64) float64 {
	p := float64(vc.seed % 97)
	return 0.9*math.Sin(t*0.09+p) + 0.45*math.Sin(t*0.23+p*1.7)
}

// flight is where the helicopter is at t, its heading, and how it sits in
// the air: nose down in forward flight, rolled into its turn as far as the
// turn is quick, its height rising and sinking. The course is the sum of
// its steps, so every tile works out the same place.
func (vc *vector) flight(t float64) (p vec3, yaw, pitch, roll float64) {
	n := int(t / vecStep)
	x, z := 0.0, 0.0
	for i := 0; i < n; i++ {
		s, c := math.Sincos(vc.heading(float64(i) * vecStep))
		x, z = x+s*vc.speed*vecStep, z+c*vc.speed*vecStep
	}
	last := t - float64(n)*vecStep
	yaw = vc.heading(t)
	s, c := math.Sincos(yaw)
	x, z = x+s*vc.speed*last, z+c*vc.speed*last
	turn := (vc.heading(t+0.2) - vc.heading(t-0.2)) / 0.4
	roll = math.Max(-0.6, math.Min(0.6, -turn*vc.speed*0.12))
	pitch = -0.1 - 0.03*math.Sin(t*0.31)
	return v3(x, 13+3.5*math.Sin(t*0.17), z), yaw, pitch, roll
}

// cam is a camera: where it is and its three ways, right, up and forward.
type cam struct {
	eye     vec3
	r, u, f vec3
}

// look is the camera at eye looking at target, turned by roll about its
// line of sight.
func look(eye, target vec3, roll float64) cam {
	f := target.sub(eye).norm()
	r := v3(0, 1, 0).cross(f).norm()
	u := f.cross(r)
	s, c := math.Sincos(roll)
	return cam{eye: eye, f: f, r: r.scale(c).add(u.scale(s)), u: u.scale(c).sub(r.scale(s))}
}

// project is a point of the world on the picture, in pixels, and how far
// in front of the camera it is.
func (c *cam) project(v *view, p vec3) (x, y, z float64) {
	d := p.sub(c.eye)
	z = d.dot(c.f)
	return d.dot(c.r)/z*vecFocal*float64(v.ph) + float64(v.pw)/2, (0.5 - d.dot(c.u)/z*vecFocal) * float64(v.ph), z
}

// toward is a direction on the picture, as a point infinitely far away is:
// false when it is behind the camera.
func (c *cam) toward(v *view, d vec3) (x, y float64, ok bool) {
	z := d.dot(c.f)
	if z < 0.05 {
		return 0, 0, false
	}
	return d.dot(c.r)/z*vecFocal*float64(v.ph) + float64(v.pw)/2, (0.5 - d.dot(c.u)/z*vecFocal) * float64(v.ph), true
}

// edge draws the world's line a-b, the part in front of the camera, its
// light falling off with its distance.
func (vc *vector) edge(v *view, c *cam, a, b vec3, light float64) {
	const near = 0.3
	za, zb := a.sub(c.eye).dot(c.f), b.sub(c.eye).dot(c.f)
	if za < near && zb < near {
		return
	}
	if za < near {
		a = mix(a, b, (near-za)/(zb-za))
	} else if zb < near {
		b = mix(b, a, (near-zb)/(za-zb))
	}
	x0, y0, z0 := c.project(v, a)
	x1, y1, z1 := c.project(v, b)
	v.line(x0, y0, x1, y1, vc.ink.scale(light*sat(1.4-math.Min(z0, z1)/90)))
}

// line draws a line of light from (x0, y0) to (x1, y1), the picture's
// pixels, a beam and the glow round it, added to what is there.
func (v *view) line(x0, y0, x1, y1 float64, c vec3) {
	const halo = 3.5
	ix0, ix1 := max(int(math.Min(x0, x1)-halo), v.x0), min(int(math.Max(x0, x1)+halo)+1, v.x0+v.w)
	iy0, iy1 := max(int(math.Min(y0, y1)-halo), v.y0), min(int(math.Max(y0, y1)+halo)+1, v.y0+v.h)
	if ix0 >= ix1 || iy0 >= iy1 {
		return
	}
	dx, dy := x1-x0, y1-y0
	l2 := dx*dx + dy*dy
	for y := iy0; y < iy1; y++ {
		py := float64(y) + 0.5
		row := v.px[(y-v.y0)*v.w:]
		for x := ix0; x < ix1; x++ {
			px := float64(x) + 0.5
			t := 0.0
			if l2 > 0 {
				t = sat(((px-x0)*dx + (py-y0)*dy) / l2)
			}
			ex, ey := px-(x0+t*dx), py-(y0+t*dy)
			d2 := ex*ex + ey*ey
			if d2 >= halo*halo {
				continue
			}
			row[x-v.x0] = row[x-v.x0].add(c.scale(math.Exp(-d2/0.45) + 0.22*math.Exp(-d2/4)))
		}
	}
}

// wirebox is the edges of a box from lo to hi turned by yaw about its
// middle.
func wirebox(lo, hi vec3, yaw float64) [][2]vec3 {
	mid := lo.add(hi).scale(0.5)
	s, c := math.Sincos(yaw)
	var p [8]vec3
	for i := range p {
		q := v3(lo.x, lo.y, lo.z)
		if i&1 != 0 {
			q.x = hi.x
		}
		if i&2 != 0 {
			q.y = hi.y
		}
		if i&4 != 0 {
			q.z = hi.z
		}
		dx, dz := q.x-mid.x, q.z-mid.z
		p[i] = v3(mid.x+dx*c+dz*s, q.y, mid.z-dx*s+dz*c)
	}
	var e [][2]vec3
	for i := 0; i < 8; i++ {
		for _, bit := range []int{1, 2, 4} {
			if i&bit == 0 {
				e = append(e, [2]vec3{p[i], p[i|bit]})
			}
		}
	}
	return e
}

// pyramid is the edges of a pyramid on a square base, turned by yaw.
func pyramid(at vec3, half, height, yaw float64) [][2]vec3 {
	s, c := math.Sincos(yaw)
	var b [4]vec3
	for i, d := range [][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
		dx, dz := d[0]*half, d[1]*half
		b[i] = v3(at.x+dx*c+dz*s, at.y, at.z-dx*s+dz*c)
	}
	top := at.add(v3(0, height, 0))
	var e [][2]vec3
	for i := range b {
		e = append(e, [2]vec3{b[i], b[(i+1)%4]}, [2]vec3{b[i], top})
	}
	return e
}

// heli is the scene's own utility helicopter, after the UH-1: a cabin
// with a sloped windscreen, a boom tapering to a fin, a stabiliser across
// the boom, a tail rotor, the main rotor's two long blades on their mast,
// and skids on struts. It sits at at, heading yaw, pitched and rolled, k
// times its own size, its rotors at angle spin.
func heli(at vec3, yaw, pitch, roll, spin, k float64) [][2]vec3 {
	sy, cy := math.Sincos(yaw)
	sp, cp := math.Sincos(pitch)
	sr, cr := math.Sincos(roll)
	pt := func(x, y, z float64) vec3 {
		x, y, z = x*k, y*k, z*k
		// Roll about the length, then pitch about the width, then yaw.
		x, y = x*cr-y*sr, x*sr+y*cr
		y, z = y*cp-z*sp, y*sp+z*cp
		return v3(at.x+x*cy+z*sy, at.y+y, at.z-x*sy+z*cy)
	}
	cab := []vec3{
		pt(-0.65, 0, -1.1), pt(0.65, 0, -1.1), pt(0.65, 0, 1.3), pt(-0.65, 0, 1.3),
		pt(-0.55, 1.05, -1.0), pt(0.55, 1.05, -1.0), pt(0.55, 1.05, 0.4), pt(-0.55, 1.05, 0.4),
	}
	var e [][2]vec3
	for i := 0; i < 4; i++ {
		e = append(e, [2]vec3{cab[i], cab[(i+1)%4]}, [2]vec3{cab[4+i], cab[4+(i+1)%4]}, [2]vec3{cab[i], cab[4+i]})
	}
	// The nose's chin, under the windscreen.
	e = append(e, [2]vec3{pt(-0.65, 0, 1.3), pt(0, -0.05, 1.7)}, [2]vec3{pt(0.65, 0, 1.3), pt(0, -0.05, 1.7)}, [2]vec3{pt(0, -0.05, 1.7), pt(0, 0.5, 1.0)})
	// The boom, narrowing to the fin, the stabiliser across it.
	e = append(e, [2]vec3{pt(0, 0.85, -1.0), pt(0, 1.0, -5.2)}, [2]vec3{pt(0, 0.35, -1.0), pt(0, 0.85, -5.0)},
		[2]vec3{pt(0, 1.0, -5.2), pt(0, 0.85, -5.0)})
	e = append(e, [2]vec3{pt(0, 1.0, -5.2), pt(0, 2.0, -5.7)}, [2]vec3{pt(0, 2.0, -5.7), pt(0, 1.8, -4.9)}, [2]vec3{pt(0, 1.8, -4.9), pt(0, 1.0, -4.7)})
	e = append(e, [2]vec3{pt(-0.9, 0.75, -3.4), pt(0.9, 0.75, -3.4)}, [2]vec3{pt(-0.9, 0.75, -3.4), pt(-0.9, 0.75, -3.0)}, [2]vec3{pt(0.9, 0.75, -3.4), pt(0.9, 0.75, -3.0)})
	// The tail rotor turns on the boom's left, faster than the main one.
	ts, tc := math.Sincos(spin * 2.7)
	e = append(e, [2]vec3{pt(-0.2, 1.5+0.6*tc, -5.4+0.6*ts), pt(-0.2, 1.5-0.6*tc, -5.4-0.6*ts)})
	// The mast and the main rotor's two blades, long as the helicopter.
	e = append(e, [2]vec3{pt(0, 1.05, -0.2), pt(0, 1.4, -0.2)})
	ms, mc := math.Sincos(spin)
	e = append(e, [2]vec3{pt(-3.6*mc, 1.4, -0.2-3.6*ms), pt(3.6*mc, 1.4, -0.2+3.6*ms)})
	// Skids on struts, turned up at the front.
	for _, x := range []float64{-0.8, 0.8} {
		e = append(e, [2]vec3{pt(x, -0.4, -1.0), pt(x, -0.4, 1.4)}, [2]vec3{pt(x, -0.4, 1.4), pt(x, -0.15, 1.7)},
			[2]vec3{pt(x*0.75, 0, -0.5), pt(x, -0.4, -0.5)}, [2]vec3{pt(x*0.75, 0, 0.8), pt(x, -0.4, 0.8)})
	}
	return e
}

func (vc *vector) frame(v *view, t float64) {
	hp, yaw, pitch, roll := vc.flight(t)
	fs, fc := math.Sincos(yaw)
	fwd, side := v3(fs, 0, fc), v3(fc, 0, -fs)
	// The camera chases from behind and to one side, a little above, so the
	// helicopter shows its side as it goes, looking past it down its
	// course, banking with it.
	eye := hp.sub(fwd.scale(11)).add(v3(0, 2.6, 0)).add(side.scale(6 + 2*math.Sin(t*0.07)))
	c := look(eye, hp.add(fwd.scale(7)).sub(v3(0, 1.2, 0)), roll*0.35)
	// The horizon, the mountains round it and the moon over it: so far off
	// they turn with the camera and never come nearer.
	horizon := func(b, el float64) vec3 {
		s, co := math.Sincos(b)
		return v3(s*math.Cos(el), math.Sin(el), co*math.Cos(el))
	}
	const ring = 128
	for i := 0; i < ring; i++ {
		b0, b1 := float64(i)/ring*2*math.Pi, float64(i+1)/ring*2*math.Pi
		if x0, y0, ok := c.toward(v, horizon(b0, 0)); ok {
			if x1, y1, ok := c.toward(v, horizon(b1, 0)); ok {
				v.line(x0, y0, x1, y1, vc.ink.scale(0.35))
			}
		}
	}
	n := len(vc.ridge)
	for i := 0; i < n; i++ {
		b0, b1 := float64(i)/float64(n)*2*math.Pi, float64(i+1)/float64(n)*2*math.Pi
		if x0, y0, ok := c.toward(v, horizon(b0, vc.ridge[i])); ok {
			if x1, y1, ok := c.toward(v, horizon(b1, vc.ridge[(i+1)%n])); ok {
				v.line(x0, y0, x1, y1, vc.ink.scale(0.55))
			}
		}
	}
	if mx, my, ok := c.toward(v, horizon(vc.moon, 0.3)); ok {
		r := 0.045 * float64(v.ph)
		for k := 0; k < 16; k++ {
			a0, a1 := math.Pi*0.15+float64(k)/16*math.Pi*1.7, math.Pi*0.15+float64(k+1)/16*math.Pi*1.7
			v.line(mx+r*math.Cos(a0), my-r*math.Sin(a0), mx+r*math.Cos(a1), my-r*math.Sin(a1), vc.ink.scale(0.7))
			v.line(mx+0.35*r+0.8*r*math.Cos(a0), my-0.8*r*math.Sin(a0), mx+0.35*r+0.8*r*math.Cos(a1), my-0.8*r*math.Sin(a1), vc.ink.scale(0.4))
		}
	}
	// Marks on the ground, a cross every five units round where the
	// camera looks, faint: the speed the plain shows.
	gx0, gz0 := hp.x+fwd.x*18, hp.z+fwd.z*18
	for gz := math.Floor((gz0-45)/5) * 5; gz <= gz0+45; gz += 5 {
		for gx := math.Floor((gx0-45)/5) * 5; gx <= gx0+45; gx += 5 {
			vc.edge(v, &c, v3(gx-0.3, 0, gz), v3(gx+0.3, 0, gz), 0.45)
			vc.edge(v, &c, v3(gx, 0, gz-0.3), v3(gx, 0, gz+0.3), 0.45)
		}
	}
	// The plain's shapes, a cell's worth at a time round where the camera
	// looks.
	ci, cj := int(math.Floor(gx0/vecCell)), int(math.Floor(gz0/vecCell))
	for j := cj - 8; j <= cj+8; j++ {
		for i := ci - 8; i <= ci+8; i++ {
			k := uint64(int64(i))*0x9e3779b1 ^ uint64(int64(j))*0x51ed27 ^ vc.seed
			kind := h01(k, 1)
			if kind > 0.3 {
				continue
			}
			p := v3((float64(i)+0.2+0.6*h01(k, 2))*vecCell, 0, (float64(j)+0.2+0.6*h01(k, 3))*vecCell)
			sy := h01(k, 4) * math.Pi
			var edges [][2]vec3
			switch {
			case kind < 0.13:
				edges = pyramid(p, 1+1.2*h01(k, 5), 1.5+2.5*h01(k, 6), sy)
			case kind < 0.24:
				s := 0.8 + 1.4*h01(k, 5)
				edges = wirebox(p.sub(v3(s, 0, s)), p.add(v3(s, 2*s, s)), sy)
			default:
				s := 0.6 + 0.6*h01(k, 5)
				edges = wirebox(p.sub(v3(s, 0, s)), p.add(v3(s, 4+4*h01(k, 6), s)), sy)
			}
			for _, e := range edges {
				vc.edge(v, &c, e[0], e[1], 0.7)
			}
		}
	}
	// The helicopter, last, over everything.
	for _, e := range heli(hp, yaw, pitch, roll, t*17, 1.2) {
		vc.edge(v, &c, e[0], e[1], 1.7)
	}
}
