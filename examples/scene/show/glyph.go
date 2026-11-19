package show

import "math"

// glyph is a character of the wall's font, five cells by seven, a row a
// string, '#' lit.
type glyph [7]string

var digits = [2]glyph{
	{".###.", "#...#", "#..##", "#.#.#", "##..#", "#...#", ".###."},
	{"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", ".###."},
}

// word is the wordmark the show opens on, vmx's name, in capitals across
// the wall; tagline is under it, the site, in lower case as the brand
// writes it.
const (
	word    = "VMX"
	tagline = "zettarack.com"
)

var letters = map[byte]glyph{
	'z': {".....", ".....", "#####", "...#.", "..#..", ".#...", "#####"},
	'e': {".....", ".....", ".###.", "#...#", "#####", "#....", ".####"},
	't': {".#...", ".#...", "####.", ".#...", ".#...", ".#..#", "..##."},
	'a': {".....", ".....", ".###.", "....#", ".####", "#...#", ".####"},
	'r': {".....", ".....", "#.##.", "##..#", "#....", "#....", "#...."},
	'c': {".....", ".....", ".###.", "#....", "#....", "#...#", ".###."},
	'k': {"#....", "#....", "#..#.", "#.#..", "##...", "#.#..", "#..#."},
	'o': {".....", ".....", ".###.", "#...#", "#...#", "#...#", ".###."},
	'm': {".....", ".....", "##.#.", "#.#.#", "#.#.#", "#.#.#", "#.#.#"},
}

// blank is a character the font has no glyph for: nothing drawn.
var blank = glyph{".....", ".....", ".....", ".....", ".....", ".....", "....."}

// glyphFor is c's glyph: a lower case letter's own, else the capitals'
// font's, else blank.
func glyphFor(c byte) glyph {
	if g, ok := letters[c]; ok {
		return g
	}
	if g, ok := font[c]; ok {
		return g
	}
	return blank
}

// on says whether cell (c, r) of g is lit.
func (g *glyph) on(c, r int) bool { return r >= 0 && r < 7 && c >= 0 && c < 5 && g[r][c] == '#' }

// font is the scroller's: capitals, digits and a few marks; '*' is the
// diamond between two lines.
var font = map[byte]glyph{
	'A':  {".###.", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'B':  {"####.", "#...#", "#...#", "####.", "#...#", "#...#", "####."},
	'C':  {".###.", "#...#", "#....", "#....", "#....", "#...#", ".###."},
	'D':  {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
	'E':  {"#####", "#....", "#....", "####.", "#....", "#....", "#####"},
	'F':  {"#####", "#....", "#....", "####.", "#....", "#....", "#...."},
	'G':  {".###.", "#...#", "#....", "#.###", "#...#", "#...#", ".####"},
	'H':  {"#...#", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'I':  {".###.", "..#..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'J':  {"..###", "...#.", "...#.", "...#.", "...#.", "#..#.", ".##.."},
	'K':  {"#...#", "#..#.", "#.#..", "##...", "#.#..", "#..#.", "#...#"},
	'L':  {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	'M':  {"#...#", "##.##", "#.#.#", "#.#.#", "#...#", "#...#", "#...#"},
	'N':  {"#...#", "#...#", "##..#", "#.#.#", "#..##", "#...#", "#...#"},
	'O':  {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'P':  {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'Q':  {".###.", "#...#", "#...#", "#...#", "#.#.#", "#..#.", ".##.#"},
	'R':  {"####.", "#...#", "#...#", "####.", "#.#..", "#..#.", "#...#"},
	'S':  {".####", "#....", "#....", ".###.", "....#", "....#", "####."},
	'T':  {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'U':  {"#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'V':  {"#...#", "#...#", "#...#", "#...#", "#...#", ".#.#.", "..#.."},
	'W':  {"#...#", "#...#", "#...#", "#.#.#", "#.#.#", "#.#.#", ".#.#."},
	'X':  {"#...#", "#...#", ".#.#.", "..#..", ".#.#.", "#...#", "#...#"},
	'Y':  {"#...#", "#...#", ".#.#.", "..#..", "..#..", "..#..", "..#.."},
	'Z':  {"#####", "....#", "...#.", "..#..", ".#...", "#....", "#####"},
	'0':  digits[0],
	'1':  digits[1],
	'2':  {".###.", "#...#", "....#", "...#.", "..#..", ".#...", "#####"},
	'3':  {"#####", "...#.", "..#..", "...#.", "....#", "#...#", ".###."},
	'4':  {"...#.", "..##.", ".#.#.", "#..#.", "#####", "...#.", "...#."},
	'5':  {"#####", "#....", "####.", "....#", "....#", "#...#", ".###."},
	'6':  {"..##.", ".#...", "#....", "####.", "#...#", "#...#", ".###."},
	'7':  {"#####", "....#", "...#.", "..#..", ".#...", ".#...", ".#..."},
	'8':  {".###.", "#...#", "#...#", ".###.", "#...#", "#...#", ".###."},
	'9':  {".###.", "#...#", "#...#", ".####", "....#", "...#.", ".##.."},
	'.':  {".....", ".....", ".....", ".....", ".....", ".##..", ".##.."},
	',':  {".....", ".....", ".....", ".....", ".##..", "..#..", ".#..."},
	':':  {".....", ".##..", ".##..", ".....", ".##..", ".##..", "....."},
	'?':  {".###.", "#...#", "....#", "...#.", "..#..", ".....", "..#.."},
	'!':  {"..#..", "..#..", "..#..", "..#..", "..#..", ".....", "..#.."},
	'-':  {".....", ".....", ".....", "#####", ".....", ".....", "....."},
	'/':  {"....#", "....#", "...#.", "..#..", ".#...", "#....", "#...."},
	'\'': {"..#..", "..#..", ".#...", ".....", ".....", ".....", "....."},
	'*':  {".....", "..#..", ".###.", "#####", ".###.", "..#..", "....."},
}

// facts are what the scrollers say, each true of vmx as it is: the
// examples' own documents say each one.
var facts = []string{
	"NO CONTAINER ENGINE RUNS ON THIS HOST",
	"EVERY CONTAINER HERE IS A MICROVM OF ITS OWN",
	"Q: WHERE IS THE DOCKER DAEMON?  A: THERE IS NONE",
	"Q: WHICH STORAGE PLUGIN?  A: NONE. A VOLUME IS ZFS",
	"VMX SERVES NO WEB PAGE: THIS ONE IS A CONTAINER'S",
	"CLONES IN AN INSTANT, NO SPACE UNTIL THEY WRITE",
	"EVERY CHANGE IS A JOB: TAKEN BACK IF A STEP FAILS",
	"THE HOST ROLLS BACK TOO",
	"NOTHING UNSIGNED BOOTS",
	"NO DEFAULT PASSWORD",
	"THE WHOLE SCREEN: ONE 22 MB PROGRAM",
	"FORTY MICROVMS, ONE PICTURE",
	"FREE OF CHARGE",
	"ZETTARACK.COM",
}

// scroll is a line of facts in a seed's order, the site at its end.
func scroll(seed uint64) string {
	o := make([]int, len(facts)-1)
	for i := range o {
		o[i] = i
	}
	s := mix64(seed + 0xfac7)
	for i := len(o) - 1; i > 0; i-- {
		s = mix64(s + 1)
		j := int(s % uint64(i+1))
		o[i], o[j] = o[j], o[i]
	}
	line := ""
	for _, i := range o[:6] {
		line += facts[i] + "   *   "
	}
	return line + facts[len(facts)-1] + "   *   "
}

// scroller draws text going right to left across the picture at height
// gy, s pixels a font cell, speed pixels a second, riding a gentle wave,
// faded at the picture's two ends, from t0 on.
func (v *view) scroller(text string, gy, s int, speed, t0, t float64, c vec3) {
	if t < t0 {
		return
	}
	adv := 6 * s
	x0 := float64(v.pw) - (t-t0)*speed
	first := max(int((float64(v.x0)-x0)/float64(adv))-1, 0)
	last := min(int((float64(v.x0+v.w)-x0)/float64(adv))+1, len(text)-1)
	for i := first; i <= last; i++ {
		g, ok := font[text[i]]
		if !ok {
			continue
		}
		x := x0 + float64(i*adv)
		if x < -float64(adv) || x > float64(v.pw) {
			continue
		}
		edge := smooth(0, 0.12, x/float64(v.pw)) * smooth(1, 0.88, x/float64(v.pw))
		y := float64(gy) + float64(2*s)*math.Sin(x/float64(v.pw)*2*math.Pi*1.5+t*1.6)
		v.stamp(&g, int(x), int(y), s, c.scale(edge))
	}
}

// features are what the vmx intro's scroller says, in order: what vmx is
// and does, the site at the end. Each is true of vmx as it ships, and each
// is said where vmx's own pages say it (examples/FEATURES.md, the row
// beside each; kbx's, zettarack.com/kbx): a line here that no page says is
// a line to take out.
var features = []string{
	"VMX: THE HYPERVISOR FROM ZETTARACK",
	"KVM MACHINES, FIRECRACKER MICROVMS AND CONTAINERS, ALL ON ZFS",     // Machine, microVM, Container; Storage
	"ABSOLUTELY EVERYTHING IS A DATASET",                                // Storage
	"MADE FOR SMALL CONTAINERS: EACH ONE A MICROVM WITH ITS OWN KERNEL", // Container
	"KVM MACHINES WITH UEFI AND SECURE BOOT",                            // Machine
	"LINUX, WINDOWS, BSD: ANYTHING THAT BOOTS ON A PC",                  // Operating systems
	"A MICROVM RUNS UNDER THE JAILER AS ITS OWN USER",                   // microVM
	"DOCKER AND OCI IMAGES RUN AS MICROVMS: NO CONTAINER ENGINE",        // Container
	"Q: WHICH STORAGE PLUGIN?  A: NONE. A VOLUME IS ZFS",                // Storage
	"CLONES IN AN INSTANT, NO SPACE UNTIL THEY WRITE",                   // Storage
	"SNAPSHOTS, ROLLBACK, SCHEDULED SNAPSHOTS, BACKUP AND RESTORE",      // a machine's rows
	"A CONTAINER'S DISK AND EVERY VOLUME, SNAPSHOT AT ONE MOMENT",       // Snapshots and rollback
	"BLUE/GREEN AT EVERY LEVEL: CONTAINERS, MACHINES, THE HOST",         // Snapshots and rollback; Clone it; The host
	"WINDOWS 11 AND SERVER 2025 INSTALLED WITH NOTHING ASKED",           // Windows
	"DESKTOPS: KDE, GNOME, XFCE, I3, SWAY, HYPRLAND, BSPWM, AWESOME",    // Desktop
	"APPLIANCES FROM RECIPES, OR YOUR OWN GOLDEN, SEALED",               // Appliance, Your own golden
	"PCI AND USB PASSTHROUGH",                                           // PCI and USB devices
	"NAT, BRIDGES, VLANS AND PRIVATE NETWORKS",                          // Networks
	"KUBERNETES? YES. KBX, IN ALPHA, BUILDS ITS NODES ON VMX",           // zettarack.com/kbx
	"EVERY CHANGE IS A JOB: TAKEN BACK IF A STEP FAILS",                 // Jobs
	"DRY RUN SHOWS THE PLAN AND CHANGES NOTHING",                        // Jobs
	"THE HOST ROLLS BACK TOO, BY ITSELF IF AN UPGRADE FAILS",            // The host
	"NOTHING UNSIGNED BOOTS",                                            // Secure Boot
	"NO DEFAULT PASSWORD",
	"THE ROOT POOL CAN BE UNLOCKED BY THE TPM", // Keys
	"THE INSTALLER NEEDS NO NETWORK",           // Offline
	"THE WHOLE SCREEN: ONE 22 MB PROGRAM",
	"FORTY MICROVMS, ONE PICTURE",
	"FREE OF CHARGE",
	"ZETTARACK.COM",
}

// introLine is the intro scroller's line: the features, a diamond between.
func introLine() string {
	line := ""
	for i, f := range features {
		if i > 0 {
			line += "   *   "
		}
		line += f
	}
	return line
}
