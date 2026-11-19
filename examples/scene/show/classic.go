package show

import "math"

// tunnel is the old flight down a tube: its walls in checks, in rings, in
// hexagons, or in bits, a light at its end, the tube winding as it goes.
type tunnel struct {
	kind               int
	speed, twist, size float64
	pal                palette
	seed               uint64
}

func newTunnel(seed uint64, v *view) scene {
	return &tunnel{kind: int(mix64(seed) % 4), speed: 0.9 + 0.8*h01(seed, 1), twist: 0.2 * (h01(seed, 2) - 0.5),
		size: 0.32 + 0.12*h01(seed, 3), pal: palFor(mix64(seed + 7)), seed: seed}
}

func (tu *tunnel) frame(v *view, t float64) {
	v.display = true
	cx, cy := 0.16*math.Sin(t*0.53), 0.09*math.Cos(t*0.41)
	v.each(func(x, y float64) vec3 {
		dx, dy := x-cx, y-cy
		r := math.Max(math.Hypot(dx, dy), 1e-4)
		a := math.Atan2(dy, dx)/math.Pi*8 + t*tu.twist*8
		w := tu.size/r + t*tu.speed*2
		var tex float64
		switch tu.kind {
		case 0: // checks
			tex = 0.06 + 0.7*float64((int(math.Floor(a))+int(math.Floor(w)))&1)
		case 1: // rings and lanes of light
			tex = 0.04 + 1.1*math.Pow(math.Abs(math.Sin(w*math.Pi)), 10) + 0.7*math.Pow(math.Abs(math.Sin(a*math.Pi)), 40)
		case 2: // a lattice of hexagons, their edges lit
			hx, hy := a*1.1547, w+0.5*float64(int(math.Floor(a*1.1547))&1)
			fx, fy := fract(hx)-0.5, fract(hy)-0.5
			tex = 0.04 + smooth(0.36, 0.44, math.Max(math.Abs(fx)*1.15+math.Abs(fy)*0.58, math.Abs(fy)*1.15))
		default: // bits: a digit to a cell, flipping as the cells come by
			ca, cw := math.Floor(a*1.5), math.Floor(w*1.2)
			d := mix64(uint64(int64(ca))*0x9e37^uint64(int64(cw))*0x51ed^tu.seed) & 1
			gx, gy := int((a*1.5-ca)*7)-1, int((w*1.2-cw)*9)-1
			tex = 0.03
			if gx >= 0 && gx < 5 && gy >= 0 && gy < 7 && digits[d][6-gy][gx] == '#' {
				tex = 1
			}
		}
		fog := smooth(0.015, 0.3, r) * (1 - 0.6*smooth(0.45, 1.1, r))
		glow := math.Exp(-r*9) * 1.1
		c := tu.pal.at(w*0.04 + 0.2).scale(tex * fog)
		return c.add(gray(glow))
	})
}

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
