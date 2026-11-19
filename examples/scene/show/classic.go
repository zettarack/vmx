package show

import "math"

// plasma is the old plasma, four waves and a cycling palette; on some
// parts a liquid one, its waves pushed about by noise.
type plasma struct {
	f      [4]float64
	s      [4]float64
	liquid bool
	pal    palette
	seed   uint64
}

func newPlasma(seed uint64, v *view) scene {
	p := &plasma{liquid: h01(seed, 1) < 0.5, pal: palFor(mix64(seed + 8)), seed: seed}
	for i := range p.f {
		p.f[i] = 9 + 14*h01(seed, uint64(10+i))
		p.s[i] = 0.5 + 1.1*h01(seed, uint64(20+i))
	}
	return p
}

func (p *plasma) frame(v *view, t float64) {
	v.display = true
	ca, sa := math.Cos(t*0.11), math.Sin(t*0.13)
	mx, my := 0.6*math.Sin(t*0.21), 0.3*math.Cos(t*0.17)
	v.each(func(x, y float64) vec3 {
		if p.liquid {
			x += 0.25 * fbm(x*1.5+t*0.05, y*1.5, 3, p.seed)
			y += 0.25 * fbm(x*1.5, y*1.5-t*0.04, 3, p.seed+1)
		}
		w := math.Sin(x*p.f[0]+t*p.s[0]) + math.Sin(y*p.f[1]-t*p.s[1]) +
			math.Sin((x*ca+y*sa)*p.f[2]+t*p.s[2]) + math.Sin(math.Hypot(x-mx, y-my)*p.f[3]-t*p.s[3])
		c := p.pal.at(w*0.125 + t*0.04)
		return c.mul(c).scale(1.25)
	})
}
