package show

import "math"

// land is a flight low over a landscape, the voxel way: the part's map
// made once (peaks, hills, mesas, islands or dunes), then each column of
// the tile drawn from near to far under the sky of its hour, into the haze.
type land struct {
	seed       uint64
	n          int
	hgt        []float64 // 0..1
	alb        []vec3    // the ground's own colour
	nrm        []vec3
	water      float64 // the sea's height, below 0: none
	amp        float64 // the height of 1, in cells
	hour, rate float64
	speed      float64
	x0, z0     float64
	yaw0       float64
	clear      float64
	clouds     float64
	fog        float64
}

// fbmP is fbm over a map that repeats every 1 each way.
func fbmP(x, y float64, oct int, period int64, seed uint64, ridged bool) float64 {
	s, a, n := 0.0, 1.0, 0.0
	for i := 0; i < oct; i++ {
		f := float64(period)
		v := perlinP(x*f, y*f, period, seed+uint64(i)*0x9e37)
		if ridged {
			v = 1 - math.Abs(v)*1.5
			v *= v
		}
		s += a * v
		n += a
		a *= 0.5
		period *= 2
	}
	return s / n
}

func newLand(seed uint64, v *view) scene {
	const n = 256
	l := &land{seed: seed, n: n, hgt: make([]float64, n*n), alb: make([]vec3, n*n), nrm: make([]vec3, n*n)}
	hours := []float64{6.4, 8.5, 12.5, 16.6, 18.2, 22.5, 1.5}
	l.hour = hours[int(h01(seed, 1)*float64(len(hours)))]
	l.rate = 0.012
	l.speed = 14 + 10*h01(seed, 2)
	l.yaw0 = 2 * math.Pi * h01(seed, 3)
	l.x0, l.z0 = n*h01(seed, 4), n*h01(seed, 5)
	l.clear = 14 + 10*h01(seed, 6)
	l.clouds = 0.6 * h01(seed, 7)
	l.fog = 0.0022 + 0.003*h01(seed, 8)
	style := mix64(seed) % 5
	l.water, l.amp = -1, 60
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			x, y := float64(i)/n, float64(j)/n
			var h float64
			switch style {
			case 0: // peaks
				h = math.Pow(fbmP(x, y, 6, 3, seed, true), 1.4)
			case 1: // hills
				h = 0.5 + 0.5*fbmP(x, y, 5, 2, seed, false)
			case 2: // mesas: terraces with steep sides
				f := (0.5 + 0.5*fbmP(x, y, 5, 2, seed, false)) / 0.11
				h = (math.Floor(f) + smooth(0.75, 1, fract(f))) * 0.11
			case 3: // islands
				h = 0.5 + 0.6*fbmP(x, y, 6, 3, seed, false)
			default: // dunes
				a := 2 * math.Pi * (x*3 + y*5 + 0.4*fbmP(x, y, 3, 2, seed, false))
				h = 0.5 + 0.3*math.Sin(a) + 0.2*fbmP(x, y, 4, 4, seed+1, false)
			}
			l.hgt[j*n+i] = h
		}
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, h := range l.hgt {
		lo, hi = math.Min(lo, h), math.Max(hi, h)
	}
	for i := range l.hgt {
		l.hgt[i] = (l.hgt[i] - lo) / math.Max(hi-lo, 1e-9)
	}
	switch style {
	case 0:
		l.water, l.amp = 0.08, 75
	case 1:
		l.water, l.amp = 0.22, 40
	case 2:
		l.amp = 55
	case 3:
		l.water, l.amp = 0.5, 45
	default:
		l.amp = 22
	}
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			k := j*n + i
			hx := l.hgt[j*n+(i+1)%n] - l.hgt[j*n+(i+n-1)%n]
			hz := l.hgt[((j+1)%n)*n+i] - l.hgt[((j+n-1)%n)*n+i]
			nm := v3(-hx*l.amp/2, 1, -hz*l.amp/2).norm()
			l.nrm[k] = nm
			l.alb[k] = l.colour(style, l.hgt[k], nm.y, float64(i), float64(j))
		}
	}
	return l
}

// colour is the ground's own colour at height h, its slope's up-ness up.
func (l *land) colour(style uint64, h, up, i, j float64) vec3 {
	n := 0.85 + 0.3*h01(uint64(i)*7919+uint64(j), l.seed)
	switch style {
	case 2: // red rock in bands
		band := 0.8 + 0.2*math.Sin(h*90)
		return mix(v3(0.42, 0.2, 0.09), v3(0.6, 0.38, 0.2), smooth(0.4, 0.9, up)).scale(band * n)
	case 4: // sand
		return v3(0.62, 0.46, 0.27).scale(n * (0.9 + 0.1*up))
	}
	rock := v3(0.25, 0.23, 0.21)
	grass := mix(v3(0.08, 0.2, 0.05), v3(0.18, 0.27, 0.08), h01(uint64(i)+uint64(j)*131, l.seed+1))
	c := mix(rock, grass, smooth(0.7, 0.88, up))
	if l.water >= 0 && h < l.water+0.03 {
		c = mix(c, v3(0.55, 0.5, 0.36), smooth(l.water+0.03, l.water, h))
	}
	if style == 0 || style == 3 {
		snow := smooth(0.58, 0.7, h+0.08*(up-0.7))
		c = mix(c, v3(0.9, 0.92, 0.96), snow)
	}
	return c.scale(n)
}

// at is the map's height at (x, z), between its cells, and the cell.
func (l *land) at(x, z float64) (float64, int) {
	n := l.n
	fx, fz := math.Floor(x), math.Floor(z)
	tx, tz := x-fx, z-fz
	i := ((int(fx) % n) + n) % n
	j := ((int(fz) % n) + n) % n
	i1, j1 := (i+1)%n, (j+1)%n
	h := lerp(lerp(l.hgt[j*n+i], l.hgt[j*n+i1], tx), lerp(l.hgt[j1*n+i], l.hgt[j1*n+i1], tx), tz)
	return h, j*n + i
}

// camera is where the flight is at t: its heading and place, and its
// height, kept clear of what is under it and just ahead.
func (l *land) camera(t float64) (yaw, x, z, y float64) {
	const sway, w = 40.0, 0.06
	fx, fz := math.Cos(l.yaw0), math.Sin(l.yaw0)
	s := sway * math.Sin(t*w)
	x, z = l.x0+fx*l.speed*t-fz*s, l.z0+fz*l.speed*t+fx*s
	yaw = l.yaw0 + math.Atan2(sway*w*math.Cos(t*w), l.speed)
	hx, hz := math.Cos(yaw), math.Sin(yaw)
	top := 0.0
	for _, a := range []float64{0, 8, 16, 26, 40} {
		h, _ := l.at(x+hx*a, z+hz*a)
		if l.water >= 0 {
			h = math.Max(h, l.water)
		}
		top = math.Max(top, h)
	}
	return yaw, x, z, top*l.amp + l.clear
}

func (l *land) frame(v *view, t float64) {
	s := newSky(math.Mod(l.hour+t*l.rate, 24), l.seed)
	s.clouds, s.cloudAt, s.wind = l.clouds, 0.25, 0.03
	yaw, cx, cz, cy := l.camera(t)
	focal := float64(v.ph) * 0.95
	horizon := float64(v.ph) * 0.42
	bottom := float64(max(v.ph, v.y0+v.h))
	sunH := v3(s.sun.x, 0, s.sun.z).norm()
	for i := 0; i < v.w; i++ {
		gx := float64(v.x0+i) + 0.5 - float64(v.pw)/2
		ang := math.Atan2(gx, focal)
		ca := math.Cos(ang)
		dx, dz := math.Cos(yaw+ang), math.Sin(yaw+ang)
		haze := s.hor.add(s.sunC.scale(0.12 * math.Pow(math.Max(v3(dx, 0, dz).dot(sunH), 0), 6)))
		ybuf := bottom
		// The last step drew: the next one's span is graded from its
		// colour, so the ground has no stairs; after a hidden step, the
		// span is a surface of its own, behind what hid it.
		var prev vec3
		joined := false
		z, step := 1.0, 0.12
		for z < 420 {
			h, k := l.at(cx+dx*z, cz+dz*z)
			wet := l.water >= 0 && h < l.water
			if wet {
				h = l.water
			}
			depth := z * ca
			sy := horizon + (cy-h*l.amp)/depth*focal
			if sy < ybuf {
				var c vec3
				if wet {
					// The sea: the sky again, darker looking down.
					el := (horizon - sy) / math.Hypot(focal, gx)
					r := s.color(v3(dx, -el, dz).norm(), t)
					c = r.scale(0.35 + 0.4*smooth(-0.6, 0, el)).add(v3(0.004, 0.01, 0.014))
				} else {
					nm := l.nrm[k]
					light := s.sunC.scale(math.Max(nm.dot(s.sun), 0)).
						add(s.moonC.scale(1.8 * math.Max(nm.dot(s.moon), 0))).
						add(s.amb.scale(0.3 + 0.55*nm.y))
					c = l.alb[k].mul(light)
				}
				c = mix(c, haze, 1-math.Exp(-depth*l.fog))
				top, end := max(int(math.Ceil(sy-0.5)), v.y0), min(int(math.Ceil(ybuf-0.5)), v.y0+v.h)
				span := ybuf - sy
				for y := top; y < end; y++ {
					px := c
					if joined {
						px = mix(c, prev, sat((float64(y)+0.5-sy)/span))
					}
					v.px[(y-v.y0)*v.w+i] = px
				}
				prev, joined = c, true
				ybuf = sy
				if ybuf <= float64(v.y0) {
					break
				}
			} else {
				joined = false
			}
			z += step
			step *= 1.011
		}
		end := min(int(math.Ceil(ybuf-0.5)), v.y0+v.h)
		// A pinhole's ray: the column's way across, the row's way up, in
		// the same measure, so the sun is round wherever it stands.
		reach := math.Hypot(focal, gx)
		for y := v.y0; y < end; y++ {
			el := (horizon - (float64(y) + 0.5)) / reach
			v.px[(y-v.y0)*v.w+i] = s.color(v3(dx, el, dz).norm(), t)
		}
	}
}
