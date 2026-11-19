package show

import "math"

// day is a whole day over hills in a scene's seven minutes: night and its stars, dawn,
// the sun's arc, dusk and the moon, over ridges that pale with distance,
// with a lake below that holds it all again, on some days clouds, on some
// nights the northern lights.
type day struct {
	seed           uint64
	start          float64 // the hour the part opens on
	ridges         []ridge
	lake           bool
	water, horizon float64
	clouds, aurora float64
	fov            float64
}

type ridge struct {
	base, amp, freq, off float64
	depth                float64 // 0 far, 1 near
	trees                bool
	seed                 uint64
}

func newDay(seed uint64, v *view) scene {
	d := &day{seed: seed, start: 17 + 2*h01(seed, 1), horizon: -0.06, fov: 1.15}
	d.lake = h01(seed, 2) < 0.65
	d.water = d.horizon - 0.07
	d.clouds = 0.15 + 0.6*h01(seed, 3)*h01(seed, 3)
	if h01(seed, 4) < 0.55 {
		d.aurora = 0.6 + 0.6*h01(seed, 5)
	}
	n := 3 + int(h01(seed, 6)*2)
	for i := 0; i < n; i++ {
		k := float64(i) / float64(n-1)
		d.ridges = append(d.ridges, ridge{
			base:  d.horizon + 0.02 + 0.07*(1-k) - 0.04*k,
			amp:   0.05 + 0.13*k*(0.6+0.8*h01(seed, uint64(10+i))),
			freq:  1.1 + 2.2*(1-k) + h01(seed, uint64(20+i)),
			off:   100 * h01(seed, uint64(30+i)),
			depth: k,
			trees: i == n-1 && h01(seed, 7) < 0.6,
			seed:  mix64(seed + uint64(i)),
		})
	}
	return d
}

func (r *ridge) top(x float64) float64 {
	h := r.base + r.amp*(0.5+0.5*fbm1(x*r.freq+r.off, 4, r.seed))
	if r.trees {
		h += 0.018 * math.Pow(math.Abs(noise1(x*95+r.off, r.seed+1)), 0.6)
	}
	return h
}

func (d *day) frame(v *view, t float64) {
	s := newSky(math.Mod(d.start+t*24/SceneLen, 24), d.seed)
	s.clouds, s.cloudAt, s.wind, s.aurora = d.clouds, 0.4, 0.05, d.aurora
	v.each(func(x, y float64) vec3 {
		if d.lake && y < d.water {
			// The lake: everything above it again, upside down, broken by
			// ripples that are finer close by; darker straight down.
			depth := (d.water - y) / (d.water + 0.5)
			rx := x + 0.006*perlin(x*30, y*260+t*1.5, d.seed+70)*(0.3+depth)
			c := d.above(rx, 2*d.water-y, t, s).scale(0.62 - 0.25*depth)
			// The sun's road on the water.
			if s.sun.y > -0.02 {
				sx := s.sun.x / (s.sun.z * d.fov)
				road := math.Exp(-sq((x-sx)/(0.02+0.1*depth))) * smooth(0.5, 0.9, perlin(x*60, y*400+t*3, d.seed+71)+0.5)
				c = c.add(s.sunC.scale(0.9 * road))
			}
			return c
		}
		return d.above(x, y, t, s)
	})
}

// above is the land and the sky at (x, y), nearest ridge first.
func (d *day) above(x, y, t float64, s *sky) vec3 {
	for i := len(d.ridges) - 1; i >= 0; i-- {
		r := &d.ridges[i]
		if top := r.top(x); y < top {
			// Far ridges take the air's colour, near ones are dark; lower
			// down each, the mist of its valley shows through; the side of
			// a ridge that faces the sun takes a little of it; a faint rim.
			air := s.hor.scale(0.8).add(s.amb.scale(0.15))
			// Woods, darker and greener the nearer, lit by the sky, the sun
			// and the moon, with the grain of their trees.
			alb := mix(v3(0.12, 0.16, 0.12), v3(0.05, 0.1, 0.04), r.depth)
			grain := 0.75 + 0.5*perlin(x*60, y*60, r.seed+5)*r.depth
			body := alb.mul(s.amb.scale(0.9).add(s.sunC.scale(0.3 * s.day)).add(s.moonC.scale(0.8))).scale(grain)
			c := mix(air, body, 0.3+0.7*r.depth)
			mist := smooth(0, 0.1*(0.6+r.depth), top-y) * (0.6 - 0.4*r.depth)
			c = mix(c, air, mist)
			rim := smooth(top-0.003, top, y)
			return c.add(s.sunC.scale(0.06 * rim * s.dusk)).add(s.moonC.scale(0.35 * rim))
		}
	}
	dir := v3(x*d.fov, y-d.horizon, 1).norm()
	return s.color(dir, t)
}
