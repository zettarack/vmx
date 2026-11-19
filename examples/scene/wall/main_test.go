package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
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
		At   string `json:"at"`
	} `json:"acts"`
	Reset    bool `json:"reset"`
	Protocol int  `json:"protocol"`
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
		Vol    *int64 `json:"vol_frames"`
		Look   string `json:"look"`
		Own    int64  `json:"own"`
	} `json:"tiles"`
	Host *struct {
		Tiles   int   `json:"tiles"`
		Memory  int64 `json:"memory"`
		Copies  int64 `json:"copies"`
		Disk    int64 `json:"disk"`
		Volumes int64 `json:"volumes"`
		Age     int64 `json:"age_ms"`
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
	if c := ask(t, s, "POST", "/do/restore", "[3]"); c != http.StatusNoContent {
		t.Fatalf("restore: %d", c)
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
	if len(a.Acts) != 2 || a.Acts[1].Tile != 3 || a.Acts[1].Do != "restore" {
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
	for _, v := range []string{"restore", "snapshot", "restore", "snapshot"} {
		ask(t, s, "POST", "/do/"+v, "[2]")
	}
	if q := s.tiles[2].queue; len(q) != queueMax || verbsOf(q) != "snapshot restore snapshot restore" {
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

// An update is a new machine in the place, as a rebuild is: waited for from
// the press, a new boot after it counted as one that took the place over,
// and a failed one waits no more.
func TestAnUpdateIsANewMachineInThePlace(t *testing.T) {
	s := fourTiles()
	frame(t, s, 3, "a")
	if c := ask(t, s, "POST", "/do/update", "[3]"); c != http.StatusNoContent {
		t.Fatalf("POST /do/update: %d", c)
	}
	if d := stateOf(t, s); !d.Tiles[3].Asked || d.Tiles[3].Act != "update" {
		t.Fatalf("an update is asked at the press: %+v", d.Tiles[3])
	}
	s.tiles[3].asked = time.Now().Add(-2 * time.Second)
	ask(t, s, "POST", "/acting/3", "")
	ask(t, s, "POST", "/did/3", "")
	frame(t, s, 3, "b")
	d := stateOf(t, s)
	if o := d.Tiles[3]; o.Asked || o.Gen != 2 || o.Took < 2000 || d.Replaced != 1 {
		t.Fatalf("the new version has the place: %+v, replaced %d", o, d.Replaced)
	}
	ask(t, s, "POST", "/do/update", "[3]")
	ask(t, s, "POST", "/acting/3", "")
	ask(t, s, "POST", "/did/3", "not updated: a clone stands on its disk", "X-Failed", "1")
	if o := stateOf(t, s).Tiles[3]; o.Asked || o.Said != "not updated: a clone stands on its disk" {
		t.Fatalf("a failed update says why and waits no more: %+v", o)
	}
}

// Every answer to the keeper's question says the wall's protocol, 2, which
// the keeper holds against its own before it does anything asked: a
// keeper of 2 met an older wall's restore, asked as rollback, as a roll
// back to the first version.
func TestTheKeepersQuestionSaysTheWallsProtocol(t *testing.T) {
	s := fourTiles()
	if p := askedOf(t, s).Protocol; p != 2 {
		t.Fatalf("the protocol: %d, want 2", p)
	}
	ask(t, s, "POST", "/do/restore", "[1]")
	if a := askedOf(t, s); a.Protocol != 2 || len(a.Acts) != 1 || a.Acts[0].Do != "restore" {
		t.Fatalf("with a verb asked: %+v", a)
	}
}

// A roll back is a new machine in the place, as an update is: asked at
// the press (the old picture grey until the new one draws), a new boot
// after the keeper says it is done takes the place over, and one refused
// says why and waits no more. It is not a restore: the same machine
// started again on its snapshot is not a new one (TestARestoreIs...).
func TestARollBackIsANewMachineInThePlace(t *testing.T) {
	s := fourTiles()
	frame(t, s, 2, "a")
	if c := ask(t, s, "POST", "/do/rollback", "[2]"); c != http.StatusNoContent {
		t.Fatalf("POST /do/rollback: %d", c)
	}
	if d := stateOf(t, s); !d.Tiles[2].Asked || d.Tiles[2].Act != "rollback" {
		t.Fatalf("a roll back is asked at the press: %+v", d.Tiles[2])
	}
	if a := askedOf(t, s); len(a.Acts) != 1 || a.Acts[0].Do != "rollback" || a.Acts[0].Tile != 2 {
		t.Fatalf("the keeper is asked to roll it back: %+v", a.Acts)
	}
	s.tiles[2].asked = time.Now().Add(-2 * time.Second)
	ask(t, s, "POST", "/acting/2", "")
	ask(t, s, "POST", "/did/2", "back to scene-agent:7 in 1.3 s")
	frame(t, s, 2, "b")
	d := stateOf(t, s)
	if o := d.Tiles[2]; o.Asked || o.Gen != 2 || o.Took < 2000 || d.Replaced != 1 {
		t.Fatalf("the first version has the place again: %+v, replaced %d", o, d.Replaced)
	}
	ask(t, s, "POST", "/do/rollback", "[2]")
	ask(t, s, "POST", "/acting/2", "")
	ask(t, s, "POST", "/did/2", "nothing to roll back to: it runs scene-agent:7", "X-Failed", "1")
	if o := stateOf(t, s).Tiles[2]; o.Asked || o.Said != "nothing to roll back to: it runs scene-agent:7" {
		t.Fatalf("a refused roll back says why and waits no more: %+v", o)
	}
}

// The look a tile drew in is given as it said it: a word of letters, at
// most 16; anything else is none, and none said is none.
func TestATilesLookIsGivenAsItSaidIt(t *testing.T) {
	s := fourTiles()
	for _, c := range []struct{ look, want string }{
		{"blue", "blue"}, {"green", "green"}, {"", ""}, {"Green", ""}, {"green1", ""},
		{"blue green", ""}, {"abcdefghijklmnopq", ""}, {"abcdefghijklmnop", "abcdefghijklmnop"}, {"\u00e9", ""},
	} {
		if code := ask(t, s, "POST", "/tile/0", "\x89PNG\r\n\x1a\nxx", "X-Boot", "a", "X-Host", "tile-0", "X-Look", c.look); code != http.StatusNoContent {
			t.Fatalf("a frame: %d", code)
		}
		if got := stateOf(t, s).Tiles[0].Look; got != c.want {
			t.Errorf("X-Look %q: the state gives %q, want %q", c.look, got, c.want)
		}
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
	ask(t, s, "POST", "/do/restore", "[0]")
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
	ask(t, s, "POST", "/do/restore", "[0]")
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
	ask(t, s, "POST", "/do/restore", "[0]")
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
	if q := s.tiles[1].queue; len(q) != 2 || q[1] != (press{do: "clone", n: 5}) {
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
	body := `{"tiles":4,"memory":402653184,"copies":159383552,"disk":38076416,"volumes":70254592,"own":{"0":8192,"2":12288,"700":4096}}`
	if got := ask(t, s, "POST", "/host", body); got != http.StatusNoContent {
		t.Fatalf("POST /host: %d", got)
	}
	d := stateOf(t, s)
	if h := d.Host; h == nil || h.Tiles != 4 || h.Memory != 402653184 || h.Copies != 159383552 || h.Disk != 38076416 || h.Volumes != 70254592 || h.Age < 0 || h.Age > 1000 {
		t.Fatalf("the host as said: %+v", h)
	}
	if d.Tiles[0].Own != 8192 || d.Tiles[1].Own != 0 || d.Tiles[2].Own != 12288 || len(s.tiles) != 4 {
		t.Errorf("each tile's own, and no place grown: %+v, %d places", d.Tiles, len(s.tiles))
	}
	ask(t, s, "POST", "/host", `{"tiles":3,"memory":1,"copies":2,"disk":3,"own":{"1":512}}`)
	if d := stateOf(t, s); d.Tiles[0].Own != 0 || d.Tiles[1].Own != 512 || d.Host.Tiles != 3 || d.Host.Volumes != 0 {
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
	for _, body := range []string{"", "[]", "4", `{"tiles":"4"}`, `{"tiles":4,"cpus":2}`, `{"tiles":-1}`, `{"memory":-1}`, `{"copies":-1}`, `{"disk":-1}`, `{"volumes":-1}`, `{"volumes":"1"}`,
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

// The page's info says what the wall does now: that the tiles draw only
// while somebody watches, every button the tools bar has (Kill among
// them), the tile's boot time, the show, the two counts and what each
// button does to them, the numbers above the wall, the two versions'
// colours and that Update is a rolling update, and that a new machine's
// snapshots go with the old one.
func TestTheInfoSaysWhatTheWallDoes(t *testing.T) {
	w := httptest.NewRecorder()
	newWall(8, 5, 16, 16, time.Now()).routes().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	page := w.Body.String()
	for _, said := range []string{
		"<h2>When nobody watches</h2>", "the picture moves within a frame",
		"<li><b>Snapshot</b>", "<li><b>Restore</b>", "a press on one puts the tile back to that one", "<li><b>Rebuild</b>", "<li><b>Update</b>", "<li><b>Kill</b>", "<li><b>Clone</b>", "<li><b>Remove</b>", "<li><b>Reset the wall</b>",
		"how long its microVM took from its kernel starting to its program running", "<code>05-demoscene-wall.sh --show</code>",
		"counts the frames it has drawn twice: on its <b>disk</b>", "its <b>volume</b>, a ZFS volume of its own that outlives the machine",
		"the disk's count starts again from nothing and the volume's goes on", "both counts in its corner go back",
		"a tile rebuilt, updated or rolled back has only the snapshots taken since",
		"<li><b>Roll back</b>", "It changes the program and keeps the data", "Restore is the other way back",
		// The snapshot list's buttons restore, by the restore verb.
		"fetch('/do/restore?at='+encodeURIComponent(r.name)",
		"the new version of its program, the wall's other image: blue to green", "A tile on the new version already says so", "This is a rolling update", "A blue/green deployment proper",
		"<li><b>live</b>", "<li><b>frames a second</b>", "<li><b>the clock</b>", "<li><b>replaced</b>", "<li><b>blue</b> and <b>green</b>",
		"turns red when it holds anything but a whole number from 1 to 1024",
	} {
		if !strings.Contains(page, said) {
			t.Errorf("the info does not say %q", said)
		}
	}
	// Every button of the tools bar is in the info's list.
	for _, b := range []string{"snapshot", "restore", "rebuild", "update", "rollback", "kill", "clone", "remove"} {
		if !strings.Contains(page, `data-do="`+b+`"`) {
			t.Errorf("the tools bar has no %s button", b)
		}
	}
	// The table of the counts has a row for every button and the reset.
	for _, b := range []string{"Snapshot", "Restore", "Rebuild, Update, Roll back", "Kill", "Clone", "Remove", "Reset the wall"} {
		if !strings.Contains(page, "<tr><td>"+b+"</td>") {
			t.Errorf("the counts' table has no row for %s", b)
		}
	}
}

// The info's legend shows a tile in every state the page can put one in,
// each with the classes the page gives a real tile (so it looks as one
// does): drawing in each version's colours, chosen, busy, a new machine
// coming, new, said, up, not drawing and no picture yet; every word the
// page says of a verb in hand is in the legend; and every picture in it
// has its look's swatch.
func TestTheInfosLegendShowsEveryStateOfATile(t *testing.T) {
	w := httptest.NewRecorder()
	newWall(8, 5, 16, 16, time.Now()).routes().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	page := w.Body.String()
	start, end := strings.Index(page, `<div class="lg">`), strings.Index(page, "<h2>The two counts in the corner</h2>")
	if start < 0 || end < start {
		t.Fatalf("no legend in the info: %d, %d", start, end)
	}
	legend := page[start:end]
	for _, c := range []string{
		`<div class="t"><img alt="" data-look="blue">`, `<div class="t"><img alt="" data-look="green">`,
		`<div class="t sel">`, `<div class="t busy">`, `<div class="t asked busy">`, `<div class="t fresh">`,
		`<div class="t said">`, `<div class="t dead">`, `<div class="t none"><img alt=""><span>`,
		"up in 0.62 s", "new in 2.3 s", "a new one is coming", "+1", "disk 1,204&#10;volume 3,388",
	} {
		if !strings.Contains(legend, c) {
			t.Errorf("the legend has no %s", c)
		}
	}
	// The page's own words for a verb in hand (its doing map) are each in
	// the legend: a word added there and not here is caught.
	i := strings.Index(page, "const doing={")
	j := strings.Index(page[i:], "};")
	if i < 0 || j < 0 {
		t.Fatal("no doing map in the page's script")
	}
	words := regexp.MustCompile(`:'([^']+)'`).FindAllStringSubmatch(page[i:i+j], -1)
	if len(words) != 8 {
		t.Fatalf("the doing map has %d words, want one for each of the 8 verbs", len(words))
	}
	for _, m := range words {
		if !strings.Contains(legend, m[1]) {
			t.Errorf("the legend does not say %q", m[1])
		}
	}
	// Each look a legend tile draws in has a swatch, and each swatch is
	// one of the looks the tiles draw in.
	for _, m := range regexp.MustCompile(`data-look="([a-z]+)"`).FindAllStringSubmatch(legend, -1) {
		if !strings.Contains(page, m[1]+":['000000'") {
			t.Errorf("the legend's %s tile has no swatch", m[1])
		}
	}
}

// A tile with a volume sends its count beside its disk's (X-Volume-Frames),
// and the state gives both; one that sends none, or what is not a count
// (empty, a word, a negative, past int64), has none; a tile that has not
// drawn has none; the next frame's count replaces the last; and the
// page shows the two counts, each named, only when there is a volume's.
func TestATilesVolumeCountIsGivenBesideItsDisks(t *testing.T) {
	s := fourTiles()
	send := func(i int, vol string) {
		t.Helper()
		h := []string{"X-Boot", "b" + itoa(i), "X-Host", "tile-" + itoa(i), "X-Frames", "12"}
		if vol != "none" {
			h = append(h, "X-Volume-Frames", vol)
		}
		if c := ask(t, s, "POST", "/tile/"+itoa(i), "\x89PNG\r\n\x1a\nxx", h...); c != http.StatusNoContent {
			t.Fatalf("a frame for %d: %d", i, c)
		}
	}
	send(0, "345")
	send(1, "none")
	send(2, "-3")
	d := stateOf(t, s)
	if v := d.Tiles[0].Vol; v == nil || *v != 345 || d.Tiles[0].Frames != 12 {
		t.Errorf("a tile with a volume: frames %d, volume %v", d.Tiles[0].Frames, v)
	}
	for _, i := range []int{1, 2, 3} {
		if v := d.Tiles[i].Vol; v != nil {
			t.Errorf("tile %d has a volume's count, %d", i, *v)
		}
	}
	for _, bad := range []string{"", "many", "9223372036854775808", "1e3", " 7"} {
		send(1, bad)
		if v := stateOf(t, s).Tiles[1].Vol; v != nil {
			t.Errorf("%q was taken for a count: %d", bad, *v)
		}
	}
	send(0, "346")
	send(0, "346")
	if v := stateOf(t, s).Tiles[0].Vol; v == nil || *v != 346 {
		t.Errorf("the next count: %v", v)
	}
	// Back to none: a tile made again with no volume.
	send(0, "none")
	if v := stateOf(t, s).Tiles[0].Vol; v != nil {
		t.Errorf("a tile with no volume now still has one's count: %d", *v)
	}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(w.Body.String(), `t.vol_frames!==undefined?'disk '+t.frames.toLocaleString('en-US')+'\nvolume '+t.vol_frames.toLocaleString('en-US')`) {
		t.Error("the page does not show the two counts, each named")
	}
}

// A restore can go back to a snapshot named (at), which the keeper is told;
// a name that is none a restore may go back to, more than one, or one
// given to another verb is refused, and nothing is kept of it.
func TestARestoreGoesBackToTheSnapshotNamed(t *testing.T) {
	s := fourTiles()
	frame(t, s, 2, "a")
	if c := ask(t, s, "POST", "/do/restore?at=20261118T052303Z", "[2]"); c != http.StatusNoContent {
		t.Fatalf("a restore to a snapshot named: %d", c)
	}
	if a := askedOf(t, s); len(a.Acts) != 1 || a.Acts[0].Do != "restore" || a.Acts[0].At != "20261118T052303Z" {
		t.Fatalf("the keeper is told the snapshot: %+v", a)
	}
	// The newest, as before, when none is named; one named again is the
	// same press, one named otherwise another.
	s = fourTiles()
	ask(t, s, "POST", "/do/restore", "[1]")
	ask(t, s, "POST", "/do/restore?at=b", "[1]")
	ask(t, s, "POST", "/do/restore?at=b", "[1]")
	if q := s.tiles[1].queue; len(q) != 2 || q[0] != (press{do: "restore"}) || q[1] != (press{do: "restore", at: "b"}) {
		t.Fatalf("the queue: %+v", q)
	}
	s = fourTiles()
	long := strings.Repeat("a", 65)
	for _, path := range []string{"/do/restore?at=", "/do/restore?at=clone-tile-40", "/do/restore?at=-x", "/do/restore?at=a%20b",
		"/do/restore?at=" + long, "/do/restore?at=a&at=b", "/do/restore?at=golden", "/do/restore?at=%C3%A9", "/do/snapshot?at=a", "/do/clone?at=a", "/do/rollback?at=a", "/do/update?at=a"} {
		if c := ask(t, s, "POST", path, "[0]"); c != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", path, c)
		}
	}
	if c := ask(t, s, "POST", "/do/restore?at="+long[:64], "[0]"); c != http.StatusNoContent {
		t.Errorf("a name of 64: %d", c)
	}
	if q := s.tiles[0].queue; len(q) != 1 {
		t.Errorf("refusals were kept: %+v", q)
	}
}

// A tile's snapshots are listed when a page asks: the keeper is asked (in
// /asked) while the ask stands, and the list it gives is what the page is
// answered, with its age; what it gives is checked before any is kept.
func TestATilesSnapshotsAreListedWhenAPageAsks(t *testing.T) {
	s := fourTiles()
	get := func(i int) (map[string]any, int) {
		t.Helper()
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/snaps/"+itoa(i), nil))
		var d map[string]any
		if w.Code == http.StatusOK {
			if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
				t.Fatal(err)
			}
		}
		return d, w.Code
	}
	if d, c := get(1); c != http.StatusOK || d["listed"] != false {
		t.Fatalf("before the keeper has listed them: %d %v", c, d)
	}
	if _, c := get(9); c != http.StatusNotFound {
		t.Errorf("a tile the wall has not: %d", c)
	}
	var a struct {
		Snaps []int `json:"snaps"`
	}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/asked", nil))
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil || len(a.Snaps) != 1 || a.Snaps[0] != 1 {
		t.Fatalf("the keeper is asked for tile 1's: %v %s", err, w.Body.String())
	}
	good := `[{"name":"20261118T052303Z","created":"2026-11-18T05:23:03Z","used_bytes":12288},{"name":"before","created":"2026-11-18T05:30:00Z","used_bytes":0}]`
	if c := ask(t, s, "POST", "/snaps/1", good); c != http.StatusNoContent {
		t.Fatalf("POST /snaps/1: %d", c)
	}
	d, _ := get(1)
	rows, _ := d["snapshots"].([]any)
	if d["listed"] != true || len(rows) != 2 || rows[0].(map[string]any)["name"] != "20261118T052303Z" || d["age_ms"] == nil {
		t.Fatalf("the list as the keeper gave it: %v", d)
	}
	// The ask lapses: nothing asked of the keeper after wantFor.
	s.mu.Lock()
	s.tiles[1].wanted = time.Now().Add(-wantFor - time.Second)
	s.mu.Unlock()
	w = httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/asked", nil))
	a.Snaps = nil
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil || len(a.Snaps) != 0 {
		t.Errorf("an ask past wantFor still stands: %v %s", err, w.Body.String())
	}
	many := "[" + strings.TrimSuffix(strings.Repeat(`{"name":"a","created":"2026-11-18T05:23:03Z","used_bytes":1},`, snapsMax+1), ",") + "]"
	for _, body := range []string{"", "{}", "[1]", `[{"name":"a","created":"2026-11-18T05:23:03Z","used_bytes":1,"x":1}]`, many,
		`[{"name":"clone-tile-40","created":"2026-11-18T05:23:03Z","used_bytes":1}]`, `[{"name":"a b","created":"2026-11-18T05:23:03Z","used_bytes":1}]`,
		`[{"name":"a","used_bytes":1}]`, `[{"name":"a","created":"2026-11-18T05:23:03Z","used_bytes":-1}]`, `[{"name":"a","created":"yesterday","used_bytes":1}]`} {
		if c := ask(t, s, "POST", "/snaps/1", body); c != http.StatusBadRequest {
			t.Errorf("%.60s: %d, want 400", body, c)
		}
	}
	if c := ask(t, s, "POST", "/snaps/9", good); c != http.StatusNotFound {
		t.Errorf("a tile the wall has not: %d", c)
	}
	if d, _ := get(1); len(d["snapshots"].([]any)) != 2 {
		t.Errorf("a refused list replaced the one kept: %v", d)
	}
	if c := ask(t, s, "POST", "/snaps/1", "[]"); c != http.StatusNoContent {
		t.Errorf("none: %d", c)
	}
	if d, _ := get(1); d["listed"] != true || len(d["snapshots"].([]any)) != 0 {
		t.Errorf("none listed: %v", d)
	}
}

type showDoc struct {
	Part     int64    `json:"part"`
	Family   string   `json:"family"`
	In       float64  `json:"in_s"`
	Held     bool     `json:"held"`
	Families []string `json:"families"`
}

func showOf(t *testing.T, s *wall) showDoc {
	t.Helper()
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/state", nil))
	var d struct {
		Show showDoc `json:"show"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatalf("state: %v: %s", err, w.Body.String())
	}
	return d.Show
}

type frameDoc struct {
	T    float64 `json:"t"`
	Show float64 `json:"show"`
	Cut  int64   `json:"cut_part"`
}

func frameOf(t *testing.T, s *wall) frameDoc {
	t.Helper()
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/frame", nil))
	var f frameDoc
	if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil {
		t.Fatalf("frame: %v: %s", err, w.Body.String())
	}
	return f
}

// The page's controls move the show's clock, and the tiles are told it
// with each frame: next to the next part's start, told as a cut; before,
// within a part's first four seconds, to the one before it, and later in
// a part to its own start; hold holds the clock and runs it on again; a
// family's name to the next part that plays it, for every family; and a
// word that is none of these is refused with nothing changed.
func TestTheShowsControlsMoveItsClockAndTheTilesAreTold(t *testing.T) {
	s := fourTiles()
	if sh := showOf(t, s); sh.Part != 0 || sh.Family != "vmx" || sh.Held || len(sh.Families) != 10 || sh.Families[0] != "vmx" {
		t.Fatalf("a new wall's show: %+v", sh)
	}
	if c := ask(t, s, "POST", "/show/next", ""); c != http.StatusNoContent {
		t.Fatalf("next: %d", c)
	}
	f := frameOf(t, s)
	if f.Show < 60 || f.Show > 61 || f.Cut != 1 {
		t.Errorf("after next the tiles draw %v, cut %d; want part 1's start, cut 1", f.Show, f.Cut)
	}
	if c := ask(t, s, "POST", "/show/before", ""); c != http.StatusNoContent {
		t.Fatalf("before: %d", c)
	}
	if sh := showOf(t, s); sh.Part != 0 {
		t.Errorf("before, at a part's start, plays part %d, want 0", sh.Part)
	}
	// Later in a part, before plays it again from its start.
	s.mu.Lock()
	s.showBase, s.showAt = 3*60+30, time.Now()
	s.mu.Unlock()
	ask(t, s, "POST", "/show/before", "")
	if sh := showOf(t, s); sh.Part != 3 || sh.In > 1 {
		t.Errorf("before, half way into part 3: part %d at %.1f s, want part 3 from its start", sh.Part, sh.In)
	}
	ask(t, s, "POST", "/show/hold", "")
	a := showOf(t, s)
	time.Sleep(60 * time.Millisecond)
	b := showOf(t, s)
	if !a.Held || a.In != b.In {
		t.Errorf("held: %+v then %+v, want the same moment", a, b)
	}
	ask(t, s, "POST", "/show/hold", "")
	time.Sleep(60 * time.Millisecond)
	if c := showOf(t, s); c.Held || c.In <= b.In {
		t.Errorf("run on: %+v, want the clock going on from %.3f", c, b.In)
	}
	for _, fam := range showOf(t, s).Families {
		before := showOf(t, s).Part
		if c := ask(t, s, "POST", "/show/"+fam, ""); c != http.StatusNoContent {
			t.Fatalf("%s: %d", fam, c)
		}
		sh := showOf(t, s)
		if sh.Family != fam || sh.Part <= before || sh.In > 1 {
			t.Errorf("%s: part %d (%s) at %.1f s, after part %d", fam, sh.Part, sh.Family, sh.In, before)
		}
		if f := frameOf(t, s); f.Cut != sh.Part {
			t.Errorf("%s: the tiles are told cut %d, want %d", fam, f.Cut, sh.Part)
		}
	}
	was := showOf(t, s)
	// A path the mux cleans first ("../next") is sent elsewhere, not done.
	for _, bad := range []string{"nope", "Bits", "next%20", "../next", "%00", "hold%2Fnext"} {
		if c := ask(t, s, "POST", "/show/"+bad, ""); c == http.StatusNoContent {
			t.Errorf("%q was done", bad)
		}
	}
	if now := showOf(t, s); now.Part != was.Part {
		t.Errorf("a refused word moved the show: part %d, was %d", now.Part, was.Part)
	}
}

// The page has the show's controls, says what they do and which keys do
// the same, and names every family.
func TestThePageHasTheShowsControlsAndSaysThem(t *testing.T) {
	w := httptest.NewRecorder()
	newWall(8, 5, 16, 16, time.Now()).routes().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	page := w.Body.String()
	for _, said := range []string{
		`<button id="before"`, `<button id="hold"`, `<button id="next"`, `<span id="fams">`,
		"<h2>The show</h2>", "<b>Before</b>", "<b>Hold</b>", "<b>Run</b>", "<b>Next</b>", "<b>Text off</b>", "<b>Text on</b>", "the left and right arrows, space, and 1 to 0",
		`<button id="text"`, "$('text').onclick=()=>showTo('text');", "else if(e.key==='t')showTo('text');",
		// Text off takes all the text away: the tiles' names and counts and
		// the lines above the wall, as well as the line in the picture.
		"body.quiet h1,body.quiet #bar,body.quiet #host,body.quiet .t span,body.quiet .t b{display:none}", "document.body.classList.toggle('quiet',sh.text===false);", "takes the page's own text away", "the words a scene writes are part of the picture and stay",
		// The names and counts fade when nobody stirs.
		"body.idle .t span,body.idle .t b{opacity:0;transition:opacity 1.2s}", "Date.now()-stirred>6000&&panel.hidden&&!sel.size", "fade six seconds after",
		// The buttons fold into cfg under the wall, closed at first.
		`<div id="panel" hidden>`, `<button id="cfg"`, "<b>cfg</b>", "else if(e.key==='c')cfg(panel.hidden);", "else if(!panel.hidden)cfg(false);",
		"fetch('/show/'+encodeURIComponent(what),{method:'POST'})", "showTick(s.show);",
	} {
		if !strings.Contains(page, said) {
			t.Errorf("the page does not have %q", said)
		}
	}
	for _, fam := range []string{"vmx", "grid", "land", "day", "space", "shapes", "Mandelbrot", "Julia", "tunnel", "plasma"} {
		if !strings.Contains(page, fam) {
			t.Errorf("the info does not name %s", fam)
		}
	}
}

// The page's Text is the page's own text alone (its title, its lines, the
// tiles' names and counts), off and on for every page open on the wall,
// as the state says: the tiles' frames carry nothing of it, since the
// words a scene writes are the picture's and stay.
func TestTextOffIsThePagesOwnTextAndTheTilesAreNotTold(t *testing.T) {
	s := fourTiles()
	frameHasQuiet := func() bool {
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/frame", nil))
		var f map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil {
			t.Fatalf("frame: %v: %s", err, w.Body.String())
		}
		_, has := f["quiet"]
		return has
	}
	text := func() bool {
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/state", nil))
		var d struct {
			Show struct {
				Text *bool `json:"text"`
			} `json:"show"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil || d.Show.Text == nil {
			t.Fatalf("state: %v: %s", err, w.Body.String())
		}
		return *d.Show.Text
	}
	if !text() {
		t.Fatal("a new wall's text is off")
	}
	before := showOf(t, s).Part
	if c := ask(t, s, "POST", "/show/text", ""); c != http.StatusNoContent {
		t.Fatalf("text: %d", c)
	}
	if text() {
		t.Error("Text off: the state says the text is on")
	}
	if frameHasQuiet() {
		t.Error("Text off is told to the tiles: the scenes' own words would go")
	}
	if showOf(t, s).Part != before {
		t.Error("Text off moved the show")
	}
	ask(t, s, "POST", "/show/text", "")
	if !text() {
		t.Error("Text on again: the state still says off")
	}
}

// A page knows the wall that served it: the page carries the wall's own
// id and the state says it, and another wall (one swapped in, or made
// again) has another, so a page left open on the old one reloads.
func TestAPageServedByAnotherWallReloads(t *testing.T) {
	a := newWall(2, 2, 16, 16, time.Now())
	b := newWall(2, 2, 16, 16, time.Now().Add(time.Millisecond))
	if a.pageID == "" || a.pageID == b.pageID {
		t.Fatalf("two walls' ids: %q and %q", a.pageID, b.pageID)
	}
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	page := w.Body.String()
	if !strings.Contains(page, "if(s.page&&s.page!=='"+a.pageID+"'){location.reload();return;}") {
		t.Error("the page does not reload for another wall's state")
	}
	if strings.Contains(page, "PAGEID") {
		t.Error("the page's id was left unreplaced")
	}
	w = httptest.NewRecorder()
	a.routes().ServeHTTP(w, httptest.NewRequest("GET", "/state", nil))
	var d struct {
		Page string `json:"page"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil || d.Page != a.pageID {
		t.Errorf("the state's page: %q (%v), want %q", d.Page, err, a.pageID)
	}
}

// The page is never kept by a browser: a kept page outlives its wall.
func TestThePageIsNeverKept(t *testing.T) {
	w := httptest.NewRecorder()
	fourTiles().routes().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: %q, want no-store", got)
	}
}
