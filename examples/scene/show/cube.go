package show

import (
	"math"
	"sort"
	"strconv"
)

// cube is a ship that is a cube of cells, its nodes, under attack: fighters
// fly round it and fire at the face that looks their way; a cell that is
// hit flashes and goes, leaving a hole through which the cube's core
// shows, and grows back from the core on a beam, and the count of nodes
// comes back. A machine lost, and made again from its golden in an
// instant. The attack comes in waves over the part; the eye goes slowly
// round the cube as it turns.
type cube struct {
	seed  uint64
	n     int // cells along a face's edge
	fleet []fighter
	seam  vec3 // the hull's own light
	spin  float64
	tilt  float64
	dist  float64
	az0   float64
	neb   palette
	text  int // the count's cell, in pixels

	// This frame's: the cells' states, the shots that matter, the eye,
	// the cube's turn, and the depth of each of the tile's pixels.
	cells   []cellAt
	shots   []shot
	eye     vec3
	r, u, f vec3
	turn    rot
	z       []float64
}

// fighter is one attacker: its orbit round the cube and when it fires.
type fighter struct {
	r, w, ph, bob float64 // the orbit's radius, turn a second, phase, rise and fall
	plane         rot     // the orbit's tilt
	every, first  float64 // seconds between its shots, and the first's time
}

// shot is one bolt: who fired it, when it left and when it lands, the cell
// it hits, where it left from, and how long the cell stays gone.
type shot struct {
	ship       int
	fire, land float64
	cell       int
	from       vec3
	hold       float64
}

// A cell is whole, aimed at, hit, gone or growing back.
const (
	cellOK = iota
	cellAimed
	cellHit
	cellGone
	cellGrowing
)

// cellAt is a cell's state and how far into it the cell is, 0 to 1 (for a
// whole cell, the glow of having just grown back).
type cellAt struct {
	state int
	k     float64
}

// The bolt's flight, the hit's flash, the growing back and the glow after
// it, in seconds; and the longest a cell's story can take.
const (
	boltFor  = 0.7
	hitFor   = 0.35
	growFor  = 1.6
	freshFor = 2.5
	storyFor = boltFor + hitFor + 3.6 + growFor + freshFor
)

// The fighters' and bolts' colours, the growing back's and an aimed cell's.
var (
	boltLight  = v3(1, 0.25, 0.35)
	engine     = v3(1, 0.62, 0.22)
	growLight  = v3(0.13, 0.9, 1)
	aimedLight = v3(1, 0.75, 0.3)
)

func newCube(seed uint64, v *view) scene {
	c := &cube{seed: seed}
	c.n = []int{5, 6, 8}[int(mix64(seed)%3)]
	seams := []vec3{v3(0.24, 1, 0.53), v3(0.13, 0.9, 1), v3(0.45, 1, 0.3)}
	c.seam = seams[int(mix64(seed+1)%uint64(len(seams)))]
	c.spin = (0.05 + 0.05*h01(seed, 2)) * []float64{1, -1}[int(mix64(seed+3)%2)]
	c.tilt = 0.25 + 0.35*h01(seed, 4)
	c.dist = 4.3 + 0.6*h01(seed, 5)
	c.az0 = 2 * math.Pi * h01(seed, 6)
	c.neb = palFor(mix64(seed + 7))
	c.text = max(2, v.ph/200)
	ships := 4 + int(mix64(seed+8)%4)
	for k := 0; k < ships; k++ {
		s := mix64(seed + 100 + uint64(k))
		c.fleet = append(c.fleet, fighter{
			r:     1.95 + 0.85*h01(s, 1),
			w:     (0.22 + 0.25*h01(s, 2)) * []float64{1, -1}[int(s%2)],
			ph:    2 * math.Pi * h01(s, 3),
			bob:   0.3 + 0.5*h01(s, 4),
			plane: rotXY(0.9*(h01(s, 5)-0.5), 2*math.Pi*h01(s, 6)),
			every: 1.3 + 1.4*h01(s, 7),
			first: 2 + 4*h01(s, 8),
		})
	}
	c.cells = make([]cellAt, 6*c.n*c.n)
	return c
}

// at is fighter k's place at t, and the way it is going.
func (c *cube) at(k int, t float64) (vec3, vec3) {
	fg := &c.fleet[k]
	a := fg.ph + fg.w*t
	sn, cs := math.Sincos(a)
	p := v3(cs*fg.r, fg.bob*math.Sin(a*2.3+fg.ph), sn*fg.r*0.8)
	d := v3(-sn*fg.r, fg.bob*2.3*math.Cos(a*2.3+fg.ph), cs*fg.r*0.8).scale(fg.w)
	return fg.plane.apply(p), fg.plane.apply(d).norm()
}

// turnAt is the cube's turn at t.
func (c *cube) turnAt(t float64) rot { return rotXY(c.tilt, c.spin*t) }

// wave is how hard the attack is at t: lulls and storms over the part.
func wave(t float64) float64 {
	return 0.3 + 0.7*smooth(-0.3, 0.6, math.Sin(t*2*math.Pi/95)*0.6+math.Sin(t*2*math.Pi/37)*0.4)
}

// fired says whether fighter k's shot j was fired (the wave allowing it),
// when, and when it lands.
func (c *cube) fired(k, j int) (bool, float64, float64) {
	fg := &c.fleet[k]
	fire := fg.first + float64(j)*fg.every
	return h01(mix64(c.seed+uint64(k)*977), uint64(j)) < wave(fire), fire, fire + boltFor
}

// aim is the cell fighter k's shot j hits: a cell of the face that looks
// most toward the fighter as it fires.
func (c *cube) aim(k, j int, fire float64) (int, vec3) {
	from, _ := c.at(k, fire)
	turn := c.turnAt(fire)
	best, face := -2.0, 0
	for f := 0; f < 6; f++ {
		if d := turn.apply(faceNormal(f)).dot(from.norm()); d > best {
			best, face = d, f
		}
	}
	h := mix64(c.seed + uint64(k)*0x9e37 + uint64(j)*0x51ed)
	i, jj := int(h%uint64(c.n)), int((h>>16)%uint64(c.n))
	return face*c.n*c.n + jj*c.n + i, from
}

// faceNormal is face f's outward normal in the cube's own frame: +x, -x,
// +y, -y, +z, -z.
func faceNormal(f int) vec3 {
	s := 1.0
	if f%2 == 1 {
		s = -1
	}
	switch f / 2 {
	case 0:
		return v3(s, 0, 0)
	case 1:
		return v3(0, s, 0)
	}
	return v3(0, 0, s)
}

// onFace is the point of face f at (u, v), each -1 to 1, in the cube's own
// frame: for x faces u runs along z and v along y; for y faces, x and z;
// for z faces, x and y.
func onFace(f int, u, v float64) vec3 {
	n := faceNormal(f)
	switch f / 2 {
	case 0:
		return v3(n.x, v, u)
	case 1:
		return v3(u, n.y, v)
	}
	return v3(u, v, n.z)
}

// cellMiddle is cell i's middle in the cube's own frame.
func (c *cube) cellMiddle(i int) vec3 {
	f, rest := i/(c.n*c.n), i%(c.n*c.n)
	cu, cv := rest%c.n, rest/c.n
	return onFace(f, (float64(cu)+0.5)/float64(c.n)*2-1, (float64(cv)+0.5)/float64(c.n)*2-1)
}

// state works out every cell's state at t from the shots that bear on it,
// in the order they land, and how many cells have grown back since the
// part began.
func (c *cube) state(t float64) int {
	c.shots = c.shots[:0]
	grown := 0
	for k := range c.fleet {
		for j := 0; ; j++ {
			on, fire, land := c.fired(k, j)
			if fire > t {
				break
			}
			if !on {
				continue
			}
			h := mix64(c.seed + uint64(k)*0x2545 + uint64(j)*0x9e37)
			hold := 1.4 + 2.2*float64(h>>11)/(1<<53)
			if land+hitFor+hold+growFor <= t {
				grown++
			}
			if land < t-storyFor {
				continue
			}
			cell, from := c.aim(k, j, fire)
			c.shots = append(c.shots, shot{ship: k, fire: fire, land: land, cell: cell, from: from, hold: hold})
		}
	}
	sort.Slice(c.shots, func(a, b int) bool { return c.shots[a].land < c.shots[b].land })
	for i := range c.cells {
		c.cells[i] = cellAt{}
	}
	for _, s := range c.shots {
		cl := &c.cells[s.cell]
		in := t - s.land
		switch {
		case in < 0:
			if cl.state == cellOK {
				*cl = cellAt{cellAimed, (t - s.fire) / boltFor}
			}
		case in < hitFor:
			*cl = cellAt{cellHit, in / hitFor}
		case in < hitFor+s.hold:
			*cl = cellAt{cellGone, (in - hitFor) / s.hold}
		case in < hitFor+s.hold+growFor:
			*cl = cellAt{cellGrowing, (in - hitFor - s.hold) / growFor}
		case in < hitFor+s.hold+growFor+freshFor:
			*cl = cellAt{cellOK, 1 - (in-hitFor-s.hold-growFor)/freshFor}
		}
	}
	return grown
}

// project is where the point p falls in the picture, and how far it is
// from the eye; ok is false behind the eye.
func (c *cube) project(v *view, p vec3) (gx, gy, depth float64, ok bool) {
	q := p.sub(c.eye)
	cf := q.dot(c.f)
	if cf < 0.05 {
		return 0, 0, 0, false
	}
	k := 1.05 / cf * float64(v.ph)
	return float64(v.pw)/2 + q.dot(c.r)*k, float64(v.ph)/2 - q.dot(c.u)*k, q.length(), true
}

// seen says whether a light at picture point (gx, gy), depth away, is in
// front of the hull there (or the point is not this tile's).
func (c *cube) seen(v *view, gx, gy, depth float64) bool {
	x, y := int(gx)-v.x0, int(gy)-v.y0
	if x < 0 || y < 0 || x >= v.w || y >= v.h {
		return true
	}
	return depth <= c.z[y*v.w+x]+0.02
}

// streak draws a line of light from a to b, its glow r pixels wide, each
// point of it hidden where the hull is in front of it when hide is set.
func (c *cube) streak(v *view, a, b vec3, r float64, col vec3, hide bool) {
	ax, ay, _, ok1 := c.project(v, a)
	bx, by, _, ok2 := c.project(v, b)
	if !ok1 || !ok2 {
		return
	}
	lo, hi := math.Min(ax, bx)-r, math.Max(ax, bx)+r
	top, bot := math.Min(ay, by)-r, math.Max(ay, by)+r
	if hi < float64(v.x0) || lo > float64(v.x0+v.w) || bot < float64(v.y0) || top > float64(v.y0+v.h) {
		return
	}
	steps := max(2, int(math.Hypot(bx-ax, by-ay)/(r*0.6)))
	for i := 0; i <= steps; i++ {
		k := float64(i) / float64(steps)
		gx, gy, depth, ok := c.project(v, mix(a, b, k))
		if ok && (!hide || c.seen(v, gx, gy, depth)) {
			v.glow(gx, gy, r, col)
		}
	}
}

func (c *cube) frame(v *view, t float64) {
	grown := c.state(t)
	c.turn = c.turnAt(t)
	az := c.az0 + 0.025*t
	el := 0.22 + 0.12*math.Sin(t*0.013)
	d := c.dist + 0.7*math.Sin(2*math.Pi*t/SceneLen)
	c.eye = v3(math.Cos(el)*math.Sin(az), math.Sin(el), -math.Cos(el)*math.Cos(az)).scale(d)
	c.f = c.eye.scale(-1).norm()
	c.r = c.f.cross(v3(0, 1, 0)).norm()
	c.u = c.r.cross(c.f)
	if len(c.z) != v.w*v.h {
		c.z = make([]float64, v.w*v.h)
	}
	light := v3(-0.5, 0.75, -0.45).norm()
	// The eye in the cube's own frame: a ray is followed there.
	eye := c.untwist(c.eye)
	i := 0
	v.each(func(x, y float64) vec3 {
		dir := c.r.scale(x).add(c.u.scale(y)).add(c.f.scale(1.05)).norm()
		col, depth := c.hull(eye, c.untwist(dir), x, y, t, light)
		c.z[i] = depth
		i++
		return col
	})
	// The beams the cells grow back on, from the core, seen through the
	// hull as the light they are.
	for ci, cl := range c.cells {
		if cl.state == cellGrowing {
			end := c.turn.apply(c.cellMiddle(ci))
			c.streak(v, vec3{}, end, 2.2, growLight.scale(0.6*(1-cl.k*0.7)), false)
		}
	}
	// The fighters, their engines, and their bolts.
	for k := range c.fleet {
		p, dir := c.at(k, t)
		c.fighter(v, p, dir, light)
	}
	for _, s := range c.shots {
		if t < s.fire || t >= s.land+1.2 {
			continue
		}
		end := c.turn.apply(c.cellMiddle(s.cell))
		if t < s.land {
			u := (t - s.fire) / boltFor
			c.streak(v, mix(s.from, end, math.Max(0, u-0.3)), mix(s.from, end, u), 3*float64(v.ph)/600+0.5, boltLight, true)
			continue
		}
		// The hit: a flash, and sparks thrown off the face.
		in := t - s.land
		nrm := c.turn.apply(faceNormal(s.cell / (c.n * c.n)))
		if gx, gy, depth, ok := c.project(v, end.add(nrm.scale(0.05))); ok && c.seen(v, gx, gy, depth) && in < 0.5 {
			v.glow(gx, gy, (3+20*in)*float64(v.ph)/600, v3(1, 0.7, 0.5).scale(1.6*(1-in/0.5)))
		}
		h := mix64(c.seed + uint64(s.cell)*0x9e37 + uint64(s.land*100))
		for sp := 0; sp < 12; sp++ {
			hs := mix64(h + uint64(sp))
			dir := nrm.add(v3(h01(hs, 1)-0.5, h01(hs, 2)-0.5, h01(hs, 3)-0.5).scale(1.6)).norm()
			reach := (0.3 + 0.6*h01(hs, 4)) * (1 - math.Exp(-3*in))
			if gx, gy, depth, ok := c.project(v, end.add(dir.scale(reach))); ok && c.seen(v, gx, gy, depth) {
				v.glow(gx, gy, 1.6*float64(v.ph)/600+0.6, v3(1, 0.5, 0.45).scale(1.4*sat(1-in/1.2)))
			}
		}
	}
	c.count(v, grown)
}

// untwist turns a point or a way from the world into the cube's own frame.
func (c *cube) untwist(p vec3) vec3 {
	return v3(c.turn.a.dot(p), c.turn.b.dot(p), c.turn.c.dot(p))
}

// hull is the light that comes back along the ray from o going d (the
// cube's own frame) and how far it went: the hull, a hole in it and what
// is inside it, or the sky.
func (c *cube) hull(o, d vec3, x, y, t float64, light vec3) (vec3, float64) {
	near, far, axis := -math.MaxFloat64, math.MaxFloat64, 0
	for a := 0; a < 3; a++ {
		oa, da := comp(o, a), comp(d, a)
		if math.Abs(da) < 1e-12 {
			if oa < -1 || oa > 1 {
				return c.sky(x, y, t), math.Inf(1)
			}
			continue
		}
		t1, t2 := (-1-oa)/da, (1-oa)/da
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > near {
			near, axis = t1, a
		}
		far = math.Min(far, t2)
	}
	if near > far || far < 0 {
		return c.sky(x, y, t), math.Inf(1)
	}
	face := axis * 2
	if comp(d, axis) > 0 {
		face++ // entering from the negative side
	}
	p := o.add(d.scale(near))
	cell, fu, fv := c.cellOf(face, p)
	cl := c.cells[cell]
	e := math.Min(math.Min(fu, 1-fu), math.Min(fv, 1-fv))
	// A cell gone is a hole in the hull, its frame left round it and
	// pulsing red; a cell growing back is a hole above the edge it has
	// grown to.
	if (cl.state == cellGone || cl.state == cellGrowing && fv > cl.k) && e > 0.07 {
		return c.inside(p, d, near, far, o, x, y, t), near
	}
	n := c.turn.apply(faceNormal(face))
	diffuse := math.Max(n.dot(light), 0)
	shade := 0.6 + 0.8*h01(uint64(cell), c.seed+9)
	col := gray(0.025 * shade).mul(v3(0.8, 0.9, 1)).scale(0.25 + 1.1*diffuse)
	// The seams round every cell, and each cell's own marks.
	seam := 0.8*math.Exp(-sq(e/0.035)) + 0.12*math.Exp(-sq(e/0.1))
	marks := 0.0
	switch mix64(uint64(cell)+c.seed) % 4 {
	case 0: // a square inside
		m := math.Max(math.Abs(fu-0.5), math.Abs(fv-0.5))
		marks = math.Exp(-sq((m - 0.2) / 0.03))
	case 1: // a bar across
		marks = math.Exp(-sq((fv - 0.5) / 0.03))
	case 2: // a corner to corner
		marks = math.Exp(-sq((fu - fv) / 0.04))
	}
	lit := c.seam.scale(0.9*seam + 0.3*marks)
	switch cl.state {
	case cellAimed:
		lit = aimedLight.scale((0.9*seam + 0.2*marks) * (0.6 + 0.4*math.Sin(t*30)))
	case cellHit:
		flash := mix(gray(2), boltLight.scale(1.4), sat(cl.k*2))
		col = flash.scale(1 - cl.k*0.6)
	case cellGone:
		lit = boltLight.scale((0.5 + 0.5*math.Sin(t*8)) * (0.4 + 1.2*math.Exp(-sq((e-0.035)/0.03))))
	case cellGrowing:
		// The edge the cell is grown to, bright, and what is grown, new.
		lit = lit.add(growLight.scale(1.6 * math.Exp(-sq((fv-cl.k)/0.04))))
		col = col.add(growLight.scale(0.15))
	case cellOK:
		lit = lit.add(growLight.scale(0.5 * cl.k * (seam + 0.4)))
		col = col.add(growLight.scale(0.12 * cl.k))
	}
	return col.add(lit), near
}

// comp is p's component along axis a: 0 x, 1 y, 2 z.
func comp(p vec3, a int) float64 {
	switch a {
	case 0:
		return p.x
	case 1:
		return p.y
	}
	return p.z
}

// cellOf is the cell of face f at the point p on it, and where in the cell
// p is, each 0 to 1.
func (c *cube) cellOf(f int, p vec3) (int, float64, float64) {
	var u, w float64
	switch f / 2 {
	case 0:
		u, w = p.z, p.y
	case 1:
		u, w = p.x, p.z
	default:
		u, w = p.x, p.y
	}
	n := float64(c.n)
	su, sw := (u+1)/2*n, (w+1)/2*n
	cu, cw := min(max(int(su), 0), c.n-1), min(max(int(sw), 0), c.n-1)
	return f*c.n*c.n + cw*c.n + cu, su - float64(cu), sw - float64(cw)
}

// inside is what a ray sees through a hole: the core, or the far wall lit
// by it, or, through a second hole, the sky.
func (c *cube) inside(p, d vec3, near, far float64, o vec3, x, y, t float64) vec3 {
	const core = 0.32
	pulse := 0.8 + 0.2*math.Sin(t*3.1)
	// The ray's nearest approach to the middle, for the core's glow.
	b := -p.dot(d)
	miss := p.add(d.scale(math.Max(b, 0))).length()
	glow := c.seam.scale(0.9 * pulse * math.Exp(-sq(miss/0.55)))
	if disc := b*b - (p.dot(p) - core*core); disc > 0 && b-math.Sqrt(disc) > 0 {
		q := p.add(d.scale(b - math.Sqrt(disc)))
		rim := 1 - math.Abs(q.norm().dot(d))
		return c.seam.scale(2.2 * pulse).add(gray(0.8 * rim))
	}
	exit := o.add(d.scale(far))
	axis, best := 0, 0.0
	for a := 0; a < 3; a++ {
		if v := math.Abs(comp(exit, a)); v > best {
			axis, best = a, v
		}
	}
	face := axis * 2
	if comp(exit, axis) < 0 {
		face++
	}
	cell, fu, fv := c.cellOf(face, exit)
	if cl := c.cells[cell]; cl.state == cellGone || cl.state == cellGrowing && fv > cl.k {
		return c.sky(x, y, t).add(glow)
	}
	e := math.Min(math.Min(fu, 1-fu), math.Min(fv, 1-fv))
	wall := c.seam.scale(0.08 * math.Exp(-sq(e/0.05)))
	return gray(0.015).add(wall).add(glow)
}

// sky is the space behind: a nebula, faint, and stars.
func (c *cube) sky(x, y, t float64) vec3 {
	x += t * 0.001
	n := fbm(x*1.2, y*1.2, 4, c.seed+50)
	dens := smooth(0, 0.9, n)
	return c.neb.at(n*0.5 + 0.2).scale(0.03 * dens).add(stars(x, y, t, c.seed))
}

// fighter draws one attacker at p going dir: a dart of dark metal lit by
// the light, its edges drawn in its engine's light, and its engine glowing
// behind it.
func (c *cube) fighter(v *view, p, dir, light vec3) {
	side := dir.cross(v3(0, 1, 0))
	if side.length() < 1e-6 {
		side = v3(1, 0, 0)
	}
	side = side.norm()
	up := side.cross(dir)
	const s = 0.19
	at := func(fwd, right, high float64) vec3 {
		return p.add(dir.scale(fwd * s)).add(side.scale(right * s)).add(up.scale(high * s))
	}
	nose, tail := at(1.2, 0, 0), at(-0.6, 0, 0.05)
	lw, rw, top, bot := at(-0.7, -1, -0.05), at(-0.7, 1, -0.05), at(-0.5, 0, 0.35), at(-0.5, 0, -0.2)
	// Each face drawn only when it looks toward the eye, so the dart's
	// faces need no order among themselves.
	for _, tri := range [][3]vec3{{nose, lw, top}, {nose, top, rw}, {nose, bot, lw}, {nose, rw, bot}, {lw, tail, top}, {rw, top, tail}, {lw, bot, tail}, {rw, tail, bot}} {
		mid := tri[0].add(tri[1]).add(tri[2]).scale(1.0 / 3)
		n := tri[1].sub(tri[0]).cross(tri[2].sub(tri[0])).norm()
		if n.dot(mid.sub(p)) < 0 {
			n = n.scale(-1)
		}
		if n.dot(c.eye.sub(mid)) <= 0 {
			continue
		}
		shade := 0.03 + 0.3*math.Max(n.dot(light), 0) + 0.08*math.Abs(n.dot(c.f))
		c.fill(v, tri, v3(0.5, 0.55, 0.65).scale(shade))
	}
	edge := 0.7*float64(v.ph)/600 + 0.5
	for _, e := range [][2]vec3{{nose, lw}, {nose, rw}, {lw, tail}, {rw, tail}, {nose, top}, {top, tail}} {
		c.streak(v, e[0], e[1], edge, engine.scale(0.45), true)
	}
	if gx, gy, depth, ok := c.project(v, tail.sub(dir.scale(0.05))); ok && c.seen(v, gx, gy, depth) {
		r := math.Max(1.2, 0.035/depth*1.05*float64(v.ph))
		v.glow(gx, gy, r*1.7, engine.scale(0.9))
		v.glow(gx, gy, r*0.8, gray(1.4))
	}
}

// fill fills a triangle of the world with col where nothing of the hull
// is in front of it. The depth kept is the hull's alone: what is drawn
// after (an engine's glow behind its own dart) is hidden by the hull only.
func (c *cube) fill(v *view, tri [3]vec3, col vec3) {
	var px, py, pd [3]float64
	for i, p := range tri {
		var ok bool
		if px[i], py[i], pd[i], ok = c.project(v, p); !ok {
			return
		}
	}
	x1 := max(int(math.Floor(min(px[0], px[1], px[2]))), v.x0)
	x2 := min(int(math.Ceil(max(px[0], px[1], px[2]))), v.x0+v.w-1)
	y1 := max(int(math.Floor(min(py[0], py[1], py[2]))), v.y0)
	y2 := min(int(math.Ceil(max(py[0], py[1], py[2]))), v.y0+v.h-1)
	area := (px[1]-px[0])*(py[2]-py[0]) - (px[2]-px[0])*(py[1]-py[0])
	if x1 > x2 || y1 > y2 || math.Abs(area) < 1e-9 {
		return
	}
	for y := y1; y <= y2; y++ {
		fy := float64(y) + 0.5
		for x := x1; x <= x2; x++ {
			fx := float64(x) + 0.5
			w0 := ((px[1]-fx)*(py[2]-fy) - (px[2]-fx)*(py[1]-fy)) / area
			w1 := ((px[2]-fx)*(py[0]-fy) - (px[0]-fx)*(py[2]-fy)) / area
			w2 := 1 - w0 - w1
			if w0 < 0 || w1 < 0 || w2 < 0 {
				continue
			}
			k := (y-v.y0)*v.w + (x - v.x0)
			if depth := w0*pd[0] + w1*pd[1] + w2*pd[2]; depth < c.z[k] {
				v.px[k] = col
			}
		}
	}
}

// count writes the cube's nodes whole of all, and how many have grown
// back since the part began, in the picture's top left corner.
func (c *cube) count(v *view, grown int) {
	whole := 0
	for _, cl := range c.cells {
		if cl.state == cellOK || cl.state == cellAimed {
			whole++
		}
	}
	lines := []string{"NODES " + strconv.Itoa(whole) + "/" + strconv.Itoa(len(c.cells)), "RECLONED " + strconv.Itoa(grown)}
	s := c.text
	gx, gy := v.pw/40, v.ph/24
	for i, line := range lines {
		col := c.seam.scale(0.9)
		if i == 0 && whole < len(c.cells) {
			col = mix(c.seam, boltLight, 0.5).scale(0.9)
		}
		for k := 0; k < len(line); k++ {
			if line[k] == ' ' {
				continue
			}
			g := glyphFor(line[k])
			v.stamp(&g, gx+k*6*s, gy+i*9*s, s, col)
		}
	}
}
