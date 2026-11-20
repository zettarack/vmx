package show

import "math"

// The materials: forty windows onto forty pieces of one craft, each tile
// its own piece (a block of stone, a length of wood, a worn gem), and one
// light passing slowly across the wall that each piece catches in turn:
// the light is what the forty share. (2026-11-19, the operator: "stone
// wood and stained glass, but not like church stained glass, more like old
// worn dust blown gems".)

// piece is a tile's own: the seed of its piece, and the map from a point of
// the picture to its window, u across (0 to 1) and w down.
type piece struct {
	seed           uint64
	x0, y0, tw, th float64
	pw, ph         float64
}

func pieceOf(v *view, seed uint64) piece {
	col, row := 0, 0
	if v.w > 0 {
		col = v.x0 / v.w
	}
	if v.h > 0 {
		row = v.y0 / v.h
	}
	return piece{mix64(seed + uint64(row*97+col+1)*0x9e3779b97f4a7c15),
		float64(v.x0), float64(v.y0), float64(max(v.w, 1)), float64(max(v.h, 1)), float64(v.pw), float64(v.ph)}
}

// uw is where picture point (x, y) falls in the tile's window, and how far
// (in the window's heights) it is from the window's nearest edge.
func (pc piece) uw(x, y float64) (u, w, edge float64) {
	u = (x*pc.ph + pc.pw/2 - pc.x0) / pc.tw
	w = (pc.ph/2 - y*pc.ph - pc.y0) / pc.th
	a := pc.tw / pc.th
	edge = math.Min(math.Min(u, 1-u)*a, math.Min(w, 1-w))
	return u, w, edge
}

// passingLight is where the light is at t: a point above the wall that
// crosses it slowly from side to side and back, low enough to rake across
// each piece's surface.
func passingLight(v *view, t float64) vec3 {
	a := v.aspect()
	return vec3{0.6 * a * math.Sin(t*0.11), 0.26 * math.Sin(t*0.07+1.1), 0.5}
}

// litBy is a surface's light under the passing light L: its colour, its
// normal n, how polished it is (0 matte to 1 glass), at picture point p on
// the wall's plane, seen from straight in front.
func litBy(p vec3, n, albedo vec3, gloss float64, L vec3) vec3 {
	d := L.sub(p)
	dist := d.length()
	l := d.scale(1 / dist)
	diff := math.Max(n.dot(l), 0)
	fall := 1 / (1 + 2.4*dist*dist)
	h := l.add(vec3{0, 0, 1}).norm()
	spec := math.Pow(math.Max(n.dot(h), 0), 10+140*gloss) * gloss
	return albedo.scale(0.05 + 1.6*diff*fall).add(vec3{1, 0.96, 0.9}.scale(spec * 2.4 * fall))
}

// bumped is the normal of a height field h at (x, y), steep by k.
func bumped(h func(x, y float64) float64, x, y, k float64) vec3 {
	const e = 0.0015
	h0 := h(x, y)
	return vec3{-(h(x+e, y) - h0) / e * k, -(h(x, y+e) - h0) / e * k, 1}.norm()
}

// wornEdge is how a piece falls away into the joint at its edge: the
// height lost within the last part of its window, and the joint itself
// dark.
func wornEdge(edge, width float64) (drop, joint float64) {
	s := sat(1 - edge/width)
	return s * s * 0.5, smooth(0.012, 0.0, edge)
}

type stone struct{ seed uint64 }

func newStone(seed uint64, _ *view) scene { return &stone{seed} }

// frame: a dressed block, one of granite, sandstone, slate or marble.
func (s *stone) frame(v *view, t float64) {
	pc := pieceOf(v, s.seed)
	kind := pc.seed % 4
	base := [...]vec3{{0.40, 0.39, 0.38}, {0.60, 0.50, 0.36}, {0.27, 0.30, 0.33}, {0.80, 0.79, 0.76}}[kind]
	gloss := [...]float64{0.25, 0.05, 0.15, 0.6}[kind]
	sd := pc.seed
	L := passingLight(v, t)
	v.each(func(x, y float64) vec3 {
		_, _, edge := pc.uw(x, y)
		drop, joint := wornEdge(edge, 0.07)
		h := func(x, y float64) float64 {
			return 0.5*fbm(x*8, y*8, 3, sd) + 0.08*fbm(x*42, y*42, 2, sd+7) - drop
		}
		n := bumped(h, x, y, 0.05+0.04*float64(kind%2))
		c := base
		switch kind {
		case 0: // granite: flecks dark and light
			f := fbm(x*70, y*70, 2, sd+3)
			if f > 0.32 {
				c = c.scale(0.45)
			} else if f < -0.42 {
				c = c.scale(1.45)
			}
		case 1: // sandstone: beds laid down in bands
			c = c.scale(0.85 + 0.25*math.Sin(y*46+2.5*fbm(x*3, y*3, 3, sd+5)))
		case 2: // slate: fine cleavage along its beds
			c = c.scale(0.9 + 0.2*fbm(x*6, y*60, 2, sd+9))
		default: // marble: veins through the white
			vein := math.Abs(math.Sin(x*5 + y*3 + 3.2*fbm(x*2.5, y*2.5, 4, sd+11)))
			c = c.scale(1 - 0.55*smooth(0.06, 0, vein))
		}
		// dust and wear in the hollows
		c = c.scale(0.8 + 0.3*sat(0.5+fbm(x*14, y*14, 2, sd+13)))
		out := litBy(vec3{x, y, 0}, n, c, gloss, L)
		return out.scale(1 - 0.9*joint)
	})
}

type wood struct{ seed uint64 }

func newWood(seed uint64, _ *view) scene { return &wood{seed} }

// frame: a length of plank, one of oak, walnut, cherry, maple or ebony,
// its rings, its grain, a knot now and then, under a worn varnish.
func (s *wood) frame(v *view, t float64) {
	pc := pieceOf(v, s.seed)
	kind := pc.seed % 5
	early := [...]vec3{{0.62, 0.45, 0.25}, {0.36, 0.24, 0.15}, {0.66, 0.35, 0.22}, {0.82, 0.68, 0.48}, {0.12, 0.09, 0.07}}[kind]
	late := early.scale(0.62)
	sd := pc.seed
	along := h01(sd, 3) < 0.7 // grain across the window, or down it
	cx, cy := 4*(h01(sd, 5)-0.5), 2.5+2*h01(sd, 6)
	knot := h01(sd, 7) < 0.35
	kx, ky := pc.x0/pc.ph-pc.pw/pc.ph/2+(0.25+0.5*h01(sd, 8))*pc.tw/pc.ph, pc.ph/2/pc.ph-(pc.y0+(0.25+0.5*h01(sd, 9))*pc.th)/pc.ph
	L := passingLight(v, t)
	v.each(func(x, y float64) vec3 {
		_, _, edge := pc.uw(x, y)
		drop, joint := wornEdge(edge, 0.05)
		gx, gy := x, y
		if !along {
			gx, gy = y, x
		}
		// the rings: distance from the log's heart, far off the plank
		r := math.Hypot((gx-cx)*0.18, gy-cy) + 0.05*fbm(gx*4, gy*10, 3, sd)
		kd := 1.0
		if knot {
			// the grain swirls round a knot, smoothly (a step in the
			// rings drew a circle, 2026-11-19)
			d := math.Hypot(x-kx, y-ky)
			r += 0.035 * math.Exp(-d*d*500)
			kd = d
		}
		ring := 0.5 + 0.5*math.Sin(r*95)
		c := early.add(late.sub(early).scale(ring * ring))
		if knot {
			c = c.scale(1 - 0.6*smooth(0.03, 0.006, kd))
		}
		c = c.scale(0.9 + 0.18*fbm(gx*3, gy*80, 2, sd+3))
		h := func(x, y float64) float64 {
			gx, gy := x, y
			if !along {
				gx, gy = y, x
			}
			return 0.2*fbm(gx*3, gy*80, 2, sd+3) - drop
		}
		n := bumped(h, x, y, 0.02)
		// the varnish worn thin in places
		gloss := 0.25 + 0.3*sat(0.5+fbm(x*9, y*9, 2, sd+21))
		return litBy(vec3{x, y, 0}, n, c, gloss, L).scale(1 - 0.9*joint)
	})
}

type gems struct{ seed uint64 }

func newGems(seed uint64, _ *view) scene { return &gems{seed} }

// The gems' colours: ruby, emerald, sapphire, amethyst, citrine, topaz,
// aquamarine, garnet, each a little faded by its years.
var gemColours = [...]vec3{
	{0.72, 0.08, 0.14}, {0.10, 0.55, 0.30}, {0.12, 0.22, 0.70}, {0.48, 0.20, 0.62},
	{0.85, 0.62, 0.12}, {0.80, 0.42, 0.14}, {0.30, 0.68, 0.78}, {0.50, 0.08, 0.10},
}

// frame: an old cut stone set in dark worn metal: its table and its facets
// catching the passing light, its body lit from within as the light comes
// near, scratched and chipped, and dust blown across the whole wall.
func (s *gems) frame(v *view, t float64) {
	pc := pieceOf(v, s.seed)
	sd := pc.seed
	col := gemColours[sd%uint64(len(gemColours))]
	col = col.scale(0.85).add(vec3{0.06, 0.06, 0.06})
	sides := []float64{6, 8, 8, 10, 12}[mix64(sd+1)%5]
	a := pc.tw / pc.th
	R := 0.36 * (0.9 + 0.2*h01(sd, 2))
	turn := h01(sd, 4) * math.Pi
	L := passingLight(v, t)
	// where the gem's middle is on the picture, for its glow
	gcx := (pc.x0 + pc.tw/2 - pc.pw/2) / pc.ph
	gcy := (pc.ph/2 - pc.y0 - pc.th/2) / pc.ph
	near := math.Exp(-((L.x-gcx)*(L.x-gcx) + (L.y-gcy)*(L.y-gcy)) * 9)
	v.each(func(x, y float64) vec3 {
		u, w, edge := pc.uw(x, y)
		_, joint := wornEdge(edge, 0.04)
		gx, gy := (u-0.5)*a, 0.5-w
		ang := math.Atan2(gy, gx) + turn
		rad := math.Hypot(gx, gy)
		sec := 2 * math.Pi / sides
		k := math.Mod(ang+20*math.Pi, sec)
		// distance along the nearest edge's normal, in circumradii: a
		// polygon with straight edges (the cosine the other way round drew
		// scallops, 2026-11-19)
		rp := rad * math.Cos(k-sec/2) / math.Cos(sec/2)
		// the dust, blown across the wall
		dust := sat(0.5+0.9*fbm(x*18+t*0.05, y*18+t*0.012, 3, 99)) * 0.55
		if rp > R*(1+0.025*fbm(gx*40, gy*40, 1, sd+3)) {
			// the setting: dark metal, a bezel round the stone
			bez := smooth(0.025, 0.0, math.Abs(rp-R*1.04))
			metal := vec3{0.05, 0.045, 0.04}.scale(0.8 + 0.4*sat(0.5+fbm(x*30, y*30, 2, sd+5)))
			n := vec3{0, 0, 1}
			if bez > 0 {
				d := 0.5 * bez
				n = vec3{math.Cos(ang-turn) * d, math.Sin(ang-turn) * d, 1}.norm()
				metal = metal.add(vec3{0.16, 0.14, 0.11}.scale(bez))
			}
			c := litBy(vec3{x, y, 0}, n, metal.add(vec3{0.32, 0.29, 0.25}.scale(dust*0.3)), 0.45*bez, L)
			return c.scale(1 - 0.9*joint)
		}
		// the table, flat, then two rings of facets round it
		var n vec3
		ring := rp / R
		facet := math.Floor(math.Mod(ang+20*math.Pi, 2*math.Pi) / (sec / 2))
		fa := (facet+0.5)*sec/2 - turn
		switch {
		case ring < 0.48:
			n = vec3{0, 0, 1}
		case ring < 0.78:
			n = vec3{math.Cos(fa) * 0.42, math.Sin(fa) * 0.42, 1}.norm()
		default:
			n = vec3{math.Cos(fa) * 0.85, math.Sin(fa) * 0.85, 1}.norm()
		}
		// the body: dark at heart, lit from within as the light comes near,
		// each facet throwing back its own share
		fire := 0.7 + 0.6*h01(sd, uint64(20+facet))
		if ring < 0.48 {
			fire = 1
		}
		body := col.scale((0.14 + 0.85*near*(0.55+0.45*(1-ring))) * fire)
		c := litBy(vec3{x, y, 0}, n, body.scale(0.6), 0.95*(1-dust), L).add(body.scale(0.5))
		// the years: fine scratches, and the dust over it
		sc := math.Abs(math.Sin(gx*31+gy*7+h01(sd, 9)*6)) * math.Abs(math.Sin(gx*-11+gy*29+h01(sd, 10)*6))
		c = c.add(vec3{0.5, 0.5, 0.5}.scale(smooth(0.04, 0.0, sc) * 0.08))
		// the cut's edges: between the facets, and round the table and the
		// rings
		fe := math.Mod(math.Mod(ang+20*math.Pi, 2*math.Pi), sec/2) / (sec / 2)
		line := math.Max(smooth(0.035, 0.0, math.Abs(ring-0.48)), smooth(0.03, 0.0, math.Abs(ring-0.78)))
		if ring >= 0.48 {
			line = math.Max(line, smooth(0.05, 0.0, math.Min(fe, 1-fe)))
		}
		c = c.scale(1 - 0.45*line)
		c = c.scale(1 - 0.5*dust).add(vec3{0.30, 0.27, 0.22}.scale(dust * 0.18))
		return c.scale(1 - 0.9*joint)
	})
}
