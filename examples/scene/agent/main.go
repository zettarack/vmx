// Command scene-agent is a tile of the demoscene wall: a small daemon that
// asks the wall for the frame's time, draws its own square of the wall's
// show (package show: an hour of scenes, each worked out from the show's
// clock alone, so every tile agrees with the others without a word
// between them), sends the picture back, and goes again. A deep fractal
// or a raymarched solid costs more to draw than a plasma: the wall's
// frames a second show real work. It is
// told its place by TILE (0 and up) and where the wall is by WALL
// (http://HOST:PORT), at most FPS frames a second (8 unless set: forty
// tiles uncapped sent a hundred frames a second each on one host).
// Every frame carries a boot id made when the program starts, so the wall
// can tell a new microVM in a tile's place from the one before it.
//
// It counts the frames it has drawn on its own disk (/var/lib/scene,
// written and flushed every second) and sends the count with each frame,
// so a snapshot of its disk holds the count it had then, and a rollback
// to that snapshot is seen on the wall as the count going back. Given a
// volume (VOLUME, the directory vmx mounts it on), it counts them there
// as well: the disk is the machine's and is made again from the image
// with it, the volume outlives it, so a tile made again (Rebuild, or a
// new image rolled out) starts its disk's count from nothing and goes on
// with its volume's. A snapshot takes both at one moment and a rollback
// returns both; a clone copies both. LOOK names the colours it draws in:
// blue (also when it is not set), the scene's light from black through
// deep blue and sky to white; green, the same through green, which is what
// the wall's second image sets (ENV LOOK=green), a new version of the
// program rolled out over the tiles; classic, each scene's own colours. A host
// named tile-N (the container's name, which vmx gives the microVM) draws
// place N whatever TILE says: a clone of tile-07 named tile-40 draws a
// place of its own, below the picture's own rows.
// Part of the vmx examples (MIT, see examples/LICENSE).
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"scene/show"
)

// counted is where the count of frames drawn is kept, on the tile's own
// disk; on a volume it is the file frames in its directory.
const counted = "/var/lib/scene/frames"

// readCount is the count kept in file, 0 when there is none yet.
func readCount(file string) int64 {
	b, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// writeCount keeps n in file, flushed, so a snapshot taken a moment later
// holds it: written to a file beside it, synced, and renamed over.
func writeCount(file string, n int64) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".new"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "%d\n", n); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// mounted says whether dir is a mount point, as the kernel lists them
// (/proc/self/mounts): a volume is one, and a directory that only looks
// like it is on the machine's own disk, whose count would not outlive it.
func mounted(dir string) (bool, error) {
	b, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) > 1 && f[1] == dir {
			return true, nil
		}
	}
	return false, nil
}

// place is the tile's place: the number in a host name tile-N, else TILE.
func place(host string, tile int) int {
	if n, ok := strings.CutPrefix(host, "tile-"); ok {
		if i, err := strconv.Atoi(n); err == nil && i >= 0 && i < 256 {
			return i
		}
	}
	return tile
}

// frame is what the wall hands out: the time to draw, and the wall's
// shape. Show is the show's own clock, which the page's controls move and
// hold, and CutPart the part they last moved it to the start of; a wall
// from before the controls sends neither, and the show then runs on the
// wall's time.
type frame struct {
	T       float64  `json:"t"`
	Show    *float64 `json:"show"`
	CutPart int64    `json:"cut_part"`
	Cols    int      `json:"cols"`
	Rows    int      `json:"rows"`
	W       int      `json:"w"`
	H       int      `json:"h"`
}

// at is the moment of the show to draw.
func (f frame) at() float64 {
	if f.Show != nil {
		return *f.Show
	}
	return f.T
}

// errIdle is the wall saying nobody watches it: nothing to draw, ask again.
// It holds the ask for a few seconds first, so asking again at once is
// not a busy loop.
var errIdle = errors.New("nobody watches the wall")

func ask(c *http.Client, wall string) (frame, error) {
	var f frame
	resp, err := c.Get(wall + "/frame")
	if err != nil {
		return f, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return f, errIdle
	}
	if resp.StatusCode != http.StatusOK {
		return f, fmt.Errorf("the wall answered %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&f); err != nil {
		return f, fmt.Errorf("the wall's frame: %w", err)
	}
	if f.Cols < 1 || f.Rows < 1 || f.W < 1 || f.H < 1 || f.W > 1024 || f.H > 1024 {
		return f, fmt.Errorf("the wall's frame has no shape: %+v", f)
	}
	return f, nil
}

// render draws the tile's square of the show at the frame's moment in its
// look; sh keeps what a part made at its first frame for the frames after.
func render(sh *show.Show, tile int, f frame, look string) *image.RGBA {
	return sh.Render(tile, show.Frame{T: f.at(), Cols: f.Cols, Rows: f.Rows, W: f.W, H: f.H, CutPart: f.CutPart}, look)
}

// upMs is how long this machine had run when the tile's program started,
// as its own kernel counts it (/proc/uptime): from the microVM's kernel
// starting to the program, its address included. 0 when it cannot be
// read, and then nothing is said.
func upMs() int64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil || v < 0 {
		return 0
	}
	return int64(v * 1000)
}

// send hands the wall a frame drawn: the picture, and with it the tile's
// counts, its disk's and, when it has a volume (vcount 0 and up), its
// volume's, and the look it was drawn in.
func send(c *http.Client, wall string, tile int, host, boot, look string, up, ms, count, vcount int64, img *image.RGBA) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/tile/%d", wall, tile), &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("X-Host", host)
	req.Header.Set("X-Boot", boot)
	req.Header.Set("X-Render-Ms", strconv.FormatInt(ms, 10))
	if up > 0 {
		req.Header.Set("X-Up-Ms", strconv.FormatInt(up, 10))
	}
	req.Header.Set("X-Frames", strconv.FormatInt(count, 10))
	req.Header.Set("X-Look", look)
	if vcount >= 0 {
		req.Header.Set("X-Volume-Frames", strconv.FormatInt(vcount, 10))
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("the wall answered %s", resp.Status)
	}
	return nil
}

func main() {
	// First, before anything of the program's own: how long the machine
	// took to get here.
	up := upMs()
	log.SetFlags(0)
	wall := os.Getenv("WALL")
	tile, err := strconv.Atoi(os.Getenv("TILE"))
	if wall == "" || err != nil || tile < 0 {
		log.Fatal("scene-agent: set WALL=http://HOST:PORT and TILE=N (0 and up)")
	}
	fps := 8
	if v := os.Getenv("FPS"); v != "" {
		if fps, err = strconv.Atoi(v); err != nil || fps < 1 || fps > 60 {
			log.Fatal("scene-agent: FPS is 1 to 60")
		}
	}
	look := os.Getenv("LOOK")
	if look == "" {
		look = "blue"
	}
	if !show.Looks[look] {
		log.Fatalf("scene-agent: LOOK=%s is none of blue, green and classic", look)
	}
	host, err := os.Hostname()
	if err != nil {
		host = "tile-" + strconv.Itoa(tile)
	}
	tile = place(host, tile)
	count := readCount(counted)
	// The volume's count, -1 when the tile has no volume. One named that is
	// not mounted is a run that went wrong: said, and nothing drawn.
	vol, vcount, vfile := os.Getenv("VOLUME"), int64(-1), ""
	if vol != "" {
		if !filepath.IsAbs(vol) {
			log.Fatalf("scene-agent: VOLUME=%s is not an absolute path", vol)
		}
		on, err := mounted(vol)
		if err != nil {
			log.Fatalf("scene-agent: reading the mounts: %v", err)
		}
		if !on {
			log.Fatalf("scene-agent: VOLUME=%s is not mounted: no volume is there, and a count on the disk would not outlive it", vol)
		}
		vfile = filepath.Join(vol, "frames")
		vcount = readCount(vfile)
	}
	kept := time.Now()
	// This start's own id: random, so two microVMs of one name are two.
	// With no randomness to read, the time it started, which differs too.
	id := make([]byte, 8)
	boot := strconv.FormatInt(time.Now().UnixNano(), 36)
	if _, err := rand.Read(id); err == nil {
		boot = hex.EncodeToString(id)
	}
	onVol := ""
	if vcount >= 0 {
		onVol = fmt.Sprintf(", %d on its volume (%s)", vcount, vol)
	}
	log.Printf("tile %d (%s, boot %s, the machine up %d ms): drawing for %s in %s, %d frames drawn on this disk before%s", tile, host, boot, up, wall, look, count, onVol)
	c := &http.Client{Timeout: 5 * time.Second}
	sh := new(show.Show)
	frames, spent, since := 0, time.Duration(0), time.Now()
	idle := false
	for {
		f, err := ask(c, wall)
		if errors.Is(err, errIdle) {
			// Said once, when it starts: the wall holds each ask a few
			// seconds while nobody watches, and the tile draws nothing.
			if !idle {
				log.Printf("tile %d: nobody watches the wall: drawing nothing until a page is open", tile)
				idle = true
			}
			continue
		}
		if err == nil && idle {
			log.Printf("tile %d: a page is open: drawing again", tile)
			idle = false
		}
		if err != nil {
			log.Printf("tile %d: %v; again in a second", tile, err)
			time.Sleep(time.Second)
			continue
		}
		t0 := time.Now()
		img := render(sh, tile, f, look)
		took := time.Since(t0)
		next := vcount
		if next >= 0 {
			next++
		}
		if err := send(c, wall, tile, host, boot, look, up, took.Milliseconds(), count+1, next, img); err != nil {
			log.Printf("tile %d: sending the frame: %v; again in a second", tile, err)
			time.Sleep(time.Second)
			continue
		}
		frames, spent, count, vcount = frames+1, spent+took, count+1, next
		if time.Since(kept) >= time.Second {
			if err := writeCount(counted, count); err != nil {
				log.Printf("tile %d: keeping the count on the disk: %v", tile, err)
			}
			if vcount >= 0 {
				if err := writeCount(vfile, vcount); err != nil {
					log.Printf("tile %d: keeping the count on the volume: %v", tile, err)
				}
			}
			kept = time.Now()
		}
		if rest := time.Second/time.Duration(fps) - time.Since(t0); rest > 0 {
			time.Sleep(rest)
		}
		if time.Since(since) >= 10*time.Second {
			log.Printf("tile %d: %d frames in %.0f s, %d ms a frame to draw", tile, frames, time.Since(since).Seconds(), spent.Milliseconds()/int64(frames))
			frames, spent, since = 0, 0, time.Now()
		}
	}
}
