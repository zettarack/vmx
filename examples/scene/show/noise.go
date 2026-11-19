package show

import "math"

// mix64 is splitmix64's finish: a well-spread number from any number.
func mix64(z uint64) uint64 {
	z ^= z >> 30
	z *= 0xbf58476d1ce4e5b9
	z ^= z >> 27
	z *= 0x94d049bb133111eb
	return z ^ z>>31
}

// h01 is a number in [0, 1) from a and salt, the same every time.
func h01(a, salt uint64) float64 {
	return float64(mix64(a*0x9e3779b97f4a7c15^mix64(salt+0x7f4a7c15))>>11) / (1 << 53)
}

func lattice(ix, iy int64, seed uint64) uint64 {
	return mix64(uint64(ix)*0x632be59bd9b4e019 ^ uint64(iy)*0x8cb92ba72f3d8dd7 ^ seed)
}

// The gradients of the noise: eight ways round.
var grads = [8][2]float64{
	{1, 0}, {-1, 0}, {0, 1}, {0, -1},
	{0.7071, 0.7071}, {-0.7071, 0.7071}, {0.7071, -0.7071}, {-0.7071, -0.7071},
}

func corner(ix, iy int64, seed uint64, dx, dy float64) float64 {
	g := &grads[lattice(ix, iy, seed)&7]
	return g[0]*dx + g[1]*dy
}

func fade(t float64) float64 { return t * t * t * (t*(t*6-15) + 10) }

// perlin is gradient noise in about [-0.7, 0.7], smooth everywhere.
func perlin(x, y float64, seed uint64) float64 {
	fx, fy := math.Floor(x), math.Floor(y)
	ix, iy := int64(fx), int64(fy)
	tx, ty := x-fx, y-fy
	u, w := fade(tx), fade(ty)
	a := corner(ix, iy, seed, tx, ty)
	b := corner(ix+1, iy, seed, tx-1, ty)
	c := corner(ix, iy+1, seed, tx, ty-1)
	d := corner(ix+1, iy+1, seed, tx-1, ty-1)
	return lerp(lerp(a, b, u), lerp(c, d, u), w)
}

// perlinP is perlin over a lattice that repeats every period cells each
// way, so a map made of it has no edge.
func perlinP(x, y float64, period int64, seed uint64) float64 {
	fx, fy := math.Floor(x), math.Floor(y)
	ix, iy := int64(fx), int64(fy)
	tx, ty := x-fx, y-fy
	u, w := fade(tx), fade(ty)
	m := func(i int64) int64 { return ((i % period) + period) % period }
	x0, x1, y0, y1 := m(ix), m(ix+1), m(iy), m(iy+1)
	a := corner(x0, y0, seed, tx, ty)
	b := corner(x1, y0, seed, tx-1, ty)
	c := corner(x0, y1, seed, tx, ty-1)
	d := corner(x1, y1, seed, tx-1, ty-1)
	return lerp(lerp(a, b, u), lerp(c, d, u), w)
}

// fbm is octaves of perlin, each twice as fine and half as strong, in
// about [-1, 1].
func fbm(x, y float64, oct int, seed uint64) float64 {
	s, a, n := 0.0, 1.0, 0.0
	for i := 0; i < oct; i++ {
		s += a * perlin(x, y, seed+uint64(i)*0x1000193)
		n += a
		x, y = 1.6*x+1.2*y+5.3, -1.2*x+1.6*y+1.7
		a *= 0.5
	}
	return s / n * 1.4
}

// noise1 is one-dimensional gradient noise in about [-1, 1].
func noise1(x float64, seed uint64) float64 {
	fx := math.Floor(x)
	i := int64(fx)
	t := x - fx
	g0 := h01(uint64(i), seed)*2 - 1
	g1 := h01(uint64(i+1), seed)*2 - 1
	return 2 * lerp(g0*t, g1*(t-1), fade(t))
}

// fbm1 is octaves of noise1, in about [-1, 1].
func fbm1(x float64, oct int, seed uint64) float64 {
	s, a, n := 0.0, 1.0, 0.0
	for i := 0; i < oct; i++ {
		s += a * noise1(x, seed+uint64(i)*977)
		n += a
		x = x*2.07 + 3.1
		a *= 0.5
	}
	return s / n
}
