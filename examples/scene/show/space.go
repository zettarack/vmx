package show

import "math"

// space is a world going by: a planet lit from one side (a giant in bands,
// a blue world of seas, land and cloud, or a moon of ice), on some a ring
// that the planet's shadow crosses, a small moon going round, a nebula and
// the stars behind; or, on some parts, a run through the stars at speed.
type space struct {
	seed       uint64
	kind       int
	ring, warp bool
	neb, body  palette
	light      vec3
	axis       rot
	ringN      vec3
	at, drift  vec3
	r          float64
}

func newSpace(seed uint64, v *view) scene {
	s := &space{seed: seed, kind: int(mix64(seed) % 3), warp: h01(seed, 1) < 0.22}
	s.ring = s.kind == 0 && h01(seed, 2) < 0.75 || s.kind == 2 && h01(seed, 2) < 0.3
	s.neb, s.body = palFor(mix64(seed+1)), palFor(mix64(seed+2))
	side := 1.0
	if h01(seed, 3) < 0.5 {
		side = -1
	}
	s.light = v3(side*0.85, 0.25+0.2*h01(seed, 4), -0.35+0.6*h01(seed, 5)).norm()
	s.axis = rotXY(0.25*(h01(seed, 6)-0.5), 0)
	s.ringN = v3(0.12*side, 1, 0.38+0.2*h01(seed, 7)).norm()
	s.r = 2.1 + 0.6*h01(seed, 8)
	s.at = v3(-side*(1.4+0.6*h01(seed, 9)), -0.25+0.5*h01(seed, 10), 7.5)
	s.drift = v3(side*0.012, 0.002, -0.025)
	return s
}

// hit is where a ray from the eye meets a sphere, or -1.
func hit(rd, c vec3, r float64) float64 {
	b := rd.dot(c)
	d := b*b - c.dot(c) + r*r
	if d < 0 {
		return -1
	}
	return b - math.Sqrt(d)
}

func (s *space) frame(v *view, t float64) {
	if s.warp {
		s.run(v, t)
		return
	}
	c := s.at.add(s.drift.scale(t))
	spin := rotXY(0, t*0.035)
	mo := 0.9 + t*0.1
	ms, mc := math.Sincos(mo)
	moon := c.add(v3(mc*s.r*1.9, 0.15*s.r*ms, ms*s.r*1.9))
	mr := s.r * 0.13
	v.each(func(x, y float64) vec3 {
		rd := v3(x, y, 1.25).norm()
		col := s.sky(x, y, t)
		// The haze of the planet's air, beyond its edge.
		if b := rd.dot(c); b > 0 {
			if dm := math.Sqrt(math.Max(c.dot(c)-b*b, 0)); dm > s.r {
				p := rd.scale(b).sub(c).norm()
				col = col.add(v3(0.25, 0.45, 1).scale(0.5 * math.Exp(-(dm-s.r)/(0.05*s.r)) * smooth(-0.3, 0.5, p.dot(s.light))))
			}
		}
		layers := [3]struct {
			d float64
			c vec3
			a float64
		}{}
		n := 0
		if tp := hit(rd, c, s.r); tp > 0 {
			layers[n].d, layers[n].c, layers[n].a = tp, s.planet(rd.scale(tp), c, spin, rd), 1
			n++
		}
		if tm := hit(rd, moon, mr); tm > 0 {
			p := rd.scale(tm)
			nm := p.sub(moon).scale(1 / mr)
			lit := math.Max(nm.dot(s.light), 0)
			if hit(s.light, c.sub(p), s.r) > 0 {
				lit = 0
			}
			alb := 0.55 + 0.25*fbm(nm.x*4+nm.z*2, nm.y*4, 3, s.seed+9)
			layers[n].d, layers[n].c, layers[n].a = tm, gray(alb*(1.6*lit+0.01)), 1
			n++
		}
		if s.ring {
			if den := rd.dot(s.ringN); math.Abs(den) > 1e-4 {
				if tr := c.dot(s.ringN) / den; tr > 0 {
					q := rd.scale(tr)
					rr := q.sub(c).length() / s.r
					if rr > 1.35 && rr < 2.35 {
						a := s.rings(rr)
						lit := 1.0
						if hit(s.light, c.sub(q), s.r) > 0 {
							lit = 0.06
						}
						layers[n].d, layers[n].c, layers[n].a = tr, s.body.at(rr*0.3+0.2).scale(1.3*lit+0.02), a
						n++
					}
				}
			}
		}
		// Far to near over the background.
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				if layers[j].d > layers[i].d {
					layers[i], layers[j] = layers[j], layers[i]
				}
			}
		}
		for i := 0; i < n; i++ {
			col = mix(col, layers[i].c, layers[i].a)
		}
		return col
	})
}

func (s *space) rings(r float64) float64 {
	a := 0.55 + 0.25*math.Sin(r*41) + 0.15*math.Sin(r*97+1) + 0.15*noise1(r*30, s.seed+20)
	if r > 1.86 && r < 1.92 { // the gap
		a *= 0.1
	}
	return sat(a) * smooth(1.35, 1.45, r) * smooth(2.35, 2.2, r) * 0.85
}

func (s *space) planet(p, c vec3, spin rot, rd vec3) vec3 {
	n := p.sub(c).scale(1 / s.r)
	m := spin.apply(s.axis.apply(n))
	lat := math.Asin(math.Max(-1, math.Min(1, m.y)))
	ndl := n.dot(s.light)
	var alb vec3
	night := vec3{}
	switch s.kind {
	case 0: // a giant in bands, a storm in them
		f := lat*6 + 0.5*fbm(m.x*2+m.z*1.7, m.y*9, 4, s.seed+30)
		alb = s.body.at(f * 0.12).scale(0.55 + 0.25*math.Sin(f*7))
		storm := math.Exp(-(sq(m.x-0.5)+sq(m.y+0.25)+sq(m.z-0.7))*30) * smooth(0, 0.4, m.z)
		alb = mix(alb, v3(0.9, 0.5, 0.35), storm*0.7)
	case 1: // a blue world: seas, land, ice and cloud; lights on its night
		land := fbm(m.x*1.8+m.z*0.9, m.y*1.8-m.z*0.7, 5, s.seed+31)
		alb = v3(0.01, 0.04, 0.13)
		if land > 0.12 {
			alb = mix(v3(0.12, 0.2, 0.06), v3(0.35, 0.28, 0.16), smooth(0.2, 0.6, land))
			if h01(uint64(math.Floor(m.x*300))*977+uint64(math.Floor(m.y*300))*31+uint64(math.Floor(m.z*300)), s.seed) < 0.04 {
				night = v3(1.2, 0.8, 0.35)
			}
		}
		alb = mix(alb, gray(0.85), smooth(1.15, 1.3, math.Abs(lat)))
		cl := fbm(m.x*2.5+m.z+0.01*float64(s.seed%97), m.y*3, 5, s.seed+32)
		alb = mix(alb, gray(0.95), smooth(0.05, 0.5, cl)*0.85)
	default: // ice and craters
		k := fbm(m.x*3+m.z*2, m.y*3-m.z, 5, s.seed+33)
		alb = mix(v3(0.55, 0.6, 0.68), v3(0.85, 0.9, 0.95), smooth(-0.3, 0.5, k))
	}
	lit := smooth(-0.04, 0.3, ndl) * (0.25 + 0.75*math.Max(ndl, 0))
	// The ring's shadow across the planet.
	if s.ring {
		if den := s.light.dot(s.ringN); math.Abs(den) > 1e-4 {
			if tr := c.sub(p).dot(s.ringN) / den; tr > 0 {
				rr := p.add(s.light.scale(tr)).sub(c).length() / s.r
				if rr > 1.35 && rr < 2.35 {
					lit *= 1 - 0.85*s.rings(rr)
				}
			}
		}
	}
	col := alb.scale(2.2 * lit).add(night.scale(smooth(0.05, -0.2, ndl)))
	rim := math.Pow(1-math.Max(0, -rd.dot(n)), 4) * smooth(-0.3, 0.4, ndl)
	return col.add(v3(0.3, 0.5, 1).scale(rim * 0.9))
}

// sky is the nebula and the stars behind everything.
func (s *space) sky(x, y, t float64) vec3 {
	x += t * 0.002
	qx := fbm(x*1.1, y*1.1, 4, s.seed+50)
	qy := fbm(x*1.1+5.2, y*1.1+1.3, 4, s.seed+51)
	n := fbm(x*1.4+1.6*qx, y*1.4+1.6*qy, 5, s.seed+52)
	dens := smooth(-0.1, 0.9, n)
	dust := smooth(0.1, 0.6, fbm(x*3+qy, y*3+qx, 4, s.seed+53))
	col := s.neb.at(n*0.5 + qx*0.4).scale(0.28 * dens * (1 - 0.7*dust)).add(s.neb.at(0.6 + qy*0.3).scale(0.05 * dens))
	return col.add(stars(x, y, t, s.seed))
}

// stars are points over the picture, a few of them bright.
func stars(x, y, t float64, seed uint64) vec3 {
	const cell = 0.0021
	ix, iy := int64(math.Floor(x/cell)), int64(math.Floor(y/cell))
	h := lattice(ix, iy, seed+0x77)
	if h&255 > 2 {
		return vec3{}
	}
	b := float64(h>>8&255) / 255
	tw := 0.7 + 0.3*math.Sin(t*(1.5+2*b)+float64(h>>16&63))
	return v3(0.85, 0.9, 1).scale((0.15 + 2.5*b*b*b) * tw)
}

// run is the flight at speed: the stars stream out of the middle, each a
// line that grows as it comes close, toward a nebula ahead.
func (s *space) run(v *view, t float64) {
	v.each(func(x, y float64) vec3 {
		r := math.Hypot(x, y)
		a := math.Atan2(y, x)
		col := s.sky(x*0.6, y*0.6, t).scale(0.6 + 0.6*math.Exp(-r*3))
		for l := 0; l < 3; l++ {
			cells := 220.0 + 140*float64(l)
			ia := math.Floor((a + math.Pi) / (2 * math.Pi) * cells)
			ca := (ia+0.5)/cells*2*math.Pi - math.Pi
			h := mix64(uint64(ia)*0x9e37 + uint64(l)<<32 ^ s.seed)
			speed := 0.25 + 0.35*float64(h>>40&255)/255
			z := fract(float64(h&0xffff)/65536 + t*speed)
			head := 0.04 * math.Exp(z*4)
			tail := head * (1 - 0.08*(1+z*3))
			if r > head || r < tail {
				continue
			}
			width := math.Abs(math.Sin(a-ca)) * r
			if width < 0.0015 {
				col = col.add(v3(0.8, 0.9, 1).scale(1.6 * z * (1 - width/0.0015)))
			}
		}
		return col
	})
}
