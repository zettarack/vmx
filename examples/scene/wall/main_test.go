package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// ask sends one request to the wall's routes and gives back its status.
func ask(t *testing.T, s *wall, method, path, body string, header ...string) int {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for k := 0; k+1 < len(header); k += 2 {
		r.Header.Set(header[k], header[k+1])
	}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	return w.Code
}

// frame is place i drawing a frame from the microVM boot.
func frame(t *testing.T, s *wall, i int, boot string) {
	t.Helper()
	if c := ask(t, s, "POST", "/tile/"+itoa(i), "\x89PNG\r\n\x1a\nxx", "X-Boot", boot, "X-Host", "tile-"+itoa(i), "X-Frames", "12"); c != http.StatusNoContent {
		t.Fatalf("a frame for %d: %d", i, c)
	}
}

func itoa(i int) string { return strconv.Itoa(i) }

type acts struct {
	Acts []struct {
		Tile int    `json:"tile"`
		Do   string `json:"do"`
		N    int    `json:"n"`
	} `json:"acts"`
	Reset bool `json:"reset"`
}

func askedOf(t *testing.T, s *wall) acts {
	t.Helper()
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/asked", nil))
	var a acts
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatalf("asked: %v: %s", err, w.Body.String())
	}
	return a
}

type stateDoc struct {
	Tiles []struct {
		V      int    `json:"v"`
		Asked  bool   `json:"asked"`
		Gen    int    `json:"gen"`
		New    int64  `json:"new_ms"`
		Took   int64  `json:"took_ms"`
		Act    string `json:"act"`
		ActN   int    `json:"act_n"`
		More   int    `json:"more"`
		Said   string `json:"said"`
		Frames int64  `json:"frames"`
		Own    int64  `json:"own"`
	} `json:"tiles"`
	Host *struct {
		Tiles  int   `json:"tiles"`
		Memory int64 `json:"memory"`
		Copies int64 `json:"copies"`
		Disk   int64 `json:"disk"`
		Age    int64 `json:"age_ms"`
	} `json:"host"`
	Up       int     `json:"up"`
	Full     float64 `json:"full_s"`
	FullN    int     `json:"full_n"`
	Replaced int     `json:"replaced"`
	Era      int     `json:"era"`
	Reset    string  `json:"reset"`
	Keeper   bool    `json:"keeper"`
}

func stateOf(t *testing.T, s *wall) stateDoc {
	t.Helper()
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/state", nil))
	var d stateDoc
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatalf("state: %v: %s", err, w.Body.String())
	}
	return d
}

func fourTiles() *wall { return newWall(2, 2, 16, 16, time.Now()) }

// verbsOf is a queue's verbs, as words.
func verbsOf(q []press) string {
	var w []string
	for _, p := range q {
		w = append(w, p.do)
	}
	return strings.Join(w, " ")
}

func TestAButtonsVerbsWaitForTheKeeperInTheirOrder(t *testing.T) {
	s := fourTiles()
	if c := ask(t, s, "POST", "/do/snapshot", "[1,3]"); c != http.StatusNoContent {
		t.Fatalf("snapshot: %d", c)
	}
	if c := ask(t, s, "POST", "/do/rollback", "[3]"); c != http.StatusNoContent {
		t.Fatalf("rollback: %d", c)
	}
	a := askedOf(t, s)
	if len(a.Acts) != 2 || a.Acts[0].Tile != 1 || a.Acts[0].Do != "snapshot" || a.Acts[1].Tile != 3 || a.Acts[1].Do != "snapshot" {
		t.Fatalf("each tile's first verb, and only that: %+v", a.Acts)
	}
	if !stateOf(t, s).Keeper {
		t.Fatal("a keeper that asked is watching")
	}
	ask(t, s, "POST", "/acting/3", "")
	if a := askedOf(t, s); len(a.Acts) != 1 || a.Acts[0].Tile != 1 {
		t.Fatalf("a verb taken is not handed out again: %+v", a.Acts)
	}
	if d := stateOf(t, s); d.Tiles[3].Act != "snapshot" || d.Tiles[3].More != 1 {
		t.Fatalf("tile 3 does a snapshot with one more waiting: %+v", d.Tiles[3])
	}
	ask(t, s, "POST", "/did/3", "snapshot at 10:00:00")
	a = askedOf(t, s)
	if len(a.Acts) != 2 || a.Acts[1].Tile != 3 || a.Acts[1].Do != "rollback" {
		t.Fatalf("the next verb after the first is done: %+v", a.Acts)
	}
	if d := stateOf(t, s); d.Tiles[3].Said != "snapshot at 10:00:00" {
		t.Fatalf("what the keeper said is on the tile: %+v", d.Tiles[3])
	}
}

func TestAPressTwiceWaitsOnceAndATileHoldsFourAtMost(t *testing.T) {
	s := fourTiles()
	ask(t, s, "POST", "/do/snapshot", "[2,2]")
	ask(t, s, "POST", "/do/snapshot", "[2]")
	if q := s.tiles[2].queue; len(q) != 1 {
		t.Fatalf("one snapshot waits, not %v", q)
	}
	for _, v := range []string{"rollback", "snapshot", "rollback", "snapshot"} {
		ask(t, s, "POST", "/do/"+v, "[2]")
	}
	if q := s.tiles[2].queue; len(q) != queueMax || verbsOf(q) != "snapshot rollback snapshot rollback" {
		t.Fatalf("four wait, the fifth is not kept: %v", q)
	}
}

func TestABadRequestIsRefusedAndKeepsNothing(t *testing.T) {
	s := fourTiles()
	many := "[" + strings.TrimSuffix(strings.Repeat("0,", maxTiles+1), ",") + "]"
	for _, c := range []struct {
		path, body string
		want       int
	}{
		{"/do/format", "[1]", http.StatusNotFound},
		{"/do/snapshot", "", http.StatusBadRequest},
		{"/do/snapshot", "[]", http.StatusBadRequest},
		{"/do/snapshot", "1", http.StatusBadRequest},
		{"/do/snapshot", `["1"]`, http.StatusBadRequest},
		{"/do/snapshot", `[1.5]`, http.StatusBadRequest},
		{"/do/snapshot", many, http.StatusBadRequest},
		{"/do/snapshot", "[0,4]", http.StatusNotFound},
		{"/do/snapshot", "[-1]", http.StatusNotFound},
		{"/acting/4", "", http.StatusNotFound},
		{"/acting/x", "", http.StatusNotFound},
		{"/did/-1", "", http.StatusNotFound},
		{"/did/" + itoa(maxTiles), "", http.StatusNotFound},
		{"/tile/" + itoa(maxTiles), "\x89PNG\r\n\x1a\n", http.StatusNotFound},
		{"/tile/0", "GIF89a", http.StatusBadRequest},
		{"/reset/down", "", http.StatusConflict},
		{"/reset/up", "", http.StatusConflict},
	} {
		if got := ask(t, s, "POST", c.path, c.body); got != c.want {
			t.Errorf("POST %s %q: %d, want %d", c.path, c.body, got, c.want)
		}
	}
	for i, o := range s.tiles {
		if len(o.queue) != 0 {
			t.Errorf("tile %d keeps %v after refusals", i, o.queue)
		}
	}
	if len(s.tiles) != 4 {
		t.Errorf("a refused request grew the wall to %d", len(s.tiles))
	}
}

func TestARebuildIsTimedFromThePressAndCounted(t *testing.T) {
	s := fourTiles()
	frame(t, s, 2, "a")
	ask(t, s, "POST", "/do/rebuild", "[2]")
	if d := stateOf(t, s); !d.Tiles[2].Asked || d.Tiles[2].Act != "rebuild" {
		t.Fatalf("a rebuild is asked at the press: %+v", d.Tiles[2])
	}
	// Pressed two seconds ago: the place is timed from then, not from its
	// last frame, a moment ago.
	s.tiles[2].asked = time.Now().Add(-2 * time.Second)
	ask(t, s, "POST", "/acting/2", "")
	ask(t, s, "POST", "/did/2", "")
	if d := stateOf(t, s); !d.Tiles[2].Asked || d.Tiles[2].Act != "" || d.Tiles[2].Said != "" {
		t.Fatalf("done and silent, still waiting for the new one to draw: %+v", d.Tiles[2])
	}
	frame(t, s, 2, "b")
	d := stateOf(t, s)
	if o := d.Tiles[2]; o.Asked || o.Gen != 2 || o.New <= 0 || o.Took < 2000 || d.Replaced != 1 {
		t.Fatalf("a new microVM has the place: %+v, replaced %d", o, d.Replaced)
	}
}

func TestAFailedRebuildStopsWaiting(t *testing.T) {
	s := fourTiles()
	frame(t, s, 1, "a")
	ask(t, s, "POST", "/do/rebuild", "[1]")
	ask(t, s, "POST", "/acting/1", "")
	ask(t, s, "POST", "/did/1", "not rebuilt: no room", "X-Failed", "1")
	if o := stateOf(t, s).Tiles[1]; o.Asked || o.Said != "not rebuilt: no room" {
		t.Fatalf("a failed rebuild says why and waits no more: %+v", o)
	}
}

func TestARestoreIsTheSameMachineStartedAgain(t *testing.T) {
	s := fourTiles()
	frame(t, s, 0, "a")
	ask(t, s, "POST", "/do/rollback", "[0]")
	ask(t, s, "POST", "/acting/0", "")
	ask(t, s, "POST", "/did/0", "back to 10:00:00 in 0.5 s")
	frame(t, s, 0, "b")
	d := stateOf(t, s)
	if o := d.Tiles[0]; o.Gen != 1 || o.New != 0 || d.Replaced != 0 {
		t.Fatalf("a restore is no replacement: %+v, replaced %d", o, d.Replaced)
	}
	// Its window is spent: the next new boot is a new machine.
	frame(t, s, 0, "c")
	if d := stateOf(t, s); d.Replaced != 1 || d.Tiles[0].Gen != 2 {
		t.Fatalf("a boot after the restore's is a new machine: replaced %d, %+v", d.Replaced, d.Tiles[0])
	}
}

func TestARestoreThatFailedLeavesTheNextBootANewMachine(t *testing.T) {
	s := fourTiles()
	frame(t, s, 0, "a")
	ask(t, s, "POST", "/do/rollback", "[0]")
	ask(t, s, "POST", "/acting/0", "")
	ask(t, s, "POST", "/did/0", "no snapshot to restore yet", "X-Failed", "1")
	frame(t, s, 0, "b")
	if d := stateOf(t, s); d.Replaced != 1 {
		t.Fatalf("not restored, so a new boot is a new machine: replaced %d", d.Replaced)
	}
}

func TestARestoreDrawingBeforeItIsSaidDoneIsNotLeftOpen(t *testing.T) {
	s := fourTiles()
	frame(t, s, 0, "a")
	ask(t, s, "POST", "/do/rollback", "[0]")
	ask(t, s, "POST", "/acting/0", "")
	frame(t, s, 0, "b")
	ask(t, s, "POST", "/did/0", "back to 10:00:00 in 0.5 s")
	frame(t, s, 0, "c")
	if d := stateOf(t, s); d.Replaced != 1 {
		t.Fatalf("the restore drew first; a boot after is a new machine: replaced %d", d.Replaced)
	}
}

func TestAVerbTakenAndNeverSaidIsHandedOutAgain(t *testing.T) {
	s := fourTiles()
	ask(t, s, "POST", "/do/clone", "[1]")
	ask(t, s, "POST", "/acting/1", "")
	if a := askedOf(t, s); len(a.Acts) != 0 {
		t.Fatalf("taken: %+v", a.Acts)
	}
	s.tiles[1].actAt = time.Now().Add(-actFor - time.Second)
	if a := askedOf(t, s); len(a.Acts) != 1 || a.Acts[0].Do != "clone" {
		t.Fatalf("taken and never said in %v: asked again, %+v", actFor, a.Acts)
	}
}

func TestAClonesPlaceIsToldBeforeItDraws(t *testing.T) {
	s := fourTiles()
	if c := ask(t, s, "POST", "/did/6", "a clone of tile-01"); c != http.StatusNoContent {
		t.Fatalf("did: %d", c)
	}
	d := stateOf(t, s)
	if len(d.Tiles) != 7 || d.Tiles[6].Said != "a clone of tile-01" || d.Tiles[6].V != 0 {
		t.Fatalf("the wall grows to the clone's place: %+v", d.Tiles)
	}
	frame(t, s, 6, "x")
	if d := stateOf(t, s); d.Tiles[6].V != 1 || d.Tiles[6].Frames != 12 {
		t.Fatalf("and it draws there: %+v", d.Tiles[6])
	}
}

func TestWhatIsSaidIsCutAtACharacter(t *testing.T) {
	s := fourTiles()
	// One byte, then two-byte characters: byte 120 falls inside one.
	ask(t, s, "POST", "/did/0", "x"+strings.Repeat("é", 100))
	got := stateOf(t, s).Tiles[0].Said
	if len(got) > 120 || !utf8.ValidString(got) || len(got) < 118 {
		t.Fatalf("120 bytes at most, whole characters: %d bytes, valid %v", len(got), utf8.ValidString(got))
	}
}

func TestAResetTakesTheWallDownAndTimesItComingBack(t *testing.T) {
	s := fourTiles()
	for i := range 4 {
		frame(t, s, i, "a"+itoa(i))
	}
	ask(t, s, "POST", "/did/5", "a clone of tile-00")
	ask(t, s, "POST", "/do/snapshot", "[0]")
	if d := stateOf(t, s); d.Full == 0 || d.FullN != 4 {
		t.Fatalf("the wall was up: %+v", d)
	}
	ask(t, s, "POST", "/reset", "")
	if a := askedOf(t, s); !a.Reset {
		t.Fatal("a reset is asked of the keeper")
	}
	if c := ask(t, s, "POST", "/do/rebuild", "[1]"); c != http.StatusConflict {
		t.Fatalf("no verb while a reset is asked: %d", c)
	}
	ask(t, s, "POST", "/reset", "")
	if c := ask(t, s, "POST", "/reset/down", ""); c != http.StatusNoContent {
		t.Fatalf("down: %d", c)
	}
	if c := ask(t, s, "POST", "/reset/down", ""); c != http.StatusConflict {
		t.Fatalf("taken once: %d", c)
	}
	d := stateOf(t, s)
	if a := askedOf(t, s); a.Reset || len(a.Acts) != 0 || d.Reset != "down" || d.Tiles[0].Act != "" {
		t.Fatalf("taken: no reset and no verb asked, down: %+v %+v", a, d.Reset)
	}
	if c := ask(t, s, "POST", "/reset/up", ""); c != http.StatusNoContent {
		t.Fatalf("up: %d", c)
	}
	d = stateOf(t, s)
	if d.Reset != "up" || len(d.Tiles) != 4 || d.Up != 0 || d.Era != 1 || d.Full != 0 || d.Replaced != 0 {
		t.Fatalf("the picture's own places, none drawing, a new era: %+v", d)
	}
	if c := ask(t, s, "POST", "/do/snapshot", "[0]"); c != http.StatusConflict {
		t.Fatalf("no verb while the tiles come back: %d", c)
	}
	for i := range 3 {
		frame(t, s, i, "b"+itoa(i))
	}
	if d := stateOf(t, s); d.Reset != "up" || d.Up != 3 {
		t.Fatalf("three of four back: %+v", d)
	}
	frame(t, s, 3, "b3")
	if d := stateOf(t, s); d.Reset != "" || d.Full == 0 || d.FullN != 4 {
		t.Fatalf("every place draws: the reset is over, timed: %+v", d)
	}
	if c := ask(t, s, "POST", "/do/rebuild", "[1]"); c != http.StatusNoContent {
		t.Fatalf("verbs again after it: %d", c)
	}
}

func TestAResetThatStallsEnds(t *testing.T) {
	s := fourTiles()
	ask(t, s, "POST", "/reset", "")
	ask(t, s, "POST", "/reset/down", "")
	s.phaseAt = time.Now().Add(-resetFor - time.Second)
	if d := stateOf(t, s); d.Reset != "" {
		t.Fatalf("stalled past %v: over, %q", resetFor, d.Reset)
	}
	if c := ask(t, s, "POST", "/do/snapshot", "[0]"); c != http.StatusNoContent {
		t.Fatalf("verbs again: %d", c)
	}
}

// A clone says how many, and the keeper is told it with the verb; one that
// does not say is three. Pressed again with the same count it waits once,
// with another count twice; no other verb carries a count.
func TestACloneSaysHowManyAndTheKeeperIsToldIt(t *testing.T) {
	s := fourTiles()
	for _, c := range []struct{ path, body string }{
		{"/do/clone?n=10", "[1]"}, {"/do/clone", "[2]"}, {"/do/clone?n=" + itoa(maxTiles), "[3]"}, {"/do/snapshot", "[0]"},
	} {
		if got := ask(t, s, "POST", c.path, c.body); got != http.StatusNoContent {
			t.Fatalf("POST %s %s: %d", c.path, c.body, got)
		}
	}
	got := map[int][2]any{}
	for _, a := range askedOf(t, s).Acts {
		got[a.Tile] = [2]any{a.Do, a.N}
	}
	want := map[int][2]any{0: {"snapshot", 0}, 1: {"clone", 10}, 2: {"clone", 3}, 3: {"clone", maxTiles}}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("tile %d asked %v, want %v", i, got[i], w)
		}
	}
	if d := stateOf(t, s); d.Tiles[1].Act != "clone" || d.Tiles[1].ActN != 10 || d.Tiles[0].ActN != 0 {
		t.Errorf("the page is told the count: %+v %+v", d.Tiles[1], d.Tiles[0])
	}
	ask(t, s, "POST", "/do/clone?n=10", "[1]")
	if q := s.tiles[1].queue; len(q) != 1 {
		t.Errorf("the same clone twice waits once: %v", q)
	}
	ask(t, s, "POST", "/do/clone?n=5", "[1]")
	if q := s.tiles[1].queue; len(q) != 2 || q[1] != (press{"clone", 5}) {
		t.Errorf("a clone of another count waits after it: %v", q)
	}
	ask(t, s, "POST", "/acting/1", "")
	ask(t, s, "POST", "/did/1", "cloned 10")
	if a := askedOf(t, s); len(a.Acts) != 4 || a.Acts[1].Tile != 1 || a.Acts[1].N != 5 {
		t.Errorf("the next clone, with its count: %+v", a.Acts)
	}
}

// A count that is not one number from 1 to maxTiles is refused, and so is
// a count on any verb but a clone; nothing is kept.
func TestACountThatIsNoneIsRefused(t *testing.T) {
	s := fourTiles()
	for _, path := range []string{"/do/clone?n=0", "/do/clone?n=-1", "/do/clone?n=" + itoa(maxTiles+1), "/do/clone?n=x", "/do/clone?n=1.5",
		"/do/clone?n=", "/do/clone?n=1&n=2", "/do/clone?n=99999999999999999999", "/do/snapshot?n=3", "/do/remove?n=1"} {
		if got := ask(t, s, "POST", path, "[1]"); got != http.StatusBadRequest {
			t.Errorf("POST %s: %d, want %d", path, got, http.StatusBadRequest)
		}
	}
	for i, o := range s.tiles {
		if len(o.queue) != 0 {
			t.Errorf("tile %d keeps %v after refusals", i, o.queue)
		}
	}
}

// What the keeper says of the host is shown: its numbers, each tile's own
// disk, and how old it is. A place it does not name holds nothing of its
// own, one the wall does not have is passed over, and after hostFor none
// of it is shown.
func TestTheHostIsShownAsTheKeeperSaysIt(t *testing.T) {
	s := fourTiles()
	if d := stateOf(t, s); d.Host != nil || d.Tiles[0].Own != 0 {
		t.Fatalf("nothing said, nothing shown: %+v", d.Host)
	}
	body := `{"tiles":4,"memory":402653184,"copies":159383552,"disk":38076416,"own":{"0":8192,"2":12288,"700":4096}}`
	if got := ask(t, s, "POST", "/host", body); got != http.StatusNoContent {
		t.Fatalf("POST /host: %d", got)
	}
	d := stateOf(t, s)
	if h := d.Host; h == nil || h.Tiles != 4 || h.Memory != 402653184 || h.Copies != 159383552 || h.Disk != 38076416 || h.Age < 0 || h.Age > 1000 {
		t.Fatalf("the host as said: %+v", h)
	}
	if d.Tiles[0].Own != 8192 || d.Tiles[1].Own != 0 || d.Tiles[2].Own != 12288 || len(s.tiles) != 4 {
		t.Errorf("each tile's own, and no place grown: %+v, %d places", d.Tiles, len(s.tiles))
	}
	ask(t, s, "POST", "/host", `{"tiles":3,"memory":1,"copies":2,"disk":3,"own":{"1":512}}`)
	if d := stateOf(t, s); d.Tiles[0].Own != 0 || d.Tiles[1].Own != 512 || d.Host.Tiles != 3 {
		t.Errorf("the newest said is what is shown: %+v %+v", d.Host, d.Tiles)
	}
	s.mu.Lock()
	s.hostAt = time.Now().Add(-hostFor - time.Second)
	s.mu.Unlock()
	if d := stateOf(t, s); d.Host != nil || d.Tiles[1].Own != 0 {
		t.Errorf("what is older than hostFor is not shown: %+v %+v", d.Host, d.Tiles[1])
	}
}

// What the host says is checked before any of it is kept: a body that is
// not its JSON, a field it does not have, a number below 0, a place that is
// not a number from 0 to maxTiles-1.
func TestWhatTheHostSaysIsCheckedBeforeAnyIsKept(t *testing.T) {
	s := fourTiles()
	good := `{"tiles":4,"memory":1,"copies":2,"disk":3,"own":{"0":5}}`
	if got := ask(t, s, "POST", "/host", good); got != http.StatusNoContent {
		t.Fatalf("POST /host: %d", got)
	}
	long := `{"tiles":4,"own":{` + strings.Repeat(`"1":1,`, 20000) + `"2":2}}`
	for _, body := range []string{"", "[]", "4", `{"tiles":"4"}`, `{"tiles":4,"cpus":2}`, `{"tiles":-1}`, `{"memory":-1}`, `{"copies":-1}`, `{"disk":-1}`,
		`{"own":{"x":1}}`, `{"own":{"-1":1}}`, `{"own":{"` + itoa(maxTiles) + `":1}}`, `{"own":{"1":-5}}`, `{"own":{"1":"5"}}`, `{"tiles":4.5}`, long} {
		if got := ask(t, s, "POST", "/host", body); got != http.StatusBadRequest {
			t.Errorf("POST /host %.60q: %d, want %d", body, got, http.StatusBadRequest)
		}
	}
	if d := stateOf(t, s); d.Host == nil || d.Host.Tiles != 4 || d.Host.Disk != 3 || d.Tiles[0].Own != 5 {
		t.Errorf("a refused one changed what was kept: %+v %+v", d.Host, d.Tiles[0])
	}
}

// A clone's place may be told up to the wall's last place, and the wall
// grows to it; the place after that is refused.
func TestAClonesPlaceIsTheWallsUpToItsLast(t *testing.T) {
	s := fourTiles()
	if got := ask(t, s, "POST", "/did/"+itoa(maxTiles-1), "a clone of tile-01"); got != http.StatusNoContent || len(s.tiles) != maxTiles {
		t.Fatalf("the last place: %d, %d places", got, len(s.tiles))
	}
	if got := ask(t, s, "POST", "/did/"+itoa(maxTiles), "a clone of tile-01"); got != http.StatusNotFound || len(s.tiles) != maxTiles {
		t.Errorf("past the last place: %d, %d places", got, len(s.tiles))
	}
}

// serve runs one request to the wall's routes in the background; its
// answer comes on the channel when it ends.
func serve(s *wall, r *http.Request) chan *httptest.ResponseRecorder {
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		done <- w
	}()
	return done
}

// A tile draws only while somebody watches: its ask for a frame is
// answered at once from the wall's start and while a page asks how the
// wall is; with nobody since, it waits, and a page asking (or loading)
// lets it, and every other tile waiting, go at once with a frame; with
// nobody at all it is answered 204 once the hold is over (nothing to
// draw: ask again), not before; a tile that gives up waiting is told
// nothing. Forty tiles drew for nobody and kept the host warm (a vmx
// host of 24 threads, 2026-11-17).
func TestATileDrawsOnlyWhileSomebodyWatches(t *testing.T) {
	s := newWall(2, 1, 16, 16, time.Now())
	s.viewFor, s.holdFor = 80*time.Millisecond, 400*time.Millisecond
	get := func(path string) *http.Request { return httptest.NewRequest("GET", path, nil) }
	start := time.Now()
	if w := <-serve(s, get("/frame")); w.Code != http.StatusOK || time.Since(start) > 50*time.Millisecond {
		t.Fatalf("from the wall's start: %d after %s", w.Code, time.Since(start))
	}
	time.Sleep(120 * time.Millisecond)
	held := serve(s, get("/frame"))
	select {
	case w := <-held:
		t.Fatalf("nobody watching: answered %d at once", w.Code)
	case <-time.After(150 * time.Millisecond):
	}
	opened := time.Now()
	if c := ask(t, s, "GET", "/state", ""); c != http.StatusOK {
		t.Fatalf("state: %d", c)
	}
	select {
	case w := <-held:
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"cols":2`) {
			t.Fatalf("let go: %d %s", w.Code, w.Body.String())
		}
		if d := time.Since(opened); d > 100*time.Millisecond {
			t.Errorf("let go %s after the page asked", d)
		}
	case <-time.After(time.Second):
		t.Fatal("a page asked, and the tile still waits")
	}
	if w := <-serve(s, get("/frame")); w.Code != http.StatusOK {
		t.Fatalf("watched: %d", w.Code)
	}
	time.Sleep(120 * time.Millisecond)
	start = time.Now()
	if w := <-serve(s, get("/frame")); w.Code != http.StatusNoContent || time.Since(start) < 350*time.Millisecond || time.Since(start) > 900*time.Millisecond {
		t.Errorf("nobody at all: %d after %s; want 204 once the hold is over", w.Code, time.Since(start))
	}
	time.Sleep(120 * time.Millisecond)
	held = serve(s, get("/frame"))
	time.Sleep(50 * time.Millisecond)
	if c := ask(t, s, "GET", "/", ""); c != http.StatusOK {
		t.Fatalf("the page: %d", c)
	}
	select {
	case w := <-held:
		if w.Code != http.StatusOK {
			t.Errorf("let go by the page loading: %d", w.Code)
		}
	case <-time.After(time.Second):
		t.Error("the page was loaded, and the tile still waits")
	}
	time.Sleep(120 * time.Millisecond)
	a, b := serve(s, get("/frame")), serve(s, get("/frame"))
	time.Sleep(50 * time.Millisecond)
	ask(t, s, "GET", "/state", "")
	for _, ch := range []chan *httptest.ResponseRecorder{a, b} {
		select {
		case w := <-ch:
			if w.Code != http.StatusOK {
				t.Errorf("one of two tiles: %d", w.Code)
			}
		case <-time.After(time.Second):
			t.Error("one of two tiles still waits")
		}
	}
	time.Sleep(120 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	gone := serve(s, get("/frame").WithContext(ctx))
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case w := <-gone:
		if w.Body.Len() != 0 || w.Header().Get("Content-Type") != "" {
			t.Errorf("a tile that gave up was told %q", w.Body.String())
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("a tile that gave up is still held")
	}
}

// A tile says how long its microVM took to start its program, as its
// boot's first frame says it (X-Up-Ms); the frames after it say the same
// and are not taken for a new boot's; a new boot says its own; a number
// that is none, below zero, past ten minutes or not a number is shown as
// none; and the page is told how long ago that boot first drew.
func TestATileSaysHowLongItsMachineTookToStart(t *testing.T) {
	s := newWall(2, 1, 16, 16, time.Now())
	post := func(boot, up string) {
		t.Helper()
		h := []string{"X-Boot", boot, "X-Host", "tile-00"}
		if up != "-" {
			h = append(h, "X-Up-Ms", up)
		}
		if c := ask(t, s, "POST", "/tile/0", "\x89PNG\r\n\x1a\nxx", h...); c != http.StatusNoContent {
			t.Fatalf("a frame: %d", c)
		}
	}
	upOf := func() (int64, int64) {
		t.Helper()
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/state", nil))
		var d struct {
			Tiles []struct {
				Up    int64 `json:"up_ms"`
				UpAge int64 `json:"up_age_ms"`
			} `json:"tiles"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		return d.Tiles[0].Up, d.Tiles[0].UpAge
	}
	post("a", "612")
	if up, age := upOf(); up != 612 || age < 1 || age > 1000 {
		t.Errorf("the first boot: up %d, %d ms ago; want 612, just now", up, age)
	}
	post("a", "999")
	if up, _ := upOf(); up != 612 {
		t.Errorf("the same boot again: up %d, want its first frame's 612", up)
	}
	post("b", "731")
	if up, _ := upOf(); up != 731 {
		t.Errorf("a new boot: up %d, want 731", up)
	}
	for _, bad := range []string{"-", "", "-5", "600001", "x", "1e3"} {
		post("c"+bad, bad)
		if up, age := upOf(); up != 0 || age != 0 {
			t.Errorf("X-Up-Ms %q: up %d, age %d; want none", bad, up, age)
		}
	}
	post("d", "600000")
	if up, _ := upOf(); up != 600000 {
		t.Errorf("ten minutes: up %d", up)
	}
}

// Kill is a button's verb like the others: asked of the keeper for each
// tile chosen, in its turn, and on the page; the page has its button.
func TestKillIsAskedOfTheKeeperAsAnyVerbIs(t *testing.T) {
	s := fourTiles()
	if c := ask(t, s, "POST", "/do/kill", "[0,2]"); c != http.StatusNoContent {
		t.Fatalf("kill: %d", c)
	}
	a := askedOf(t, s)
	if len(a.Acts) != 2 || a.Acts[0].Tile != 0 || a.Acts[0].Do != "kill" || a.Acts[1].Tile != 2 || a.Acts[1].Do != "kill" {
		t.Fatalf("kill for the tiles chosen: %+v", a.Acts)
	}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(w.Body.String(), `<button data-do="kill">Kill</button>`) || !strings.Contains(w.Body.String(), "kill:'killing it'") {
		t.Fatal("the page has no Kill button, or no word for a tile being killed")
	}
}

// The keeper is told whether somebody watches: from the wall's start, and
// while a page asks how the wall is; not once nobody has for a while.
func TestTheKeeperIsToldWhetherSomebodyWatches(t *testing.T) {
	s := newWall(2, 1, 16, 16, time.Now())
	s.viewFor = 60 * time.Millisecond
	watched := func() bool {
		t.Helper()
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/asked", nil))
		var d struct {
			Watched *bool `json:"watched"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil || d.Watched == nil {
			t.Fatalf("asked: %v %s", err, w.Body.String())
		}
		return *d.Watched
	}
	if !watched() {
		t.Error("from the wall's start: not watched")
	}
	time.Sleep(100 * time.Millisecond)
	if watched() {
		t.Error("nobody for a while: watched")
	}
	ask(t, s, "GET", "/state", "")
	if !watched() {
		t.Error("a page asked: not watched")
	}
}
