// Command scene-agent is a tile of the demoscene wall: a small daemon that
// asks the wall for the frame's time, draws its own part of a Mandelbrot
// zoom in a cycling palette, sends the picture back, and goes again. Each
// frame deeper in the zoom costs more: the wall shows real work. It is
// told its place by TILE (0 and up) and where the wall is by WALL
// (http://HOST:PORT), at most FPS frames a second (8 unless set: forty
// tiles uncapped sent a hundred frames a second each on one host).
// Part of the vmx examples (MIT, see examples/LICENSE).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
)

// frame is what the wall hands out: the time to draw, and the wall's shape.
type frame struct {
	T    float64 `json:"t"`
	Cols int     `json:"cols"`
	Rows int     `json:"rows"`
	W    int     `json:"w"`
	H    int     `json:"h"`
}

func ask(c *http.Client, wall string) (frame, error) {
	var f frame
	resp, err := c.Get(wall + "/frame")
	if err != nil {
		return f, err
	}
	defer resp.Body.Close()
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

func wave(p, o float64) uint8 {
	return uint8(127.5 + 127.5*math.Sin(2*math.Pi*(p+o)))
}

// render draws tile's part of the whole wall at time f.T: a zoom into the
// seahorse valley that starts over every 48 seconds.
func render(tile int, f frame) *image.RGBA {
	col, row := tile%f.Cols, tile/f.Cols
	ww, wh := float64(f.Cols*f.W), float64(f.Rows*f.H)
	loop := math.Mod(f.T, 48)
	scale := 3.2 * math.Exp(-0.28*loop) / wh
	maxIt := 120 + int(40*loop)
	const cx, cy = -0.743643887037151, 0.131825904205330
	img := image.NewRGBA(image.Rect(0, 0, f.W, f.H))
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			cr := cx + (float64(col*f.W+x)-ww/2)*scale
			ci := cy + (float64(row*f.H+y)-wh/2)*scale
			zr, zi, n := 0.0, 0.0, 0
			for ; n < maxIt && zr*zr+zi*zi < 256; n++ {
				zr, zi = zr*zr-zi*zi+cr, 2*zr*zi+ci
			}
			if n == maxIt {
				img.SetRGBA(x, y, color.RGBA{A: 255})
				continue
			}
			mu := float64(n) + 1 - math.Log2(math.Log2(math.Sqrt(zr*zr+zi*zi)))
			p := mu*0.035 + f.T*0.2
			img.SetRGBA(x, y, color.RGBA{wave(p, 0), wave(p, 0.33), wave(p, 0.67), 255})
		}
	}
	return img
}

func send(c *http.Client, wall string, tile int, host string, ms int64, img *image.RGBA) error {
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
	req.Header.Set("X-Render-Ms", strconv.FormatInt(ms, 10))
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
	host, err := os.Hostname()
	if err != nil {
		host = "tile-" + strconv.Itoa(tile)
	}
	log.Printf("tile %d (%s): drawing for %s", tile, host, wall)
	c := &http.Client{Timeout: 5 * time.Second}
	frames, spent, since := 0, time.Duration(0), time.Now()
	for {
		f, err := ask(c, wall)
		if err == nil && tile >= f.Cols*f.Rows {
			err = fmt.Errorf("the wall has %d tiles, and this is tile %d", f.Cols*f.Rows, tile)
		}
		if err != nil {
			log.Printf("tile %d: %v; again in a second", tile, err)
			time.Sleep(time.Second)
			continue
		}
		t0 := time.Now()
		img := render(tile, f)
		took := time.Since(t0)
		if err := send(c, wall, tile, host, took.Milliseconds(), img); err != nil {
			log.Printf("tile %d: sending the frame: %v; again in a second", tile, err)
			time.Sleep(time.Second)
			continue
		}
		frames, spent = frames+1, spent+took
		if rest := time.Second/time.Duration(fps) - time.Since(t0); rest > 0 {
			time.Sleep(rest)
		}
		if time.Since(since) >= 10*time.Second {
			log.Printf("tile %d: %d frames in %.0f s, %d ms a frame to draw", tile, frames, time.Since(since).Seconds(), spent.Milliseconds()/int64(frames))
			frames, spent, since = 0, 0, time.Now()
		}
	}
}
