package show

import "math"

// mandel is a dive into the Mandelbrot set, toward a point of its edge
// where there is something at every depth (the seahorse valley, a spiral,
// a dendrite's tip, the Feigenbaum point), turning as it goes; a frame
// deeper in costs more, so the wall's frames a second are real work.
type mandel struct {
	cx, cy, rate, turn float64
	pal                palette
}

var dives = [][2]float64{
	{-0.743643887037151, 0.131825904205330},
	{-0.77568377, 0.13646737},
	{-0.10109636384562, 0.95628651080914},
	// Not the Feigenbaum point (-1.401155189092, 0): past 1e-5 it is all
	// inside the set, and its parts drew black for minutes (2026-11-19,
	// found when the show's families grew; see TestEveryDiveStaysDrawn).
	{-0.7746806106269039, -0.1374168856037867},
	{0, 1},
}

func newMandel(seed uint64, v *view) scene {
	d := dives[int(mix64(seed)%uint64(len(dives)))]
	// Deep by the part's end, as deep as a float's 53 bits draw cleanly
	// (a picture height about 1e-10 across).
	return &mandel{cx: d[0], cy: d[1], rate: (21 + 2.5*h01(seed, 1)) / SceneLen, turn: 0.03 * (h01(seed, 2) - 0.5), pal: palFor(mix64(seed + 4))}
}

func (m *mandel) frame(v *view, t float64) {
	v.display = true
	scale := 2.8 * math.Exp(-m.rate*t)
	// As many steps as the depth wants, not the time: a dive of seven
	// minutes at a minute's count would ask thirteen thousand.
	maxIt := min(150+int(70*math.Log(2.8/scale)), 2400)
	sn, cs := math.Sincos(t * m.turn)
	v.each(func(x, y float64) vec3 {
		px, py := (x*cs-y*sn)*scale, (x*sn+y*cs)*scale
		cr, ci := m.cx+px, m.cy+py
		// The two great bulbs are inside: no need to count to know.
		if q := sq(cr-0.25) + ci*ci; q*(q+cr-0.25) < 0.25*ci*ci || sq(cr+1)+ci*ci < 0.0625 {
			return vec3{}
		}
		zr, zi, n := 0.0, 0.0, 0
		for ; n < maxIt && zr*zr+zi*zi < 256; n++ {
			zr, zi = zr*zr-zi*zi+cr, 2*zr*zi+ci
		}
		if n == maxIt {
			return vec3{}
		}
		mu := float64(n) + 1 - math.Log2(math.Log2(math.Sqrt(zr*zr+zi*zi)))
		return m.pal.at(mu*0.022 + t*0.05).scale(smooth(0, 6, mu))
	})
}

// julia is a Julia set changing shape as its constant goes round the
// main bulb's edge, the whole turning slowly.
type julia struct {
	r, w, a0, zoom, turn float64
	pal                  palette
}

func newJulia(seed uint64, v *view) scene {
	return &julia{r: 0.7885 - 0.02*h01(seed, 1), w: 0.02 + 0.02*h01(seed, 2), a0: 2 * math.Pi * h01(seed, 3),
		zoom: 2.6 + 0.6*h01(seed, 4), turn: 0.05 * (h01(seed, 5) - 0.5), pal: palFor(mix64(seed + 6))}
}

func (j *julia) frame(v *view, t float64) {
	v.display = true
	cr, ci := j.r*math.Cos(j.a0+t*j.w), j.r*math.Sin(j.a0+t*j.w)
	sn, cs := math.Sincos(t * j.turn)
	z := j.zoom * (1 - 0.25*smooth(0, SceneLen, t))
	v.each(func(x, y float64) vec3 {
		zr, zi := (x*cs-y*sn)*z, (x*sn+y*cs)*z
		n := 0
		for ; n < 220 && zr*zr+zi*zi < 256; n++ {
			zr, zi = zr*zr-zi*zi+cr, 2*zr*zi+ci
		}
		if n == 220 {
			return vec3{}
		}
		mu := float64(n) + 1 - math.Log2(math.Log2(math.Sqrt(zr*zr+zi*zi)))
		return j.pal.at(mu*0.045 + t*0.04).scale(smooth(0, 4, mu))
	})
}
