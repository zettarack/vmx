package show

import "math"

// shapes is a solid turning in a studio's light, one becoming the next
// every fifteen seconds (a sphere, a box, a torus, an octahedron, a
// sponge, a gyroid ball), over a floor of lines that holds its shadow and
// its reflection; chrome on some parts, a colour of its own on others.
type shapes struct {
	seed   uint64
	order  [5]int
	key    vec3 // the key light, as the camera sees it: right, up, toward it
	light  vec3 // the same in the world, this frame
	chrome bool
	pal    palette
	speed  float64
}

func newShapes(seed uint64, v *view) scene {
	s := &shapes{seed: seed, chrome: h01(seed, 1) < 0.45, pal: palFor(mix64(seed + 3)), speed: 0.8 + 0.5*h01(seed, 2)}
	perm := []int{0, 1, 2, 3, 4, 5}
	r := mix64(seed + 9)
	for i := len(perm) - 1; i > 0; i-- {
		r = mix64(r + 1)
		j := int(r % uint64(i+1))
		perm[i], perm[j] = perm[j], perm[i]
	}
	copy(s.order[:], perm)
	s.key = v3(-0.7+1.4*h01(seed, 4), 0.8, 0.45).norm()
	return s
}

func box(p vec3, b float64) float64 {
	qx, qy, qz := math.Abs(p.x)-b, math.Abs(p.y)-b, math.Abs(p.z)-b
	out := math.Sqrt(sq(math.Max(qx, 0)) + sq(math.Max(qy, 0)) + sq(math.Max(qz, 0)))
	return out + math.Min(math.Max(qx, math.Max(qy, qz)), 0)
}

func gmod(x, y float64) float64 { return x - y*math.Floor(x/y) }

func solid(kind int, p vec3) float64 {
	switch kind {
	case 0:
		return p.length() - 1.05
	case 1:
		return box(p, 0.72) - 0.14
	case 2:
		return math.Hypot(math.Hypot(p.x, p.z)-0.85, p.y) - 0.33
	case 3:
		return (math.Abs(p.x) + math.Abs(p.y) + math.Abs(p.z) - 1.3) * 0.57735
	case 4: // a sponge, three times cut
		d := box(p, 0.95)
		s := 1.0
		for m := 0; m < 3; m++ {
			ax, ay, az := gmod(p.x*s/0.95, 2)-1, gmod(p.y*s/0.95, 2)-1, gmod(p.z*s/0.95, 2)-1
			s *= 3
			rx, ry, rz := math.Abs(1-3*math.Abs(ax)), math.Abs(1-3*math.Abs(ay)), math.Abs(1-3*math.Abs(az))
			da, db, dc := math.Max(rx, ry), math.Max(ry, rz), math.Max(rz, rx)
			d = math.Max(d, (math.Min(da, math.Min(db, dc))-1)/s*0.95)
		}
		return d
	default: // a ball of gyroid
		q := p.scale(5)
		g := math.Sin(q.x)*math.Cos(q.y) + math.Sin(q.y)*math.Cos(q.z) + math.Sin(q.z)*math.Cos(q.x)
		return math.Max(p.length()-1.08, (math.Abs(g)-0.35)/5*0.7)
	}
}

const floorY = -1.3

// state is a frame's own: the solid's turn and what it is becoming.
type state struct {
	turn    rot
	a, b    int
	k       float64
	eye     vec3
	f, r, u vec3
}

func (s *shapes) shape(st *state, p vec3) float64 {
	q := st.turn.apply(p)
	if st.k <= 0 {
		return solid(st.a, q)
	}
	return lerp(solid(st.a, q), solid(st.b, q), st.k)
}

// backdrop is the studio behind the solid: dark, a little light low down.
func (s *shapes) backdrop(d vec3) vec3 {
	return gray(0.002).add(v3(0.016, 0.02, 0.026).scale(math.Exp(-sq(d.y-0.05) * 10)))
}

// soft is how much of the light reaches p along l: 1 in the open, less
// in the solid's soft shadow.
func (s *shapes) soft(st *state, p, l vec3) float64 {
	sh, d := 1.0, 0.04
	for i := 0; i < 20 && d < 6; i++ {
		h := s.shape(st, p.add(l.scale(d)))
		if h < 0.001 {
			return 0
		}
		sh = math.Min(sh, 8*h/d)
		d += math.Max(h, 0.04)
	}
	return sat(sh)
}

func (s *shapes) env(d vec3) vec3 {
	if d.y < 0 {
		return gray(0.006)
	}
	c := gray(0.004).add(gray(0.05).scale(math.Exp(-sq(d.y) * 30)))
	// The studio's softbox, up and to the side, and a strip behind.
	c = c.add(gray(2.4).scale(math.Pow(math.Max(d.dot(s.light), 0), 30)))
	c = c.add(gray(0.6).scale(math.Exp(-sq(d.x*1.5)) * smooth(0.2, 0.5, d.y) * smooth(0.95, 0.7, d.y)))
	return c
}

func (s *shapes) frame(v *view, t float64) {
	st := &state{turn: rotXY(t*0.29*s.speed, t*0.41*s.speed)}
	seg := int(t / 15)
	st.a = s.order[seg%5]
	if u := t - float64(seg)*15; seg > 0 && u < 2.5 {
		st.b, st.k = st.a, 1-smooth(0, 2.5, u)
		st.a = s.order[(seg-1)%5]
		st.k = 1 - st.k
	}
	ph := t * 0.13
	st.eye = v3(5.2*math.Sin(ph), 0.9+0.5*math.Sin(t*0.17), -5.2*math.Cos(ph))
	st.f = st.eye.scale(-1).norm()
	st.r = st.f.cross(v3(0, 1, 0)).norm()
	st.u = st.r.cross(st.f)
	// The studio's light goes round with the camera, as a photographer's
	// would: the solid is always lit from the front, a little to a side.
	s.light = st.r.scale(-s.key.x).add(st.u.scale(s.key.y)).sub(st.f.scale(s.key.z)).norm()
	v.each(func(x, y float64) vec3 {
		rd := st.f.scale(1.7).add(st.r.scale(-x)).add(st.u.scale(y)).norm()
		return s.ray(st, st.eye, rd)
	})
}

// march is where along rd from ro the solid is met, or -1.
func (s *shapes) march(st *state, ro, rd vec3, steps int, far float64) float64 {
	d := 0.0
	for i := 0; i < steps && d < far; i++ {
		h := s.shape(st, ro.add(rd.scale(d)))
		if h < 0.0012*(1+d) {
			return d
		}
		d += h * 0.9
	}
	return -1
}

func (s *shapes) normal(st *state, p vec3) vec3 {
	const e = 0.0015
	a := s.shape(st, p.add(v3(e, -e, -e)))
	b := s.shape(st, p.add(v3(-e, -e, e)))
	c := s.shape(st, p.add(v3(-e, e, -e)))
	d := s.shape(st, p.add(v3(e, e, e)))
	return v3(a-b-c+d, -a-b+c+d, -a+b-c+d).norm()
}

func (s *shapes) lit(st *state, p, rd vec3) vec3 {
	n := s.normal(st, p)
	refl := rd.sub(n.scale(2 * rd.dot(n)))
	fres := 0.04 + 0.96*math.Pow(1-math.Max(-rd.dot(n), 0), 5)
	if s.chrome {
		return s.env(refl).mul(v3(0.9, 0.92, 1)).scale(0.35 + 0.65*fres)
	}
	alb := s.pal.at(0.15 + 0.12*n.y)
	dif := math.Max(n.dot(s.light), 0)
	if dif > 0 {
		dif *= s.soft(st, p.add(n.scale(0.01)), s.light)
	}
	spec := math.Pow(math.Max(n.dot(s.light.sub(rd).norm()), 0), 70) * dif
	rim := math.Pow(1-math.Max(-rd.dot(n), 0), 3)
	return alb.scale(1.9*dif + 0.04).add(gray(1.4 * spec)).add(v3(0.25, 0.35, 0.55).scale(0.45 * rim)).add(s.env(refl).scale(0.4 * fres))
}

func (s *shapes) ray(st *state, ro, rd vec3) vec3 {
	ts := s.march(st, ro, rd, 72, 12)
	tf := -1.0
	if rd.y < 0 {
		tf = (floorY - ro.y) / rd.y
	}
	if ts > 0 && (tf < 0 || ts < tf) {
		return s.lit(st, ro.add(rd.scale(ts)), rd)
	}
	if tf < 0 || tf > 40 {
		return s.backdrop(rd)
	}
	p := ro.add(rd.scale(tf))
	// The floor: lines going away, the solid's shadow soft on it, its
	// reflection in it.
	lx := math.Exp(-sq(fract(p.x*0.75+0.5)-0.5) * 600)
	lz := math.Exp(-sq(fract(p.z*0.75+0.5)-0.5) * 600)
	fade := math.Exp(-tf * 0.09)
	c := gray(0.012).add(live.scale(0.18 * (lx + lz) * fade))
	sh := s.soft(st, p, s.light)
	ao := smooth(0, 0.9, s.shape(st, p))
	c = c.scale(0.3 + 0.7*sh).scale(0.4 + 0.6*ao).add(gray(0.05 * sh * fade))
	rr := v3(rd.x, -rd.y, rd.z)
	if tr := s.march(st, p, rr, 44, 8); tr > 0 {
		c = c.add(s.lit(st, p.add(rr.scale(tr)), rr).scale(0.32 * fade))
	} else {
		c = c.add(s.env(rr).scale(0.08))
	}
	return c
}
