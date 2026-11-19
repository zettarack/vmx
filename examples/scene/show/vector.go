package show

import "math"

// vector is a drive across a plain drawn the way the old vector screens
// drew: lines of light on black, a glow round each, every edge of every
// shape seen. Faint crosses mark the ground; blocks and pyramids stand on
// the plain, a range of mountains
// runs round the horizon with a moon over it, a few wireframe rovers roam
// among the blocks, and now and then a saucer crosses. The camera drives
// on, swinging slowly from side to side.
type vector struct {
	seed  uint64
	speed float64
	sway  float64
	ridge []float64 // the mountains' heights, round the horizon
	moon  float64   // the moon's bearing
	ink   vec3
}

const (
	vecEye     = 1.4  // the camera's height
	vecFocal   = 1.1  // picture heights from the eye to the screen
	vecHorizon = 0.53 // the horizon, from the picture's top, in heights
	vecCell    = 9.0  // the plain's cells, each with a shape or none
)

func newVector(seed uint64, v *view) scene {
	vc := &vector{seed: seed, speed: 3 + 2*h01(seed, 1), sway: 6 + 6*h01(seed, 2), moon: 2 * math.Pi * h01(seed, 3)}
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

// cam is the camera at t: where it is and which way it faces.
type cam struct {
	eye  vec3
	r, f vec3
	yaw  float64
}

func (vc *vector) camera(t float64) cam {
	const w = 0.11
	x := vc.sway * math.Sin(t*w)
	z := vc.speed * t
	yaw := math.Atan2(vc.sway*w*math.Cos(t*w), vc.speed)
	sy, cy := math.Sincos(yaw)
	return cam{eye: v3(x, vecEye, z), f: v3(sy, 0, cy), r: v3(cy, 0, -sy), yaw: yaw}
}

// project is a point of the world on the picture, in pixels, and how far
// in front of the camera it is.
func (c *cam) project(v *view, p vec3) (x, y, z float64) {
	d := p.sub(c.eye)
	z = d.dot(c.f)
	x = d.dot(c.r) / z * vecFocal
	y = d.y / z * vecFocal
	return x*float64(v.ph) + float64(v.pw)/2, (vecHorizon - y) * float64(v.ph), z
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
	far := math.Min(z0, z1)
	v.line(x0, y0, x1, y1, vc.ink.scale(light*sat(1.35-far/45)))
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

// rover is a wireframe vehicle of the scene's own: a low hull with a
// sloped nose, a turret on it and a gun from the turret, heading yaw,
// k times its own size.
func rover(at vec3, yaw, k float64) [][2]vec3 {
	s, c := math.Sincos(yaw)
	turn := func(x, y, z float64) vec3 { x, y, z = x*k, y*k, z*k; return v3(at.x+x*c+z*s, at.y+y, at.z-x*s+z*c) }
	hull := []vec3{
		turn(-0.9, 0, -1.4), turn(0.9, 0, -1.4), turn(0.9, 0, 1.0), turn(-0.9, 0, 1.0),
		turn(-0.7, 0.55, -1.1), turn(0.7, 0.55, -1.1), turn(0.7, 0.55, 0.5), turn(-0.7, 0.55, 0.5),
	}
	var e [][2]vec3
	for i := 0; i < 4; i++ {
		e = append(e, [2]vec3{hull[i], hull[(i+1)%4]}, [2]vec3{hull[4+i], hull[4+(i+1)%4]}, [2]vec3{hull[i], hull[4+i]})
	}
	tur := []vec3{turn(-0.35, 0.55, -0.6), turn(0.35, 0.55, -0.6), turn(0.3, 0.9, -0.5), turn(-0.3, 0.9, -0.5), turn(0, 0.95, 0.1)}
	e = append(e, [2]vec3{tur[0], tur[1]}, [2]vec3{tur[1], tur[2]}, [2]vec3{tur[2], tur[3]}, [2]vec3{tur[3], tur[0]},
		[2]vec3{tur[2], tur[4]}, [2]vec3{tur[3], tur[4]}, [2]vec3{tur[0], tur[4]}, [2]vec3{tur[1], tur[4]})
	e = append(e, [2]vec3{turn(0, 0.8, 0.0), turn(0, 0.8, 1.6)})
	return e
}

// saucer is two shallow cones rim to rim.
func saucer(at vec3, r, spin float64) [][2]vec3 {
	const n = 10
	var rim [n]vec3
	for i := range rim {
		s, c := math.Sincos(spin + float64(i)/n*2*math.Pi)
		rim[i] = at.add(v3(c*r, 0, s*r))
	}
	top, bot := at.add(v3(0, r*0.4, 0)), at.add(v3(0, -r*0.25, 0))
	var e [][2]vec3
	for i := range rim {
		e = append(e, [2]vec3{rim[i], rim[(i+1)%n]})
		if i%2 == 0 {
			e = append(e, [2]vec3{rim[i], top}, [2]vec3{rim[i], bot})
		}
	}
	return e
}

func (vc *vector) frame(v *view, t float64) {
	c := vc.camera(t)
	// The horizon, the mountains round it and the moon over it: so far off
	// they turn with the camera alone, never coming nearer.
	hy := vecHorizon * float64(v.ph)
	v.line(-10, hy, float64(v.pw)+10, hy, vc.ink.scale(0.35))
	fov := math.Atan(float64(v.pw) / 2 / float64(v.ph) / vecFocal)
	at := func(bearing, up float64) (float64, float64, bool) {
		a := math.Remainder(bearing-c.yaw, 2*math.Pi)
		if math.Abs(a) > fov+0.2 {
			return 0, 0, false
		}
		return math.Tan(a)*vecFocal*float64(v.ph) + float64(v.pw)/2, hy - up*float64(v.ph), true
	}
	n := len(vc.ridge)
	for i := 0; i < n; i++ {
		b0, b1 := float64(i)/float64(n)*2*math.Pi, float64(i+1)/float64(n)*2*math.Pi
		x0, y0, ok0 := at(b0, vc.ridge[i])
		x1, y1, ok1 := at(b1, vc.ridge[(i+1)%n])
		if ok0 && ok1 {
			v.line(x0, y0, x1, y1, vc.ink.scale(0.55))
		}
	}
	if mx, my, ok := at(vc.moon, 0.3); ok {
		r := 0.045 * float64(v.ph)
		for k := 0; k < 16; k++ {
			a0, a1 := math.Pi*0.15+float64(k)/16*math.Pi*1.7, math.Pi*0.15+float64(k+1)/16*math.Pi*1.7
			v.line(mx+r*math.Cos(a0), my-r*math.Sin(a0), mx+r*math.Cos(a1), my-r*math.Sin(a1), vc.ink.scale(0.7))
			// The crescent's inner edge, a smaller circle set off to the side.
			v.line(mx+0.35*r+0.8*r*math.Cos(a0), my-0.8*r*math.Sin(a0), mx+0.35*r+0.8*r*math.Cos(a1), my-0.8*r*math.Sin(a1), vc.ink.scale(0.4))
		}
	}
	// Marks on the ground, a cross every five units, faint: what the plain
	// shows of the camera's speed.
	for gz := math.Ceil((c.eye.z+1.5)/5) * 5; gz < c.eye.z+38; gz += 5 {
		for gx := math.Floor((c.eye.x-26)/5) * 5; gx <= c.eye.x+26; gx += 5 {
			vc.edge(v, &c, v3(gx-0.25, 0, gz), v3(gx+0.25, 0, gz), 0.45)
			vc.edge(v, &c, v3(gx, 0, gz-0.25), v3(gx, 0, gz+0.25), 0.45)
		}
	}
	// The plain's shapes, a cell's worth at a time round the camera.
	ci, cj := int(math.Floor(c.eye.x/vecCell)), int(math.Floor(c.eye.z/vecCell))
	for j := cj - 2; j <= cj+6; j++ {
		for i := ci - 7; i <= ci+7; i++ {
			k := uint64(int64(i))*0x9e3779b1 ^ uint64(int64(j))*0x51ed27 ^ vc.seed
			kind := h01(k, 1)
			if kind > 0.3 {
				continue
			}
			p := v3((float64(i)+0.2+0.6*h01(k, 2))*vecCell, 0, (float64(j)+0.2+0.6*h01(k, 3))*vecCell)
			if math.Abs(p.x-c.eye.x) < 2.5 && math.Abs(p.z-c.eye.z) < 4 {
				continue // the road the camera drives is kept clear
			}
			yaw := h01(k, 4) * math.Pi
			var edges [][2]vec3
			switch {
			case kind < 0.13:
				edges = pyramid(p, 1+1.2*h01(k, 5), 1.5+2.5*h01(k, 6), yaw)
			case kind < 0.24:
				s := 0.8 + 1.4*h01(k, 5)
				edges = wirebox(p.sub(v3(s, 0, s)), p.add(v3(s, 2*s, s)), yaw)
			default:
				s := 0.6 + 0.6*h01(k, 5)
				edges = wirebox(p.sub(v3(s, 0, s)), p.add(v3(s, 4+4*h01(k, 6), s)), yaw)
			}
			for _, e := range edges {
				vc.edge(v, &c, e[0], e[1], 0.9)
			}
		}
	}
	// The rovers, roaming ahead of the camera.
	for r := 0; r < 3; r++ {
		fr := float64(r)
		x := c.eye.x + 14*math.Sin(t*0.13+fr*2.1)
		z := c.eye.z + 13 + 6*math.Cos(t*0.17+fr*1.3) + 6*fr
		dx := 14 * 0.13 * math.Cos(t*0.13+fr*2.1)
		dz := vc.speed - 6*0.17*math.Sin(t*0.17+fr*1.3)
		for _, e := range rover(v3(x, 0, z), math.Atan2(dx, dz), 1.5) {
			vc.edge(v, &c, e[0], e[1], 1.2)
		}
	}
	// A saucer crossing, now and then.
	if u := math.Mod(t, 24); u < 9 {
		k := uint64(t / 24)
		x := c.eye.x + lerp(-40, 40, u/9)*(1-2*float64(mix64(k^vc.seed)&1))
		pos := v3(x, 6+4*h01(k, 7), c.eye.z+30+15*h01(k, 8))
		for _, e := range saucer(pos, 1.6, t*2) {
			vc.edge(v, &c, e[0], e[1], 1.2)
		}
	}
}
