package show

import "math"

// grid is a race on a floor of light: the floor's lines running to a
// glowing horizon, the camera gliding over it, and cycles of light on it,
// each laying a wall of light behind it that fades, turning only at right
// angles; a quiet line of facts along the horizon.
type grid struct {
	seed   uint64
	speed  float64 // the camera's, cells a second
	hues   []vec3  // each cycle's light, in the scene's own colours
	off    []float64
	line   string
	floorC vec3
}

const (
	gridEye     = 1.25 // the camera's height, in cells
	gridWall    = 0.4  // a trail's height
	gridTrail   = 16   // cells a trail keeps
	gridLife    = 20.0 // seconds a cycle runs before it starts again ahead
	gridHorizon = 0.12 // where the horizon sits, up from the middle
)

func newGrid(seed uint64, v *view) scene {
	g := &grid{seed: seed, speed: 2.0 + 1.4*h01(seed, 2), line: scroll(seed)}
	cyan, orange, white := v3(0.2, 0.85, 1.1), v3(1.2, 0.45, 0.06), v3(0.9, 0.95, 1.1)
	n := 4 + int(h01(seed, 1)*4)
	for i := 0; i < n; i++ {
		c := cyan
		switch h := h01(seed, uint64(10+i)); {
		case h < 0.3:
			c = orange
		case h < 0.4:
			c = white
		}
		g.hues = append(g.hues, c)
		g.off = append(g.off, gridLife*float64(i)/float64(n))
	}
	g.floorC = v3(0.08, 0.5, 0.75)
	if h01(seed, 3) < 0.3 {
		g.floorC = v3(0.6, 0.25, 0.7)
	}
	return g
}

// wall is a straight piece of a trail on the floor, x0..x1 at z0 or
// z0..z1 at x0, lit from b0 at its start to b1 at its end.
type wall struct {
	x0, z0, x1, z1 float64
	b0, b1         float64
	c              vec3
}

// cycle is cycle i's trail at t, and its head: a walk on the floor's
// lines from a start ahead of the camera, a cell at a time, mostly
// forward and now and then a turn to a side and back, every turn its
// seed's. A cycle runs for gridLife seconds, fading in at its start and
// out at its end, then starts again ahead.
func (g *grid) cycle(i int, t float64) (walls []wall, hx, hz, fade float64) {
	tt := t + g.off[i]
	life := math.Floor(tt / gridLife)
	lt := tt - life*gridLife
	s := mix64(g.seed + uint64(i)*0x9e3779b1 + uint64(int64(life))*0x51ed27)
	start := t - lt // when this life began, on the show's clock
	x := math.Round((h01(s, 1) - 0.5) * 14)
	z := math.Round(start*g.speed + 6 + h01(s, 2)*10)
	pace := g.speed * (1.15 + 0.25*h01(s, 3))
	steps := lt * pace
	n := int(steps)
	type pt struct{ x, z float64 }
	pts := make([]pt, 0, n+2)
	pts = append(pts, pt{x, z})
	dir, side := 0, 0
	for k := 0; k <= n; k++ {
		h := h01(s, uint64(100+k))
		switch {
		case dir == 0 && h < 0.2:
			dir, side = 1, 0
			if h01(s, uint64(5000+k)) < 0.5 {
				dir = -1
			}
		case dir != 0 && (side >= 1 && h < 0.45 || side >= 4):
			dir = 0
		}
		step := 1.0
		if k == n {
			step = steps - float64(n)
		}
		if dir == 0 {
			z += step
		} else {
			x += float64(dir) * step
			side++
		}
		pts = append(pts, pt{x, z})
	}
	fade = smooth(0, 1, lt) * smooth(gridLife, gridLife-2, lt)
	hx, hz = x, z
	// The last gridTrail cells, as straight pieces, brighter toward the head.
	first := max(len(pts)-1-gridTrail, 0)
	total := float64(len(pts) - 1 - first)
	for a := first; a < len(pts)-1; {
		b := a + 1
		for b+1 < len(pts) && (pts[b+1].x == pts[a].x) == (pts[b].x == pts[a].x) && (pts[b+1].z == pts[a].z) == (pts[b].z == pts[a].z) {
			b++
		}
		w := wall{x0: pts[a].x, z0: pts[a].z, x1: pts[b].x, z1: pts[b].z, c: g.hues[i]}
		if total > 0 {
			w.b0, w.b1 = float64(a-first)/total, float64(b-first)/total
		}
		walls = append(walls, w)
		a = b
	}
	return walls, hx, hz, fade
}

func (g *grid) frame(v *view, t float64) {
	camZ := t * g.speed
	camX := 1.5 * math.Sin(t*0.09)
	type cyc struct {
		walls []wall
		fade  float64
	}
	cycles := make([]cyc, len(g.hues))
	heads := make([][3]float64, len(g.hues))
	for i := range g.hues {
		w, hx, hz, f := g.cycle(i, t)
		cycles[i] = cyc{w, f}
		heads[i] = [3]float64{hx, hz, f}
	}
	v.each(func(x, y float64) vec3 {
		rd := v3(x, y-gridHorizon, 1.1)
		var c vec3
		far := math.Inf(1)
		if rd.y < 0 {
			// The floor: dark, its lines lit, each as wide as a pixel there.
			s := -gridEye / rd.y
			far = s
			fx, fz := camX+rd.x*s, camZ+rd.z*s
			foot := math.Max(s/float64(v.ph)*1.6, 0.012)
			lx := math.Abs(fract(fx+0.5) - 0.5)
			lz := math.Abs(fract(fz+0.5) - 0.5)
			line := math.Exp(-sq(lx/foot)) + math.Exp(-sq(lz/foot)) + 0.15*(math.Exp(-sq(lx/(5*foot)))+math.Exp(-sq(lz/(5*foot))))
			c = gray(0.003).add(g.floorC.scale(0.55 * line * math.Exp(-s*0.025)))
		} else {
			// The sky: black, a little light low down.
			c = g.floorC.scale(0.04 * math.Exp(-rd.y*8))
		}
		c = c.add(g.floorC.scale(0.5 * math.Exp(-sq(rd.y/0.012))))
		// The walls of light, nearest over the rest.
		for _, cy := range cycles {
			if cy.fade <= 0 {
				continue
			}
			for _, w := range cy.walls {
				var s, along float64
				if w.z0 == w.z1 { // across: the plane z = z0
					s = (w.z0 - camZ) / rd.z
					hx := camX + rd.x*s
					lo, hi := math.Min(w.x0, w.x1), math.Max(w.x0, w.x1)
					if s <= 0.3 || hx < lo || hx > hi {
						continue
					}
					along = (hx - w.x0) / (w.x1 - w.x0 + 1e-9)
				} else { // along: the plane x = x0
					if math.Abs(rd.x) < 1e-6 {
						continue
					}
					s = (w.x0 - camX) / rd.x
					hz := camZ + rd.z*s
					lo, hi := math.Min(w.z0, w.z1), math.Max(w.z0, w.z1)
					if s <= 0.3 || hz < lo || hz > hi {
						continue
					}
					along = (hz - w.z0) / (w.z1 - w.z0 + 1e-9)
				}
				hy := gridEye + rd.y*s
				if hy < 0 || hy > gridWall || s > far {
					continue
				}
				b := lerp(w.b0, w.b1, sat(along))
				glow := (0.35 + 0.65*b*b) * cy.fade * math.Exp(-s*0.02)
				edge := 1 + 2.2*math.Exp(-sq((gridWall-hy)/0.035))
				c = mix(c, w.c.scale(0.9*glow*edge), 0.85)
				far = s
			}
		}
		return c
	})
	// Each cycle's head: a bright light where it runs.
	for _, h := range heads {
		dz := h[1] - camZ
		if dz < 0.5 || h[2] <= 0 {
			continue
		}
		px := (h[0]-camX)/dz*1.1*float64(v.ph) + float64(v.pw)/2
		py := float64(v.ph)/2 - ((gridWall*0.6-gridEye)/dz*1.1+gridHorizon)*float64(v.ph)
		r := math.Max(3, 0.9/dz*float64(v.ph)*0.25)
		v.glow(px, py, r, gray(1.4*h[2]))
	}
	// Along the horizon, quiet: what is true here.
	s := max(1, v.ph/300)
	v.scroller(g.line, int(float64(v.ph)*(0.5-gridHorizon))-7*s-6, s, 70, 3, t, g.floorC.scale(0.6))
}
