// Command scene-wall is the demoscene wall: it hands every tile the
// frame's time, keeps each tile's newest picture, and shows them all as
// one picture in a browser, with how many tiles are live, how many frames
// a second they draw together, and how long the whole wall took to come
// up.
//
// On the page a click chooses a tile or lets it go, and a drag chooses
// every tile it passes. The buttons under the wall do their verb to every
// tile chosen: snapshot its disk and its volume; restore both to its
// newest snapshot, or with one tile chosen to any of its snapshots (the
// microVM stopped and booted again with them, so both counts of frames the
// tile keeps go back); rebuild it (a new microVM from the image in its
// place on the same volume, or in a place left empty); update it (the
// same, from the wall's other image: a new version of the tile's program,
// blue to green and back); kill it (its microVM ended outright, as a crash
// would end it, and started again by the keeper); clone it as many times
// as the number beside the button says (each clone a microVM of its own,
// named for a place after the picture's own, so the wall grows); or remove
// it (its place stays empty). Reset the wall takes every tile and clone
// away and runs the picture's own tiles again, and the clock says how long
// that took from the press. A line under the clock says what the host
// holds of the wall, as the keeper reads it from the host: the microVMs
// running, the memory they take, their disks as full copies against what
// ZFS keeps, each tile's own share shown on it, and their volumes. The
// page's info says what every mark on a tile means.
//
// The wall does none of that itself: it is a program in a container, and
// nothing in a container commands the host. It keeps each request for the
// keeper, a loop on the host (05-demoscene-wall.sh, run as the service
// demoscene-keeper), which asks for them (GET /asked), does each with the
// vmx command and says what became of it (POST /did/N), and the page shows
// that on the tile. The wall knows a new microVM by the boot id every tile
// sends with its frames, so the page can say how long a rebuilt tile was
// away. With no keeper watching, a request waits, and the page says so.
// Part of the vmx examples (MIT, see examples/LICENSE).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type tile struct {
	png  []byte
	ver  int
	at   time.Time
	host string
	ms   int
	// boot is the id its microVM sends with every frame, made when the
	// tile's program starts: another id is another boot.
	boot string
	// gen is how many microVMs have had the place. newAt is when the last
	// one that took it over from another drew first, and took how long the
	// place was without a picture then: from the press that asked for it,
	// or with none from the old one's last frame.
	gen   int
	newAt time.Time
	took  time.Duration
	// asked is when a rebuild or an update of the place was asked (zero
	// when none waits). Until restartBy, a new boot is the same machine started
	// again by a restore, not a new one.
	asked, restartBy time.Time
	// frames is the count of frames the tile has drawn, as its disk keeps
	// it, and vframes as its volume keeps it: -1 when it sent none (a tile
	// with no volume, or an agent from before volumes).
	frames, vframes int64
	// look is the colours its program said it drew in (blue, green),
	// "" when it said none.
	look string
	// snaps is its snapshots as the keeper last listed them, at snapsAt
	// (zero when it has not); wanted is when a page last asked for them.
	snaps           []snapRow
	snapsAt, wanted time.Time
	// queue is the buttons' verbs waiting for the keeper, the first the
	// one it does next; acting whether it took that one, actAt when. said
	// is what the keeper said became of the last, saidAt when.
	queue         []press
	acting        bool
	actAt, saidAt time.Time
	said          string
	// own is the bytes of disk the tile holds of its own, as ZFS keeps it:
	// what it wrote since it was cloned (POST /host).
	own int64
	// up is how long its microVM had run when the tile's program started,
	// in ms, as the machine's own kernel counts it (X-Up-Ms, 0 when none
	// was said), from its boot's first frame; upAt is when that came.
	up   int64
	upAt time.Time
}

// press is one press of a button for a tile: its verb, and for a clone
// how many.
type press struct {
	do string
	n  int
	// at is the snapshot a restore goes back to, "" for the newest.
	at string
}

// snapRow is one of a tile's snapshots as vmx lists them (vmx container
// snapshots NAME --json), which the keeper hands on as it is.
type snapRow struct {
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
	Used    int64     `json:"used_bytes"`
}

// snapsMax is the most snapshots of one tile the wall keeps a list of,
// and wantFor how long a page's ask for a tile's list stands.
const (
	snapsMax = 256
	wantFor  = 5 * time.Second
)

// restorable says whether l is a snapshot a restore may go back to: a name
// by vmx's rule (a letter or a digit, then letters, digits, . _ and -, at
// most 64) and none of a clone's own (clone-), which the clone stands on.
func restorable(l string) bool {
	if l == "" || len(l) > 64 || l == "golden" || strings.HasPrefix(l, "clone-") {
		return false
	}
	for k, c := range l {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case k > 0 && (c == '.' || c == '_' || c == '-'):
		default:
			return false
		}
	}
	return true
}

// hostNow is what the keeper last said of the host (POST /host): how many
// of the wall's microVMs run, the memory they take (each one's resident
// set), and their disks: as many bytes as full copies would take, and as
// ZFS keeps them, each tile's own and the images they share counted once.
// Volumes is what the volumes the tiles count on hold, as ZFS keeps them
// (each one's own, the room ZFS sets aside for it with it), apart from
// their disks. Own is each place's own bytes, by its number.
type hostNow struct {
	Tiles   int              `json:"tiles"`
	Memory  int64            `json:"memory"`
	Copies  int64            `json:"copies"`
	Disk    int64            `json:"disk"`
	Volumes int64            `json:"volumes"`
	Own     map[string]int64 `json:"own"`
}

type wall struct {
	mu               sync.Mutex
	cols, rows, w, h int
	tiles            []tile
	epoch, start     time.Time
	frames           []time.Time
	// full is how long the wall took to have every place drawing, from
	// start, and fullN how many places that was.
	full  time.Duration
	fullN int
	// replaced counts the microVMs that took over a place from another;
	// watched is when the keeper last asked what to do.
	replaced int
	watched  time.Time
	// The wall made again: phase is "" (none), "asked", "down" (the keeper
	// takes every tile away) or "up" (the picture's own tiles run again,
	// until every place draws), since phaseAt; pressed is when it was
	// asked, and era counts the times the places started again, so the
	// page knows their pictures are new.
	phase            string
	phaseAt, pressed time.Time
	era              int
	// host is what the keeper last said of the host, hostAt when; nil
	// until it has.
	host   *hostNow
	hostAt time.Time
	// viewed is when a page (or the launch script) last asked how the
	// wall is; a tile that asks for a frame when nobody has for viewFor
	// waits on wake for one to, holdFor at most (idle, below).
	viewed           time.Time
	wake             chan struct{}
	viewFor, holdFor time.Duration
}

const (
	liveFor = 2 * time.Second
	// A keeper that has not asked for this long is not watching.
	keeperFor = 3 * time.Second
	// A verb the keeper took and did not say it finished in this long is
	// handed out again: the keeper stopped half way.
	actFor = 60 * time.Second
	// What the keeper said is shown this long.
	sayFor = 8 * time.Second
	// A tile holds this many verbs waiting at most.
	queueMax = 4
	// A reset that has not moved on in this long has failed (the keeper
	// stopped half way): the wall takes requests again.
	resetFor = 3 * time.Minute
	// restartIn is how soon after a restore is said done its machine draws
	// again; a new boot later than that is a new machine.
	restartIn = 10 * time.Second
	// maxTiles is how many places the wall takes, its own and the rows a
	// clone's place adds below them: a clone's count is one to this. The
	// host's own limits come first on most (its memory, its network's
	// addresses), and the keeper says which.
	maxTiles = 1024
	// hostFor is how long what the keeper said of the host is shown.
	hostFor = 10 * time.Second
	// A page asks how the wall is ten times a second: one that has not
	// for idleAfter is not open. A tile's ask for a frame is held idleHold
	// at most while nobody watches, within the agent's own five seconds.
	idleAfter = 3 * time.Second
	idleHold  = 4 * time.Second
)

// verbs are the buttons under the wall, as the keeper knows them.
var verbs = map[string]bool{"snapshot": true, "rollback": true, "rebuild": true, "update": true, "kill": true, "clone": true, "remove": true}

// remakes says whether verb do puts a new machine in the place: a rebuild
// from the image the tile runs, or an update to the wall's other image.
func remakes(do string) bool { return do == "rebuild" || do == "update" }

func newWall(cols, rows, w, h int, now time.Time) *wall {
	// Seen as watched from its start, so the launch's first look at it
	// (a frame, within two seconds) is answered at once.
	return &wall{cols: cols, rows: rows, w: w, h: h, tiles: make([]tile, cols*rows), epoch: now, start: now,
		viewed: now, viewFor: idleAfter, holdFor: idleHold}
}

// clip cuts s to at most n bytes, at the start of a character.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// place is the tile a request's {i} names. With grow, a place after the
// wall's own up to maxTiles is a clone's, and the wall grows to it.
// s.mu is held.
func (s *wall) place(r *http.Request, grow bool) (*tile, int, bool) {
	i, err := strconv.Atoi(r.PathValue("i"))
	if err != nil || i < 0 || i >= maxTiles || !grow && i >= len(s.tiles) {
		return nil, i, false
	}
	for len(s.tiles) <= i {
		s.tiles = append(s.tiles, tile{})
	}
	return &s.tiles[i], i, true
}

// expire ends a reset that has not moved on in resetFor. s.mu is held.
func (s *wall) expire(now time.Time) {
	if s.phase != "" && now.Sub(s.phaseAt) > resetFor {
		log.Printf("reset: %s for %.0f s, and no further: the wall takes requests again", s.phase, now.Sub(s.phaseAt).Seconds())
		s.phase = ""
	}
}

// seen is a page (or the launch script) asking how the wall is: every
// tile waiting for one is let go. s.mu is held.
func (s *wall) seen(now time.Time) {
	s.viewed = now
	if s.wake != nil {
		close(s.wake)
		s.wake = nil
	}
}

// frame is the frame's time and the picture's shape, for a tile to draw
// its part of. While nobody watches, a tile's ask waits until a page asks
// how the wall is, holdFor at most, and is then answered 204 (nothing to
// draw: ask again): forty tiles drew eight frames a second each for as
// long as they ran, page or no page, and kept the host's processors busy
// and warm (a vmx host of 24 threads, 2026-11-17: the wall was taken down
// for its heat). A
// page opened lets every waiting tile go at once.
func (s *wall) frame(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	var wake chan struct{}
	if time.Since(s.viewed) > s.viewFor {
		if s.wake == nil {
			s.wake = make(chan struct{})
		}
		wake = s.wake
	}
	s.mu.Unlock()
	if wake != nil {
		t := time.NewTimer(s.holdFor)
		defer t.Stop()
		select {
		case <-wake:
		case <-t.C:
			w.WriteHeader(http.StatusNoContent)
			return
		case <-r.Context().Done():
			// The tile gave up waiting: nobody is told anything.
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"t": time.Since(s.epoch).Seconds(), "cols": s.cols, "rows": s.rows, "w": s.w, "h": s.h,
	}); err != nil {
		log.Printf("frame: %v", err)
	}
}

func (s *wall) put(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil || !bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")) {
		http.Error(w, "a tile is one PNG of at most 4 MiB", http.StatusBadRequest)
		return
	}
	ms, _ := strconv.Atoi(r.Header.Get("X-Render-Ms"))              // a missing number is shown as none
	frames, _ := strconv.ParseInt(r.Header.Get("X-Frames"), 10, 64) // a missing count is shown as none
	// The volume's count: none (-1) when the tile sent none, or sent what
	// is not a count.
	vframes, err := strconv.ParseInt(r.Header.Get("X-Volume-Frames"), 10, 64)
	if err != nil || vframes < 0 {
		vframes = -1
	}
	host, boot := clip(r.Header.Get("X-Host"), 40), clip(r.Header.Get("X-Boot"), 40)
	// The look it drew in, a word of at most 16 letters; anything else is
	// shown as none.
	look := r.Header.Get("X-Look")
	if len(look) > 16 || strings.Trim(look, "abcdefghijklmnopqrstuvwxyz") != "" {
		look = ""
	}
	// Ten minutes at most: a number past that, or none, is shown as none.
	up, err := strconv.ParseInt(r.Header.Get("X-Up-Ms"), 10, 64)
	if err != nil || up < 0 || up > 600000 {
		up = 0
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	// A place after the picture's own is a clone's: the wall grows to it.
	t, i, ok := s.place(r, true)
	if !ok {
		http.Error(w, "no such tile", http.StatusNotFound)
		return
	}
	switch {
	case t.ver == 0:
		log.Printf("tile %d is up (%s, %s)", i, host, r.RemoteAddr)
		t.gen = 1
	case boot != "" && t.boot != "" && boot != t.boot && now.Before(t.restartBy):
		// A restore stopped it and started it again: the same machine.
		t.restartBy = time.Time{}
		log.Printf("tile %d started again, %.1f s after its last frame", i, now.Sub(t.at).Seconds())
	case boot != "" && t.boot != "" && boot != t.boot:
		// Another microVM has the place. How long the wall went without
		// it: since the press that asked, or since the old one's last
		// frame when nobody asked here (it went some other way).
		from := t.at
		if !t.asked.IsZero() {
			from = t.asked
		}
		t.took, t.gen, t.newAt, t.asked = now.Sub(from), t.gen+1, now, time.Time{}
		s.replaced++
		log.Printf("tile %d is a new microVM, %.1f s after the old one (%s)", i, t.took.Seconds(), host)
	}
	// A boot's first frame says how long its machine took; the rest say
	// the same, and are not taken for a new boot's.
	if t.ver == 0 || boot != t.boot {
		t.up, t.upAt = up, now
	}
	t.boot = boot
	t.png, t.ver, t.at, t.host, t.ms, t.frames, t.vframes, t.look = body, t.ver+1, now, host, ms, frames, vframes, look
	s.frames = append(s.frames, now)
	for len(s.frames) > 0 && now.Sub(s.frames[0]) > liveFor {
		s.frames = s.frames[1:]
	}
	if s.full == 0 {
		all := true
		for _, o := range s.tiles {
			all = all && o.ver > 0 && !o.at.Before(s.start)
		}
		if all {
			s.full, s.fullN = now.Sub(s.start), len(s.tiles)
			if s.phase == "up" {
				s.phase = ""
			}
			log.Printf("all %d tiles up, %.1f s after the start", len(s.tiles), s.full.Seconds())
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *wall) get(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	var body []byte
	t, i, ok := s.place(r, false)
	if ok {
		body = t.png
	}
	s.mu.Unlock()
	if body == nil {
		http.Error(w, "no picture yet", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(body); err != nil {
		log.Printf("tile %d: %v", i, err)
	}
}

// do is a button under the wall pressed: its verb for each tile chosen
// (the body, their numbers as a JSON list), kept for the keeper in the
// order asked, at most queueMax waiting on a tile. A clone says how many
// (?n=, 1 to maxTiles; 3 when it does not say); no other verb takes a
// count. A verb asked again straight after itself, a clone with the same
// count, is the same press twice, and waits once. Every number is checked
// before any is kept; while the wall is made again, from the press of its
// reset until every place draws, nothing is.
func (s *wall) do(w http.ResponseWriter, r *http.Request) {
	what := r.PathValue("what")
	if !verbs[what] {
		http.Error(w, "no such verb", http.StatusNotFound)
		return
	}
	n := 0
	if what == "clone" {
		n = 3
	}
	if v, given := r.URL.Query()["n"]; given {
		k, err := strconv.Atoi(v[0])
		switch {
		case what != "clone":
			http.Error(w, "only a clone takes a count", http.StatusBadRequest)
			return
		case len(v) != 1 || err != nil || k < 1 || k > maxTiles:
			http.Error(w, "n is how many clones: one number, 1 to "+strconv.Itoa(maxTiles), http.StatusBadRequest)
			return
		}
		n = k
	}
	at := ""
	if v, given := r.URL.Query()["at"]; given {
		switch {
		case what != "rollback":
			http.Error(w, "only a restore goes back to a snapshot named", http.StatusBadRequest)
			return
		case len(v) != 1 || !restorable(v[0]):
			http.Error(w, "at is one snapshot's name: a letter or a digit, then letters, digits, . _ and -, at most 64, and not a clone's", http.StatusBadRequest)
			return
		}
		at = v[0]
	}
	var which []int
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&which); err != nil || len(which) == 0 || len(which) > maxTiles {
		http.Error(w, "the body is the chosen tiles' numbers: a JSON list of 1 to "+strconv.Itoa(maxTiles), http.StatusBadRequest)
		return
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
	if s.phase != "" {
		http.Error(w, "the wall is being made again", http.StatusConflict)
		return
	}
	for _, i := range which {
		if i < 0 || i >= len(s.tiles) {
			http.Error(w, "no such tile: "+strconv.Itoa(i), http.StatusNotFound)
			return
		}
	}
	kept, over := 0, 0
	for _, i := range which {
		t := &s.tiles[i]
		switch q := t.queue; {
		case len(q) > 0 && q[len(q)-1] == press{what, n, at}:
		case len(q) >= queueMax:
			over++
		default:
			if len(q) == 0 && remakes(what) {
				t.asked = now
			}
			t.queue = append(t.queue, press{what, n, at})
			kept++
		}
	}
	log.Printf("%s asked for %d tiles, %d of them waiting already, %d with %d verbs waiting", what, len(which), len(which)-kept-over, over, queueMax)
	w.WriteHeader(http.StatusNoContent)
}

// asked is the keeper's question: for each tile, the verb it does next
// unless it has taken it (a clone with its count), and whether a reset is
// asked. Asking it is how the wall knows a keeper is watching.
func (s *wall) asked(w http.ResponseWriter, _ *http.Request) {
	type act struct {
		Tile int    `json:"tile"`
		Do   string `json:"do"`
		N    int    `json:"n,omitempty"`
		At   string `json:"at,omitempty"`
	}
	now := time.Now()
	s.mu.Lock()
	s.watched = now
	s.expire(now)
	acts, snaps := []act{}, []int{}
	for i := range s.tiles {
		t := &s.tiles[i]
		if !t.wanted.IsZero() && now.Sub(t.wanted) <= wantFor {
			snaps = append(snaps, i)
		}
		if len(t.queue) == 0 {
			continue
		}
		if t.acting && now.Sub(t.actAt) > actFor {
			t.acting = false
			log.Printf("tile %d: %s asked again, %.0f s after the keeper took it and said nothing", i, t.queue[0].do, now.Sub(t.actAt).Seconds())
		}
		if !t.acting {
			acts = append(acts, act{Tile: i, Do: t.queue[0].do, N: t.queue[0].n, At: t.queue[0].at})
		}
	}
	reset := s.phase == "asked"
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	// Whether somebody watches: the keeper looks for tiles that went, and
	// says what the host holds, less often while nobody does.
	watched := time.Since(s.viewed) <= s.viewFor
	if err := json.NewEncoder(w).Encode(map[string]any{"acts": acts, "reset": reset, "watched": watched, "snaps": snaps}); err != nil {
		log.Printf("asked: %v", err)
	}
}

// acting is the keeper saying it has taken a tile's next verb: it is not
// handed out again while it works.
func (s *wall) acting(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	t, _, ok := s.place(r, false)
	if !ok {
		http.Error(w, "no such tile", http.StatusNotFound)
		return
	}
	if len(t.queue) > 0 {
		t.acting, t.actAt = true, now
		switch t.queue[0].do {
		case "rollback":
			t.restartBy = now.Add(actFor)
		case "rebuild", "update":
			if t.asked.IsZero() {
				t.asked = now
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// did is the keeper saying what became of a tile's verb: the body is a
// few words the page shows on the tile for sayFor (or none), and X-Failed:
// 1 says it was not done. A place after the picture's own may be told
// before it draws (a clone's), and the wall grows to it.
func (s *wall) did(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 512))
	if err != nil {
		http.Error(w, "what was said did not read", http.StatusBadRequest)
		return
	}
	said := clip(strings.TrimSpace(string(body)), 120)
	failed := r.Header.Get("X-Failed") == "1"
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	t, i, ok := s.place(r, true)
	if !ok {
		http.Error(w, "no such tile", http.StatusNotFound)
		return
	}
	if t.acting && len(t.queue) > 0 {
		what := t.queue[0].do
		t.queue, t.acting = t.queue[1:], false
		switch {
		case what == "rollback" && failed:
			t.restartBy = time.Time{}
		case what == "rollback" && !t.restartBy.IsZero():
			// Not drawing again yet: it will be, within moments.
			t.restartBy = now.Add(restartIn)
		case remakes(what) && failed:
			t.asked = time.Time{}
		}
		if len(t.queue) > 0 && remakes(t.queue[0].do) && t.asked.IsZero() {
			t.asked = now
		}
	}
	if said != "" {
		t.said, t.saidAt = said, now
		log.Printf("tile %d: %s", i, said)
	}
	w.WriteHeader(http.StatusNoContent)
}

// reset is the page asking for the wall to be made again: every tile and
// clone taken away, and the picture's own run again. The keeper takes it
// (down), takes every tile away and says so (up); the wall's places start
// again then, and its clock runs from the press.
func (s *wall) reset(w http.ResponseWriter, _ *http.Request) {
	now := time.Now()
	s.mu.Lock()
	s.expire(now)
	if s.phase == "" {
		s.phase, s.phaseAt, s.pressed = "asked", now, now
		log.Printf("reset: asked")
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// down is the keeper saying it takes the wall down for a reset. What
// waited on the tiles is moot: they all go.
func (s *wall) down(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase != "asked" {
		http.Error(w, "no reset is asked", http.StatusConflict)
		return
	}
	s.phase, s.phaseAt = "down", time.Now()
	for i := range s.tiles {
		t := &s.tiles[i]
		t.queue, t.acting, t.asked, t.said = nil, false, time.Time{}, ""
	}
	log.Printf("reset: the keeper takes the wall down")
	w.WriteHeader(http.StatusNoContent)
}

// up is the keeper saying every tile is down: the places start again,
// the picture's own, and the clock from the press.
func (s *wall) up(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase != "down" {
		http.Error(w, "the wall is not being taken down", http.StatusConflict)
		return
	}
	now := time.Now()
	s.phase, s.phaseAt = "up", now
	s.tiles = make([]tile, s.cols*s.rows)
	s.start, s.full, s.fullN, s.frames, s.replaced = s.pressed, 0, 0, nil, 0
	s.era++
	log.Printf("reset: every tile down %.1f s after the press; the places start again", now.Sub(s.pressed).Seconds())
	w.WriteHeader(http.StatusNoContent)
}

func (s *wall) state(w http.ResponseWriter, _ *http.Request) {
	type one struct {
		V    int    `json:"v"`
		Age  int64  `json:"age_ms"`
		Host string `json:"host"`
		Ms   int    `json:"ms"`
		// Asked: a rebuild was asked and the new microVM has not drawn
		// yet. Gen: how many microVMs have had the place. New: how long
		// ago the last that took it over from another drew first (0 when
		// none has), and Took how long the place was without a picture
		// then.
		Asked bool  `json:"asked"`
		Gen   int   `json:"gen"`
		New   int64 `json:"new_ms"`
		Took  int64 `json:"took_ms"`
		// Frames: the count its disk keeps, and Vol its volume's (none for
		// a tile that sent none). Act: the verb the keeper does
		// next or is doing, a clone's count in ActN, and More how many wait
		// behind it. Said: what the keeper said became of the last, for
		// sayFor. Own: the disk it holds of its own, while what the keeper
		// said of the host is shown.
		Frames int64  `json:"frames"`
		Vol    *int64 `json:"vol_frames,omitempty"`
		Look   string `json:"look,omitempty"`
		Act    string `json:"act,omitempty"`
		ActN   int    `json:"act_n,omitempty"`
		More   int    `json:"more,omitempty"`
		Said   string `json:"said,omitempty"`
		Own    int64  `json:"own,omitempty"`
		// Up: how long the tile's microVM took to start its program, as
		// its kernel counts it, and UpAge how long ago that boot first
		// drew (none when it did not say).
		Up    int64 `json:"up_ms,omitempty"`
		UpAge int64 `json:"up_age_ms,omitempty"`
	}
	now := time.Now()
	s.mu.Lock()
	s.expire(now)
	s.seen(now)
	out := make([]one, len(s.tiles))
	live, up := 0, 0
	fresh := s.host != nil && now.Sub(s.hostAt) <= hostFor
	for i, t := range s.tiles {
		o := one{V: t.ver, Host: t.host, Ms: t.ms, Age: now.Sub(t.at).Milliseconds(),
			Asked: !t.asked.IsZero(), Gen: t.gen, Took: t.took.Milliseconds(), Frames: t.frames, Look: t.look}
		if t.ver > 0 && t.vframes >= 0 {
			v := t.vframes
			o.Vol = &v
		}
		if !t.newAt.IsZero() {
			o.New = max(now.Sub(t.newAt).Milliseconds(), 1)
		}
		if len(t.queue) > 0 {
			o.Act, o.ActN, o.More = t.queue[0].do, t.queue[0].n, len(t.queue)-1
		}
		if fresh {
			o.Own = t.own
		}
		if t.up > 0 {
			o.Up, o.UpAge = t.up, max(now.Sub(t.upAt).Milliseconds(), 1)
		}
		if t.said != "" && now.Sub(t.saidAt) <= sayFor {
			o.Said = t.said
		}
		if t.ver > 0 {
			up++
			if now.Sub(t.at) <= liveFor {
				live++
			}
		}
		out[i] = o
	}
	n := 0
	for _, f := range s.frames {
		if now.Sub(f) <= liveFor {
			n++
		}
	}
	doc := map[string]any{"tiles": out, "live": live, "up": up, "fps": float64(n) / liveFor.Seconds(),
		"since_s": now.Sub(s.start).Seconds(), "full_s": s.full.Seconds(), "full_n": s.fullN,
		"replaced": s.replaced, "keeper": !s.watched.IsZero() && now.Sub(s.watched) <= keeperFor,
		"era": s.era, "reset": s.phase, "reset_s": now.Sub(s.phaseAt).Seconds()}
	if fresh {
		doc["host"] = map[string]any{"tiles": s.host.Tiles, "memory": s.host.Memory, "copies": s.host.Copies,
			"disk": s.host.Disk, "volumes": s.host.Volumes, "age_ms": now.Sub(s.hostAt).Milliseconds()}
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		log.Printf("state: %v", err)
	}
}

// hostSays is the keeper saying what the host holds of the wall, every
// few seconds: the body is hostNow as JSON, each number 0 or more, and own
// a place's own bytes by its number, 0 to maxTiles-1. A place the wall
// does not have (gone since, in a reset) is passed over; a place not named
// holds nothing of its own. What it says is shown for hostFor.
// snapsOf is a page asking for a tile's snapshots: the keeper is asked to
// list them (in /asked, for wantFor), and the list it last gave is
// answered, oldest first, with its age; listed is false until it has
// given one.
func (s *wall) snapsOf(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	s.mu.Lock()
	t, _, ok := s.place(r, false)
	if !ok {
		s.mu.Unlock()
		http.Error(w, "no such tile", http.StatusNotFound)
		return
	}
	t.wanted = now
	doc := map[string]any{"listed": !t.snapsAt.IsZero(), "snapshots": append([]snapRow{}, t.snaps...)}
	if !t.snapsAt.IsZero() {
		doc["age_ms"] = now.Sub(t.snapsAt).Milliseconds()
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		log.Printf("snaps: %v", err)
	}
}

// snapsSaid is the keeper giving a tile's snapshots, as vmx lists them: at
// most snapsMax, each a name a restore may go back to, a time it was
// taken and its bytes, 0 or more. All of it is checked before any is kept.
func (s *wall) snapsSaid(w http.ResponseWriter, r *http.Request) {
	var list []snapRow
	dec := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&list); err != nil {
		http.Error(w, "the body is the tile's snapshots, as vmx lists them in JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(list) > snapsMax {
		http.Error(w, "at most "+strconv.Itoa(snapsMax)+" snapshots", http.StatusBadRequest)
		return
	}
	for _, x := range list {
		if !restorable(x.Name) || x.Created.IsZero() || x.Used < 0 {
			http.Error(w, "a snapshot is a name a restore may go back to, when it was taken and its bytes, 0 or more: not "+strconv.Quote(x.Name), http.StatusBadRequest)
			return
		}
	}
	s.mu.Lock()
	t, _, ok := s.place(r, false)
	if ok {
		t.snaps, t.snapsAt = list, time.Now()
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "no such tile", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *wall) hostSays(w http.ResponseWriter, r *http.Request) {
	var h hostNow
	dec := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&h); err != nil {
		http.Error(w, "the body is what the host holds, as JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if h.Tiles < 0 || h.Memory < 0 || h.Copies < 0 || h.Disk < 0 || h.Volumes < 0 {
		http.Error(w, "the host's numbers are 0 or more", http.StatusBadRequest)
		return
	}
	own := make(map[int]int64, len(h.Own))
	for k, v := range h.Own {
		i, err := strconv.Atoi(k)
		if err != nil || i < 0 || i >= maxTiles || v < 0 {
			http.Error(w, "own is bytes of 0 or more by place, 0 to "+strconv.Itoa(maxTiles-1)+": not "+strconv.Quote(k), http.StatusBadRequest)
			return
		}
		own[i] = v
	}
	h.Own = nil
	s.mu.Lock()
	s.host, s.hostAt = &h, time.Now()
	for i := range s.tiles {
		s.tiles[i].own = own[i]
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *wall) page(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.seen(time.Now())
	s.mu.Unlock()
	// NTILES is the count the info text gives, MAXTILES the most a clone
	// asks for. The page's words keep clear of the six names in capitals,
	// which the replacer takes as its own.
	r := strings.NewReplacer("MAXTILES", strconv.Itoa(maxTiles), "NTILES", strconv.Itoa(s.cols*s.rows), "COLS", strconv.Itoa(s.cols), "ROWS", strconv.Itoa(s.rows), "TW", strconv.Itoa(s.w), "TH", strconv.Itoa(s.h))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := io.WriteString(w, r.Replace(page)); err != nil {
		log.Printf("page: %v", err)
	}
}

func (s *wall) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.page)
	mux.HandleFunc("GET /frame", s.frame)
	mux.HandleFunc("GET /state", s.state)
	mux.HandleFunc("POST /tile/{i}", s.put)
	mux.HandleFunc("GET /tile/{i}", s.get)
	mux.HandleFunc("POST /do/{what}", s.do)
	mux.HandleFunc("GET /asked", s.asked)
	mux.HandleFunc("POST /acting/{i}", s.acting)
	mux.HandleFunc("POST /did/{i}", s.did)
	mux.HandleFunc("POST /reset", s.reset)
	mux.HandleFunc("POST /reset/down", s.down)
	mux.HandleFunc("POST /reset/up", s.up)
	mux.HandleFunc("POST /host", s.hostSays)
	mux.HandleFunc("GET /snaps/{i}", s.snapsOf)
	mux.HandleFunc("POST /snaps/{i}", s.snapsSaid)
	return mux
}

func main() {
	listen := flag.String("listen", ":8090", "where the wall answers")
	cols := flag.Int("cols", 8, "tiles across")
	rows := flag.Int("rows", 5, "tiles down")
	tw := flag.Int("w", 160, "a tile's width in pixels")
	th := flag.Int("h", 120, "a tile's height in pixels")
	flag.Parse()
	if *cols < 1 || *rows < 1 || *cols**rows > maxTiles || *tw < 16 || *th < 16 || *tw > 1024 || *th > 1024 {
		log.Fatal("scene-wall: 1 to " + strconv.Itoa(maxTiles) + " tiles, each 16 to 1024 pixels a side")
	}
	s := newWall(*cols, *rows, *tw, *th, time.Now())
	log.Printf("the wall: %d by %d tiles of %dx%d, on %s", *cols, *rows, *tw, *th, *listen)
	srv := &http.Server{Addr: *listen, Handler: s.routes(), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

const page = `<!doctype html>
<html><head><meta charset="utf-8"><title>Firecracker wall</title>
<style>
body{margin:0;background:#05040a;color:#cfd2ff;font:15px/1.4 ui-monospace,monospace;display:flex;flex-direction:column;align-items:center}
h1{margin:18px 0 6px;font-size:28px;letter-spacing:6px;background:linear-gradient(90deg,#ff3cac,#784ba0,#2b86c5,#3cffb4,#ffe53c,#ff3cac);background-size:300% 100%;-webkit-background-clip:text;background-clip:text;color:transparent;animation:c 6s linear infinite}
@keyframes c{to{background-position:300% 0}}
#bar{display:flex;flex-wrap:wrap;justify-content:center;gap:6px 28px;margin:4px 16px 12px;font-size:17px}
#bar b{color:#fff}
#host{min-height:20px;margin:-6px 16px 12px;font-size:14px;color:#8d90c4;text-align:center}
#host b{color:#3cffb4}
#wall{display:grid;grid-template-columns:repeat(COLS,TWpx);gap:2px;background:#000;padding:2px;border:1px solid #2a2850;user-select:none;-webkit-user-select:none;touch-action:none}
.t{position:relative;width:TWpx;height:THpx;background:#0b0a16;cursor:pointer}
.t img{display:block;width:TWpx;height:THpx;pointer-events:none;-webkit-user-drag:none}
.t img:not([src]){visibility:hidden}
.t span{position:absolute;left:4px;bottom:4px;max-width:calc(100% - 16px);padding:1px 4px;border-radius:2px;background:#05040acc;font-size:10px;line-height:1.3;color:#fff;pointer-events:none}
.t b{position:absolute;right:4px;top:4px;padding:0 4px;border-radius:2px;background:#05040acc;font-size:12px;color:#fff;pointer-events:none;white-space:pre;text-align:right}
.t b:empty{display:none}
.t::after{content:'';position:absolute;inset:0;border:3px solid transparent;pointer-events:none}
.t.sel::after{border-color:#ff3cac;box-shadow:inset 0 0 0 1px #000,inset 0 0 22px #ff3cacbb}
.none span{color:#8d90c4}
.dead img{opacity:.18}
.dead span{color:#ff5c7a}
.asked img{opacity:.3;filter:grayscale(1)}
.busy{outline:2px solid #ffe53c;outline-offset:-2px}
.busy span{color:#ffe53c}
.fresh{outline:2px solid #3cffb4;outline-offset:-2px}
.fresh span{color:#3cffb4}
.said{outline:2px solid #3cd2ff;outline-offset:-2px}
.said span{color:#3cd2ff}
.sel.busy,.sel.fresh,.sel.said{outline-offset:-6px}
#dock{position:sticky;bottom:0;z-index:2;box-sizing:border-box;width:100%;display:flex;flex-direction:column;align-items:center;gap:6px;padding:14px 16px 12px;background:linear-gradient(#05040a00,#05040a 30%)}
#tools{display:flex;flex-wrap:wrap;justify-content:center;align-items:center;gap:8px}
#tools i{width:14px}
#snaps{display:flex;flex-wrap:wrap;justify-content:center;align-items:center;gap:6px;min-height:30px;margin-top:6px;font-size:12px;color:#b8b6d8}
#chosen{min-width:200px;text-align:right;color:#8d90c4;margin-right:4px}
#chosen b{color:#ff3cac}
button{background:#151330;color:#cfd2ff;border:1px solid #3a3870;border-radius:3px;padding:6px 14px;font:inherit;cursor:pointer}
button:hover:not(:disabled){border-color:#ff3cac;color:#fff}
button:disabled{opacity:.35;cursor:default}
#reset{border-color:#8a3a62}
#count{width:5.5em;background:#0b0a16;color:#fff;border:1px solid #3a3870;border-radius:3px;padding:6px 6px;font:inherit;margin-left:-4px}
#count.bad{border-color:#ff5c7a}
#hint{min-height:18px;font-size:13px;color:#8d90c4}
#hint b{color:#ffe53c}
#info{box-sizing:border-box;width:min(94vw,980px);margin:8px 0 32px;padding:12px 22px 16px;background:#0b0a16;border:1px solid #2a2850;font-size:14px;line-height:1.55}
#info[hidden]{display:none}
#info h2{margin:16px 0 4px;font-size:15px;color:#ffe53c;letter-spacing:1px}
#info h2:first-child{margin-top:4px}
#info p{margin:4px 0}
#info ul{margin:4px 0;padding-left:20px}
#info li{margin:3px 0}
#info b{color:#fff}
#info code{color:#3cffb4}
#info table{border-collapse:collapse;margin:6px 0 8px;font-size:13px}
#info th,#info td{border:1px solid #2a2850;padding:3px 10px;text-align:left;vertical-align:top}
#info th{color:#fff;font-weight:normal;background:#151330}
.lg{display:grid;grid-template-columns:repeat(auto-fill,minmax(min(100%,430px),1fr));gap:12px 22px;margin:8px 0 6px}
.lr{display:flex;gap:12px;align-items:flex-start}
.lr p{margin:0;font-size:13px;line-height:1.45}
.lg .t{flex:none;width:160px;height:120px;cursor:default;background:#000;box-shadow:0 0 0 1px #2a2850}
.lg .t img{width:160px;height:120px}
</style></head><body>
<h1>FIRECRACKER WALL</h1>
<div id="bar"><span id="live"></span><span id="fps"></span><span id="clock"></span><span id="new"></span><span id="looks"></span></div>
<div id="host"></div>
<div id="wall"></div>
<div id="dock">
<div id="tools"><span id="chosen"></span><button data-do="snapshot">Snapshot</button><button data-do="rollback">Restore</button><button data-do="rebuild">Rebuild</button><button data-do="update">Update</button><button data-do="kill">Kill</button><button data-do="clone">Clone</button><input id="count" type="number" min="1" max="MAXTILES" value="3" aria-label="how many clones of each tile chosen"><button data-do="remove">Remove</button><i></i><button id="all">All</button><button id="none">None</button><i></i><button id="reset">Reset the wall</button><button id="infobtn" aria-expanded="false" aria-controls="info">info</button></div>
<div id="snaps"></div>
<div id="hint"></div>
</div>
<div id="info" hidden>
<h2>What you are looking at</h2>
<p>One picture, a Mandelbrot zoom, drawn by <b>NTILES</b> copies of one small program. Each copy is a container of its own, and each container is a Firecracker microVM with its own Linux kernel. One more container, this wall, puts the pieces together and serves this page.</p>
<h2>What a tile shows</h2>
<p>Its part of the picture, its name along the bottom, and in its corner two counts of the frames it has drawn. The picture's colours say which version of the tile's program it runs, blue the first and green the new one; the colour of its outline and of its name say what is happening to it.</p>
<div class="lg">
<div class="lr"><div class="t"><img alt="" data-look="blue"><span>tile-07  140 KiB  23 ms</span><b>disk 1,204&#10;volume 3,388</b></div><p><b>Drawing, in blue:</b> its part of the zoom, drawn by the first version of its program. Along the bottom its name, the disk it holds of its own and how long its last frame took to draw; in the corner the frames it has drawn, as its disk counts them and as its volume counts them.</p></div>
<div class="lr"><div class="t"><img alt="" data-look="green"><span>tile-07  green  152 KiB  25 ms</span><b>disk 86&#10;volume 3,474</b></div><p><b>In green: the new version.</b> After Update it runs the new version of its program, which draws in green. Its disk is new, so the disk's count started again; the volume's went on. While some tiles are blue and some green, each names its version.</p></div>
<div class="lr"><div class="t sel"><img alt="" data-look="blue"><span>tile-07  140 KiB  23 ms</span><b>disk 1,204&#10;volume 3,388</b></div><p><b>Chosen</b>, a pink border: the buttons do their verb to it. With every tile chosen the whole wall glows pink, and goes on drawing.</p></div>
<div class="lr"><div class="t busy"><img alt="" data-look="blue"><span>tile-07  taking a snapshot  +1</span><b>disk 1,204&#10;volume 3,388</b></div><p><b>Busy</b>, in yellow: the keeper is doing a press, or it waits its turn: taking a snapshot, restoring, updating, killing it, cloning 3, removing. <b>+1</b> says one more press waits behind it.</p></div>
<div class="lr"><div class="t asked busy"><img alt="" data-look="blue"><span>tile-07  a new one is coming</span><b>disk 1,204&#10;volume 3,388</b></div><p><b>A new machine is coming:</b> after Rebuild or Update the old picture stays, grey, until the new microVM draws.</p></div>
<div class="lr"><div class="t fresh"><img alt="" data-look="blue"><span>tile-07  new in 2.3 s</span><b>disk 9&#10;volume 3,397</b></div><p><b>New</b>, outlined in mint for four seconds: a new boot has the place, a new machine or the same one started again after it died, and how long the place was without a picture.</p></div>
<div class="lr"><div class="t said"><img alt="" data-look="blue"><span>tile-07  snapshot at 06:31:12</span><b>disk 1,204&#10;volume 3,388</b></div><p><b>Said</b>, outlined in cyan for eight seconds: what the keeper says became of the last press. A refusal says why: <i>not updated: a clone stands on its disk; remove the clone first</i>.</p></div>
<div class="lr"><div class="t"><img alt="" data-look="blue"><span>tile-07  up in 0.62 s</span><b>disk 9&#10;volume 3,397</b></div><p><b>Up</b>, for ten seconds after it boots: how long its microVM took from its kernel starting to its program running, as the machine itself counts it.</p></div>
<div class="lr"><div class="t dead"><img alt="" data-look="blue"><span>tile-07  23 ms</span><b>disk 1,204&#10;volume 3,388</b></div><p><b>Not drawing</b>, dim with its name in red: no frame for two seconds. The keeper starts again by itself a tile that died or went; one removed here stays so until Rebuild puts a tile in its place.</p></div>
<div class="lr"><div class="t none"><img alt=""><span>tile-07</span><b></b></div><p><b>No picture yet</b>, its name in grey: its microVM has not drawn its first frame. After a reset every place starts so.</p></div>
</div>
<h2>The two counts in the corner</h2>
<p>Every tile counts the frames it has drawn twice: on its <b>disk</b>, which is the machine's and is made again from the image with it, and on its <b>volume</b>, a ZFS volume of its own that outlives the machine. Each count is written to a file of its own and flushed every second, so a snapshot holds both as they were at one moment. The volume's count runs ahead of the disk's by what the tile drew on the machines it had before this one.</p>
<table>
<tr><th>Button</th><th>The disk's count</th><th>The volume's count</th></tr>
<tr><td>Snapshot</td><td colspan="2">both go on; the snapshot keeps them as they were</td></tr>
<tr><td>Restore</td><td colspan="2">both go back to what the snapshot kept</td></tr>
<tr><td>Rebuild, Update</td><td>starts again from nothing: a new machine has a new disk</td><td>goes on</td></tr>
<tr><td>Kill</td><td colspan="2">both go on from the last count written, a second at most before</td></tr>
<tr><td>Clone</td><td colspan="2">each clone goes on from both, as they were when it was cloned</td></tr>
<tr><td>Remove</td><td colspan="2">both gone, with the tile and its volume</td></tr>
<tr><td>Reset the wall</td><td colspan="2">both start again from nothing, on new machines and new volumes</td></tr>
</table>
<h2>The numbers above the wall</h2>
<ul>
<li><b>live</b>: the microVMs that sent a frame in the last two seconds, of all the places.</li>
<li><b>frames a second</b>: the frames the wall took from all of them over the last two seconds, halved: the work they do together.</li>
<li><b>the clock</b>: how long the wall took to have every place drawing, from its start or from the press of Reset the wall; until then, the seconds since; during a reset, which part of it is under way.</li>
<li><b>replaced</b>: how many times since then a new boot took over a place: every Rebuild and Update, and every tile the keeper started again after it died. A restore starts the same machine again and is not counted.</li>
<li><b>blue</b> and <b>green</b>: how many tiles run each version. During an Update the green count climbs eight at a time until no blue is left; Update again brings the blue back the same way.</li>
</ul>
<h2>How the work is shared out</h2>
<p>The wall hands out the time of the next frame. Each tile knows only its number and where the wall is: it asks for the time, draws its own part of the zoom for that moment, sends the picture back, and goes again, up to eight times a second. The wall keeps every tile's newest picture, and this page shows them side by side. Deeper in the zoom a frame costs more to draw, so the frames a second above are real work, shared by NTILES machines. No tile knows any other: when one goes, the rest carry on, and its place stays dark until it is back.</p>
<h2>When nobody watches</h2>
<p>The tiles draw only while somebody watches. This page asks the wall how it is ten times a second; when nothing has asked for three seconds, the wall holds each tile's request for its next frame, and the tiles draw nothing and take next to no processor, though their microVMs keep running. Open the page again and every waiting tile is let go at once: the picture moves within a frame. The keeper looks for tiles that went every five seconds then, and every second while the page is open.</p>
<h2>What the buttons do</h2>
<p>Click a tile to choose it, or drag across tiles to choose every one you pass; click a chosen tile to let it go. <b>All</b> and <b>None</b> choose every tile or none, and Escape lets them all go. A button does its verb to every tile chosen, at once, and beside the buttons the page says how many are chosen. The table under the two counts says what each button does to them.</p>
<ul>
<li><b>Snapshot</b> takes the tile's disk and its volume as they are, at one moment, in one ZFS call.</li>
<li><b>Restore</b> puts the disk and the volume back to the newest snapshot together: the microVM is stopped, both returned and booted again, and both counts in its corner go back to what they were. With one tile chosen, its snapshots are listed under the buttons, oldest first, with what each holds of its own: a press on one puts the tile back to that one, and the snapshots newer than it go with it. Rebuild and Update give a tile a new machine, and its snapshots go with the old one: a tile rebuilt or updated has only the snapshots taken since.</li>
<li><b>Rebuild</b> destroys the microVM and its disk and boots a new one from the image in its place, on the same volume: the disk's count starts again from nothing and the volume's goes on, as a program's data does when the program is put on a new machine; its snapshots go with the old one. The tile says how long it was away. A place left empty gets a tile again.</li>
<li><b>Update</b> moves the tile to the wall's other image, a new version of its program: blue goes to green, and green back to blue. Its microVM and disk are destroyed and a new one booted from that image on the same volume, eight at a time across the wall while the rest keep drawing, so green rolls over the picture a row at a time. The disk's count starts again and the volume's goes on, as a program's data does through an upgrade; its snapshots go with the old machine. Pressed again, each goes back to the image it came from: the roll back is the same button. This is a rolling update, the way most fleets update a program: the old version is not kept running beside the new, and each tile is without a picture for a second or two while its machine is replaced. A blue/green deployment proper would start every green tile beside its blue one first and then switch the picture over at once.</li>
<li><b>Kill</b> ends the microVM as a crash or a power cut would: its process killed outright, with no shutdown. Nothing asks for it back: the keeper finds the machine stopped, as it would one that died any other way, and starts it again with its disk and its volume as they were, and the tile says how long it was away.</li>
<li><b>Clone</b> makes as many new microVMs from the tile's disk and volume as they are, at one moment, as the number beside it says, for each tile chosen: each a ZFS clone, its volume one too, so it copies nothing and takes no space until it writes, and each draws a new place below the picture, counting on from the counts it was cloned with. The wall takes MAXTILES places; the host's memory and the addresses its network gives out end most walls sooner, and the tile says which. The box beside the button turns red when it holds anything but a whole number from 1 to MAXTILES.</li>
<li><b>Remove</b> takes the tile away with its volume and their snapshots, and its place stays empty.</li>
<li><b>Reset the wall</b> takes every tile and every clone away with its volume and runs the picture's own tiles again from the image on new volumes, and the clock above says how long the whole wall took, from the press.</li>
</ul>
<h2>The line under the clock</h2>
<p>What the host holds of the wall, as the keeper reads it from the host every few seconds: how many of its microVMs run, the memory they take (each microVM's resident memory on the host), and their disks counted two ways: as full copies would take them, and as ZFS keeps them, each tile's own changes and the image they all share counted once; then their volumes, as ZFS keeps them, which is mostly the room it sets aside for each, written to or not. Each tile shows the disk it holds of its own beside its name.</p>
<h2>Who does it</h2>
<p>The page cannot do any of that by itself, and that is the point: it is a program in a container, and nothing in a container commands the host. A button only leaves a request on the wall. The keeper, a small loop on the host (<code>05-demoscene-wall.sh</code>, running as the service <code>demoscene-keeper</code>), asks the wall what was asked, does it with the <code>vmx</code> command and says what became of it, which the tile shows. It also runs again any tile that is gone or stopped, however it went. Every tile sends an id made when its program starts, so the wall can tell a new machine from the old one. With no keeper watching, a request waits, and the page says so. The same requests can come from a script: <code>05-demoscene-wall.sh --show</code> presses the buttons in a fixed order, each step waited out, for a recording.</p>
<h2>How it is put together</h2>
<p>This runs on a vmx host. Each image is pulled once, or imported from an archive, by its digest, and unpacked onto a ZFS volume kept as its golden. Every container is a ZFS clone of that golden, so a new one copies nothing and takes no space until it writes. It boots in a Firecracker microVM, confined by Firecracker's jailer, running its own copy of the kernel the host runs, with a small init that starts the image's program. The tiles talk to the wall on the host's private network, each with its address the moment it asks; the wall's port is forwarded from the host, which is how this page reaches you.</p>
<h2>Why build it this way</h2>
<ul>
<li><b>A kernel each.</b> A crash, a kernel bug or a runaway program in one tile stays in that tile. Containers that share one kernel cannot promise that.</li>
<li><b>Nothing is fixed in place.</b> A tile that misbehaves is thrown away and a new one made from the same golden, so repairing a tile and starting a new one are the same act.</li>
<li><b>Identical by construction.</b> Every tile runs the same image, named by its digest, so a replacement is the same bytes as the one it replaced.</li>
<li><b>Cheap to keep and to throw away.</b> A snapshot is a moment, a restore a reboot, a clone copies nothing, and each rebuilt tile shows how long it was away.</li>
<li><b>The host decides.</b> The page and the containers can only ask; what runs is chosen on the host.</li>
</ul>
<h2>Where the same shape fits</h2>
<ul>
<li>A render or batch farm: one job cut into pieces, each piece handled by a copy of one program, such as the frames of a film, the tiles of a map or the chunks of a dataset.</li>
<li>CI runners: each build in a microVM of its own, thrown away after it, so nothing one job leaves behind reaches the next.</li>
<li>Code you did not write, such as plugins, customers' scripts or code an AI agent wrote, each run behind its own kernel.</li>
<li>One service scaled out: many copies of one web or API program, any one of them replaced while the others keep serving.</li>
<li>A sandbox each: a lab machine per student, a game server per match, a preview per pull request.</li>
<li>Practising failure: kill pieces on purpose and watch the rest carry on, as here.</li>
</ul>
</div>
<script>
const n=COLS*ROWS,$=id=>document.getElementById(id),wall=$('wall'),tiles=[],sel=new Set();
const verbs=[...document.querySelectorAll('#tools button[data-do]')];
const doing={snapshot:'taking a snapshot',rollback:'restoring',rebuild:'a new one is coming',update:'updating',kill:'killing it',clone:'cloning',remove:'removing'};
const count=$('count'),most=MAXTILES;
// size is bytes in the units ZFS and the host count in.
const size=b=>{const u=['B','KiB','MiB','GiB','TiB'];let k=0;while(b>=1024&&k<u.length-1){b/=1024;k++;}return(k&&b<10?b.toFixed(1):Math.round(b))+' '+u[k];};
// clones is the count beside Clone, or 0 when it is not a whole number
// from 1 to the most the wall takes.
function clones(){const v=count.value.trim(),k=/^[0-9]+$/.test(v)?+v:0;return k>=1&&k<=most?k:0;}
count.addEventListener('input',()=>count.classList.toggle('bad',!clones()));
let era=0,held=false,painting=null;
const pad=i=>String(i).padStart(2,'0');
function tools(){$('chosen').innerHTML=sel.size?'<b>'+sel.size+'</b> chosen':'click or drag to choose';verbs.forEach(b=>b.disabled=!sel.size||held);$('all').disabled=held;$('none').disabled=!sel.size;}
function choose(i,on){const o=tiles[i];if(!o)return;if(on)sel.add(i);else sel.delete(i);o.d.classList.toggle('sel',on);}
function add(i){const d=document.createElement('div');d.className='t dead';d.dataset.i=i;const im=new Image();im.alt='';im.draggable=false;const l=document.createElement('span');l.textContent='tile-'+pad(i);const c=document.createElement('b');d.append(im,l,c);wall.append(d);tiles.push({d,im,l,c,v:-1});}
for(let i=0;i<n;i++)add(i);
// A press chooses the tile or lets it go; moving on with the button held
// does the same to every tile passed over.
wall.addEventListener('pointerdown',e=>{const d=e.target.closest('.t');if(!d||e.button!==0)return;e.preventDefault();const i=+d.dataset.i;painting=!sel.has(i);choose(i,painting);tools();});
document.addEventListener('pointermove',e=>{if(painting===null)return;const d=document.elementFromPoint(e.clientX,e.clientY)?.closest('#wall .t');if(d)choose(+d.dataset.i,painting);tools();});
for(const k of ['pointerup','pointercancel'])document.addEventListener(k,()=>{painting=null;});
verbs.forEach(b=>b.onclick=()=>{const what=b.dataset.do,chosen=[...sel].sort((a,c)=>a-c);if(!chosen.length)return;let path='/do/'+what;
if(what==='clone'){const k=clones();if(!k){count.classList.add('bad');count.focus();return;}path+='?n='+k;}
fetch(path,{method:'POST',body:JSON.stringify(chosen)});if(what==='remove')chosen.forEach(i=>choose(i,false));tools();});
$('all').onclick=()=>{tiles.forEach((o,i)=>choose(i,true));tools();};
$('none').onclick=()=>{[...sel].forEach(i=>choose(i,false));tools();};
$('reset').onclick=()=>{fetch('/reset',{method:'POST'});};
// The one tile chosen: its snapshots, oldest first, as the keeper lists
// them, each a button that puts the tile back to that one; the newer ones
// go with it, as a restore's do.
const clockOf=t=>new Date(t).toLocaleTimeString('en-GB');
async function snapsTick(){const el=$('snaps');if(sel.size!==1||held){el.replaceChildren();return;}
const i=[...sel][0],name='tile-'+pad(i);let d;try{d=await(await fetch('/snaps/'+i)).json();}catch(e){return;}
if(sel.size!==1||[...sel][0]!==i)return;
if(!d.listed){el.textContent=name+': its snapshots are being listed';return;}
const rows=d.snapshots||[];if(!rows.length){el.textContent=name+': no snapshot yet';return;}
const head=document.createElement('span');head.textContent=name+', back to:';
el.replaceChildren(head,...rows.map((r,k)=>{const b=document.createElement('button'),newer=rows.length-1-k;
b.textContent=clockOf(r.created)+'  '+size(r.used_bytes);b.title=newer?'the '+newer+' newer '+(newer>1?'snapshots go':'snapshot goes')+' with it':'the newest';
b.onclick=()=>{fetch('/do/rollback?at='+encodeURIComponent(r.name),{method:'POST',body:JSON.stringify([i])});};return b;}));}
setInterval(snapsTick,1000);
tools();
// label is what tile i says along its bottom, and how it is outlined;
// mixed is whether the wall runs more than one version, when each tile
// names its own.
function label(t,i,mixed){const name=t.host||'tile-'+pad(i);
if(t.act)return[name+'  '+(doing[t.act]||t.act)+(t.act_n?' '+t.act_n:'')+(t.more?'  +'+t.more:''),'busy'];
if(t.asked)return[name+'  '+doing.rebuild,'busy'];
if(t.new_ms>0&&t.new_ms<4000)return[name+'  new in '+(t.took_ms/1000).toFixed(1)+' s','fresh'];
if(t.said)return[name+'  '+t.said,'said'];
if(t.up_ms>0&&t.up_age_ms<10000)return[name+'  up in '+(t.up_ms/1000).toFixed(2)+' s',''];
return[name+(mixed&&t.look?'  '+t.look:'')+(t.own?'  '+size(t.own):'')+(t.ms?'  '+t.ms+' ms':''),''];}
async function tick(){try{const s=await(await fetch('/state')).json();
if(s.era!==era){era=s.era;tiles.forEach((o,i)=>{o.v=-1;o.im.removeAttribute('src');choose(i,false);});}
while(tiles.length<s.tiles.length)add(tiles.length);
while(tiles.length>s.tiles.length){const o=tiles.pop();sel.delete(tiles.length);o.d.remove();}
// How many tiles draw each version: a tile is counted by its newest
// picture while that is under ten seconds old, so one being replaced keeps
// its count until the new machine draws, and one removed drops out.
const by={};s.tiles.forEach(t=>{if(t.v>0&&t.age_ms<10000&&t.look)by[t.look]=(by[t.look]||0)+1;});
const kinds=Object.keys(by).sort(),mixed=kinds.length>1;
s.tiles.forEach((t,i)=>{const o=tiles[i];if(t.v>0&&t.v!==o.v){o.v=t.v;o.im.src='/tile/'+i+'?e='+era+'&v='+t.v;}
o.c.textContent=t.frames?(t.vol_frames!==undefined?'disk '+t.frames.toLocaleString('en-US')+'\nvolume '+t.vol_frames.toLocaleString('en-US'):t.frames.toLocaleString('en-US')):'';
const[text,kind]=label(t,i,mixed);o.l.textContent=text;
o.d.classList.toggle('none',t.v===0);o.d.classList.toggle('dead',t.v>0&&t.age_ms>2000);o.d.classList.toggle('asked',t.asked);
for(const k of ['busy','fresh','said'])o.d.classList.toggle(k,kind===k);});
held=!!s.reset;
$('live').innerHTML='<b>'+s.live+'</b> of '+s.tiles.length+' microVMs live';$('fps').innerHTML='<b>'+s.fps.toFixed(1)+'</b> frames a second';
$('clock').innerHTML=s.reset==='asked'?'a reset is asked':s.reset==='down'?'taking the wall down, <b>'+s.reset_s.toFixed(1)+' s</b>':s.reset==='up'?'the wall comes back: <b>'+s.up+'</b> of '+s.tiles.length+', <b>'+s.since_s.toFixed(1)+' s</b>':s.full_s>0?'all '+s.full_n+' up in <b>'+s.full_s.toFixed(1)+' s</b>':'<b>'+s.since_s.toFixed(1)+' s</b> since the start';
$('new').innerHTML=s.replaced>0?'<b>'+s.replaced+'</b> replaced':'';
$('looks').innerHTML=kinds.map(k=>'<b>'+by[k]+'</b> '+k).join(' &middot; ');
const h=s.host;$('host').innerHTML=h?'<b>'+h.tiles+'</b> microVMs &middot; <b>'+size(h.memory)+'</b> of memory &middot; their disks: <b>'+size(h.copies)+'</b> as full copies, <b>'+size(h.disk)+'</b> as ZFS keeps them'+(h.volumes?' &middot; their volumes: <b>'+size(h.volumes)+'</b>':''):'';
$('reset').disabled=!!s.reset;$('reset').textContent=s.reset?'Resetting':'Reset the wall';
const waiting=s.reset==='asked'||s.tiles.some(t=>t.act||t.asked);
$('hint').innerHTML=!s.keeper&&waiting?'<b>no keeper is watching</b>: nothing asked here is done until the host runs 05-demoscene-wall.sh --keep':'';
tools();}catch(e){}setTimeout(tick,100);}
const ib=$('infobtn'),info=$('info');
function showInfo(on){info.hidden=!on;ib.setAttribute('aria-expanded',on?'true':'false');if(on)info.scrollIntoView({behavior:'smooth',block:'start'});}
ib.onclick=()=>showInfo(info.hidden);
// The page's address ending #info opens it with the info shown.
if(location.hash==='#info')showInfo(true);
document.addEventListener('keydown',e=>{if(e.key!=='Escape')return;if(!info.hidden)showInfo(false);else $('none').onclick();});
// The legend's tiles in the info: each picture is a swatch of its look's
// colours, sampled from the tile program's own (agent/main.go, shade).
const swatches={blue:['000000','000768','031c93','0f3fb4','3271d0','7ab1e8','ffffff'],green:['000000','076800','1c9303','3fb40f','71d032','b1e87a','ffffff']};
const swatch=c=>'data:image/svg+xml,'+encodeURIComponent("<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 4 3' preserveAspectRatio='none'><linearGradient id='g' x2='1' y2='1'>"+c.map((h,k)=>"<stop offset='"+(k/(c.length-1)).toFixed(3)+"' stop-color='#"+h+"'/>").join('')+"</linearGradient><rect width='4' height='3' fill='url(#g)'/></svg>");
document.querySelectorAll('.lg img[data-look]').forEach(im=>{im.src=swatch(swatches[im.dataset.look]);});
tick();
</script></body></html>`
