package show

import "math"

// sky is the sky at an hour of the day: where the sun and the moon are,
// their light, and the colours from the horizon up. Directions have y up;
// the sun rises on the left (x -1), stands high in front (z +1) at noon
// and sets on the right.
type sky struct {
	sun, moon         vec3
	sunC, moonC       vec3 // their light on the ground
	amb               vec3 // the sky's own light on the ground
	zen, hor          vec3
	day, dusk         float64
	phase             float64 // the moon's: 0 new, 0.5 full
	clouds, aurora    float64
	cloudAt, wind     float64
	seed              uint64
	starLight, sunBig float64
}

// arc is the way across the sky of a body at angle a (0 rising, pi/2 at
// its highest, pi setting).
func arc(a, high float64) vec3 {
	s, c := math.Sincos(a)
	return v3(-c, s*high, 0.55+0.4*math.Max(s, 0)).norm()
}

func newSky(hour float64, seed uint64) *sky {
	s := &sky{seed: seed, phase: 0.15 + 0.7*h01(seed, 20), sunBig: 1}
	a := (hour - 6) / 12 * math.Pi
	s.sun = arc(a, 0.85)
	s.moon = arc(a+math.Pi+0.5, 0.7)
	e := s.sun.y
	s.day = smooth(-0.1, 0.22, e)
	s.dusk = math.Exp(-sq((e - 0.03) / 0.11))
	warm := smooth(-0.02, 0.4, e)
	s.sunC = mix(v3(1.6, 0.5, 0.12), v3(2.6, 2.4, 2.1), warm).scale(smooth(-0.06, 0.06, e))
	moonUp := smooth(-0.02, 0.15, s.moon.y) * (1 - s.day)
	s.moonC = v3(0.10, 0.13, 0.2).scale(moonUp * (0.4 + 0.6*math.Sin(s.phase*math.Pi)))
	s.zen = mix(v3(0.0015, 0.003, 0.009), v3(0.05, 0.14, 0.42), s.day).add(v3(0.05, 0.02, 0.07).scale(s.dusk))
	s.hor = mix(v3(0.005, 0.008, 0.02), v3(0.45, 0.6, 0.8), s.day).add(v3(1.1, 0.38, 0.1).scale(s.dusk))
	s.amb = mix(v3(0.012, 0.016, 0.03), v3(0.22, 0.28, 0.4), s.day).add(v3(0.2, 0.08, 0.04).scale(s.dusk))
	s.starLight = 1 - smooth(-0.2, 0.05, e)
	return s
}

// color is the sky's light seen along d (unit), at t seconds.
func (s *sky) color(d vec3, t float64) vec3 {
	up := math.Max(d.y, 0)
	c := mix(s.hor, s.zen, math.Pow(up, 0.45))
	// The sun's glow, warm and wide low down; its disc.
	if cs := d.dot(s.sun); cs > 0 {
		c = c.add(s.sunC.scale(0.10*math.Pow(cs, 6) + 0.35*math.Pow(cs, 60)))
		if cs > 0.9993*s.sunBig {
			c = c.add(s.sunC.scale(9 * smooth(0.9993, 0.9996, cs)))
		}
	}
	// The moon, lit on the side of its phase.
	if s.moon.y > -0.05 && s.day < 0.9 {
		if cm := d.dot(s.moon); cm > 0.9985 {
			c = c.add(s.moonDisc(d, cm))
		} else if cm > 0 {
			c = c.add(s.moonC.scale(0.5 * math.Pow(cm, 40)))
		}
	}
	if d.y > 0 {
		if s.starLight > 0.01 {
			c = c.add(s.star(d, t).scale(s.starLight))
		}
		if s.aurora > 0 && s.starLight > 0.3 {
			c = c.add(s.lights(d, t).scale(s.aurora * s.starLight))
		}
		if s.clouds > 0 {
			c = s.cloud(d, t, c)
		}
	}
	return c
}

func (s *sky) moonDisc(d vec3, cm float64) vec3 {
	// The disc's own two axes, and the point of the sphere seen.
	right := s.moon.cross(v3(0, 1, 0)).norm()
	upv := right.cross(s.moon)
	r := math.Sqrt(1 - 0.9985*0.9985)
	u, w := d.dot(right)/r, d.dot(upv)/r
	z := math.Sqrt(math.Max(0, 1-u*u-w*w))
	ph := (s.phase - 0.5) * 2 * math.Pi
	ls := math.Sin(ph)*u + math.Cos(ph)*z
	lit := smooth(-0.05, 0.12, ls)
	mare := 0.75 + 0.25*fbm(u*3, w*3, 2, s.seed+3)
	edge := smooth(0.9985, 0.99858, cm)
	return v3(1.6, 1.7, 1.9).scale(lit * mare * edge).add(v3(0.01, 0.012, 0.02).scale(edge))
}

func (s *sky) star(d vec3, t float64) vec3 {
	az := math.Atan2(d.x, d.z)
	el := math.Asin(math.Min(d.y, 1))
	const cell = 0.0026
	ix, iy := int64(math.Floor(az/cell)), int64(math.Floor(el/cell))
	h := lattice(ix, iy, s.seed+0x5)
	if h&1023 > 3 {
		return vec3{}
	}
	b := float64(h>>10&255) / 255
	tw := 0.65 + 0.35*math.Sin(t*(2+3*b)+float64(h>>20&63))
	return v3(0.8, 0.85, 1).scale((0.25 + 2.2*b*b) * tw * smooth(0, 0.08, el))
}

// lights is the aurora: curtains folding slowly over the north.
func (s *sky) lights(d vec3, t float64) vec3 {
	el := math.Asin(math.Min(d.y, 1))
	if el < 0.02 || el > 0.85 {
		return vec3{}
	}
	az := math.Atan2(d.x, d.z)
	// Curtains in a few long folds that drift, lit in patches along the
	// sky: each a bright lower hem, fading as it rises, in fine rays.
	fold := az*1.7 + 0.8*fbm1(az*0.9+t*0.03, 3, s.seed+40)
	patch := smooth(-0.1, 0.55, fbm1(az*1.3-t*0.025, 3, s.seed+43))
	hem := 0.09 + 0.07*fbm1(fold*0.7+t*0.02, 2, s.seed+41)
	h := el - hem
	if h < -0.03 || patch <= 0 {
		return vec3{}
	}
	shape := smooth(-0.03, 0.008, h) * math.Exp(-math.Max(h, 0)/0.15)
	rays := 0.3 + 0.7*sq(math.Abs(noise1(fold*34+t*0.22, s.seed+42)))
	g := shape * rays * patch
	return v3(0.04, 0.8, 0.32).scale(g * 0.42).add(v3(0.4, 0.08, 0.5).scale(g * 0.3 * smooth(0.04, 0.25, h)))
}

func (s *sky) cloud(d vec3, t float64, c vec3) vec3 {
	if d.y < 0.015 {
		return c
	}
	k := s.cloudAt / d.y
	px, pz := d.x*k+t*s.wind, d.z*k+t*s.wind*0.3
	n := fbm(px*0.9, pz*0.9, 5, s.seed+60)
	dens := smooth(0.55-s.clouds*0.7, 1.05-s.clouds*0.5, n+0.2) * smooth(0.015, 0.12, d.y)
	if dens <= 0 {
		return c
	}
	toward := math.Max(d.dot(s.sun), 0)
	lit := s.amb.scale(0.9).add(s.sunC.scale(0.35 + 0.65*math.Pow(toward, 4))).add(s.moonC.scale(1.2))
	edge := s.sunC.scale(1.4 * math.Pow(toward, 12) * (1 - dens))
	return mix(c, lit.add(edge), dens*0.9)
}
