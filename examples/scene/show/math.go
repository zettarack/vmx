package show

import "math"

// vec3 is a colour of light, or a point or a direction (y up).
type vec3 struct{ x, y, z float64 }

func v3(x, y, z float64) vec3 { return vec3{x, y, z} }
func gray(v float64) vec3     { return vec3{v, v, v} }

func (a vec3) add(b vec3) vec3      { return vec3{a.x + b.x, a.y + b.y, a.z + b.z} }
func (a vec3) sub(b vec3) vec3      { return vec3{a.x - b.x, a.y - b.y, a.z - b.z} }
func (a vec3) scale(s float64) vec3 { return vec3{a.x * s, a.y * s, a.z * s} }
func (a vec3) mul(b vec3) vec3      { return vec3{a.x * b.x, a.y * b.y, a.z * b.z} }
func (a vec3) dot(b vec3) float64   { return a.x*b.x + a.y*b.y + a.z*b.z }
func (a vec3) length() float64      { return math.Sqrt(a.dot(a)) }
func (a vec3) cross(b vec3) vec3 {
	return vec3{a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x}
}

func (a vec3) norm() vec3 {
	if l := a.length(); l > 0 {
		return a.scale(1 / l)
	}
	return a
}

// lum is the light's brightness as an eye weighs it.
func (a vec3) lum() float64 { return 0.2126*a.x + 0.7152*a.y + 0.0722*a.z }

func mix(a, b vec3, t float64) vec3 { return a.add(b.sub(a).scale(t)) }
func lerp(a, b, t float64) float64  { return a + (b-a)*t }
func sat(x float64) float64         { return math.Min(math.Max(x, 0), 1) }
func fract(x float64) float64       { return x - math.Floor(x) }
func sq(x float64) float64          { return x * x }

// smooth is 0 below e0, 1 above e1 and an S between (e1 may be below e0).
func smooth(e0, e1, x float64) float64 {
	t := sat((x - e0) / (e1 - e0))
	return t * t * (3 - 2*t)
}

// rot is a rotation: the three columns of its matrix.
type rot struct{ a, b, c vec3 }

// rotXY turns by ax about x, then ay about y.
func rotXY(ax, ay float64) rot {
	sx, cx := math.Sincos(ax)
	sy, cy := math.Sincos(ay)
	return rot{v3(cy, 0, -sy), v3(sy*sx, cx, cy*sx), v3(sy*cx, -sx, cy*cx)}
}

func (r rot) apply(p vec3) vec3 { return r.a.scale(p.x).add(r.b.scale(p.y)).add(r.c.scale(p.z)) }

// palette is a cosine palette: a + b cos(2 pi (c t + d)), each channel its
// own phase.
type palette struct{ a, b, c, d vec3 }

func (p palette) at(t float64) vec3 {
	f := func(a, b, c, d float64) float64 { return a + b*math.Cos(2*math.Pi*(c*t+d)) }
	return vec3{f(p.a.x, p.b.x, p.c.x, p.d.x), f(p.a.y, p.b.y, p.c.y, p.d.y), f(p.a.z, p.b.z, p.c.z, p.d.z)}
}

// palFor is a palette of the seed's: a few well-made families of phases.
func palFor(seed uint64) palette {
	phases := []vec3{
		v3(0.0, 0.33, 0.67),
		v3(0.0, 0.10, 0.20),
		v3(0.3, 0.20, 0.20),
		v3(0.8, 0.90, 0.30),
		v3(0.0, 0.15, 0.20),
		v3(0.5, 0.20, 0.25),
	}
	d := phases[int(mix64(seed)%uint64(len(phases)))]
	c := v3(1, 1, 1)
	if h01(seed, 5) < 0.4 {
		c = v3(1, 1, 0.5)
	}
	return palette{gray(0.5), gray(0.5), c, d}
}
