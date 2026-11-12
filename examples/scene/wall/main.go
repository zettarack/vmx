// Command scene-wall is the demoscene wall: it hands every tile the
// frame's time, keeps each tile's newest picture, and shows them all as
// one picture in a browser, with how many tiles are live, how many frames
// a second they draw together, and how long the whole wall took to come
// up after a reset. Part of the vmx examples (MIT, see examples/LICENSE).
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
)

type tile struct {
	png  []byte
	ver  int
	at   time.Time
	host string
	ms   int
}

type wall struct {
	mu               sync.Mutex
	cols, rows, w, h int
	tiles            []tile
	epoch, start     time.Time
	frames           []time.Time
	full             time.Duration
}

const liveFor = 2 * time.Second

func (s *wall) frame(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"t": time.Since(s.epoch).Seconds(), "cols": s.cols, "rows": s.rows, "w": s.w, "h": s.h,
	}); err != nil {
		log.Printf("frame: %v", err)
	}
}

func (s *wall) index(r *http.Request) (int, bool) {
	i, err := strconv.Atoi(r.PathValue("i"))
	return i, err == nil && i >= 0 && i < len(s.tiles)
}

func (s *wall) put(w http.ResponseWriter, r *http.Request) {
	i, ok := s.index(r)
	if !ok {
		http.Error(w, "no such tile", http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil || !bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")) {
		http.Error(w, "a tile is one PNG of at most 4 MiB", http.StatusBadRequest)
		return
	}
	ms, _ := strconv.Atoi(r.Header.Get("X-Render-Ms")) // a missing number is shown as none
	host := r.Header.Get("X-Host")
	if len(host) > 40 {
		host = host[:40]
	}
	now := time.Now()
	s.mu.Lock()
	t := &s.tiles[i]
	if t.ver == 0 {
		log.Printf("tile %d is up (%s, %s)", i, host, r.RemoteAddr)
	}
	t.png, t.ver, t.at, t.host, t.ms = body, t.ver+1, now, host, ms
	s.frames = append(s.frames, now)
	for len(s.frames) > 0 && now.Sub(s.frames[0]) > liveFor {
		s.frames = s.frames[1:]
	}
	if s.full == 0 {
		all := true
		for _, o := range s.tiles {
			all = all && !o.at.Before(s.start) && o.ver > 0
		}
		if all {
			s.full = now.Sub(s.start)
			log.Printf("all %d tiles up, %.1f s after the reset", len(s.tiles), s.full.Seconds())
		}
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *wall) get(w http.ResponseWriter, r *http.Request) {
	i, ok := s.index(r)
	s.mu.Lock()
	var body []byte
	if ok {
		body = s.tiles[i].png
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

func (s *wall) state(w http.ResponseWriter, _ *http.Request) {
	type one struct {
		V    int    `json:"v"`
		Age  int64  `json:"age_ms"`
		Host string `json:"host"`
		Ms   int    `json:"ms"`
	}
	now := time.Now()
	s.mu.Lock()
	out := make([]one, len(s.tiles))
	live := 0
	for i, t := range s.tiles {
		out[i] = one{V: t.ver, Host: t.host, Ms: t.ms, Age: now.Sub(t.at).Milliseconds()}
		if t.ver > 0 && now.Sub(t.at) <= liveFor {
			live++
		}
	}
	n := 0
	for _, f := range s.frames {
		if now.Sub(f) <= liveFor {
			n++
		}
	}
	doc := map[string]any{"tiles": out, "live": live, "fps": float64(n) / liveFor.Seconds(),
		"since_s": now.Sub(s.start).Seconds(), "full_s": s.full.Seconds()}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		log.Printf("state: %v", err)
	}
}

func (s *wall) reset(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.tiles = make([]tile, s.cols*s.rows)
	s.start, s.full, s.frames = time.Now(), 0, nil
	s.mu.Unlock()
	log.Printf("reset: the stopwatch starts")
	w.WriteHeader(http.StatusNoContent)
}

func (s *wall) page(w http.ResponseWriter, _ *http.Request) {
	r := strings.NewReplacer("COLS", strconv.Itoa(s.cols), "ROWS", strconv.Itoa(s.rows), "TW", strconv.Itoa(s.w), "TH", strconv.Itoa(s.h))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := io.WriteString(w, r.Replace(page)); err != nil {
		log.Printf("page: %v", err)
	}
}

func main() {
	listen := flag.String("listen", ":8090", "where the wall answers")
	cols := flag.Int("cols", 8, "tiles across")
	rows := flag.Int("rows", 5, "tiles down")
	tw := flag.Int("w", 160, "a tile's width in pixels")
	th := flag.Int("h", 120, "a tile's height in pixels")
	flag.Parse()
	if *cols < 1 || *rows < 1 || *cols**rows > 256 || *tw < 16 || *th < 16 || *tw > 1024 || *th > 1024 {
		log.Fatal("scene-wall: 1 to 256 tiles, each 16 to 1024 pixels a side")
	}
	now := time.Now()
	s := &wall{cols: *cols, rows: *rows, w: *tw, h: *th, tiles: make([]tile, *cols**rows), epoch: now, start: now}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.page)
	mux.HandleFunc("GET /frame", s.frame)
	mux.HandleFunc("GET /state", s.state)
	mux.HandleFunc("POST /tile/{i}", s.put)
	mux.HandleFunc("GET /tile/{i}", s.get)
	mux.HandleFunc("POST /reset", s.reset)
	log.Printf("the wall: %d by %d tiles of %dx%d, on %s", *cols, *rows, *tw, *th, *listen)
	srv := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

const page = `<!doctype html>
<html><head><meta charset="utf-8"><title>Firecracker wall</title>
<style>
body{margin:0;background:#05040a;color:#cfd2ff;font:15px/1.4 ui-monospace,monospace;display:flex;flex-direction:column;align-items:center}
h1{margin:18px 0 6px;font-size:28px;letter-spacing:6px;background:linear-gradient(90deg,#ff3cac,#784ba0,#2b86c5,#3cffb4,#ffe53c,#ff3cac);background-size:300% 100%;-webkit-background-clip:text;color:transparent;animation:c 6s linear infinite}
@keyframes c{to{background-position:300% 0}}
#bar{display:flex;gap:28px;margin:4px 0 12px;font-size:17px}
#bar b{color:#fff}
#wall{display:grid;grid-template-columns:repeat(COLS,TWpx);gap:2px;background:#000;padding:2px;border:1px solid #2a2850}
.t{position:relative;width:TWpx;height:THpx;background:#0b0a16}
.t img{display:block;width:TWpx;height:THpx}
.t span{position:absolute;left:3px;bottom:2px;font-size:10px;color:#fff;text-shadow:0 0 3px #000}
.dead img{opacity:.18}
.dead span{color:#ff5c7a}
button{margin-top:12px;background:#151330;color:#cfd2ff;border:1px solid #3a3870;padding:6px 16px;font:inherit;cursor:pointer}
canvas{margin-top:10px}
</style></head><body>
<h1>FIRECRACKER WALL</h1>
<div id="bar"><span id="live"></span><span id="fps"></span><span id="clock"></span></div>
<div id="wall"></div>
<button onclick="fetch('/reset',{method:'POST'})">reset the stopwatch</button>
<canvas id="sc" width="900" height="56"></canvas>
<script>
const n=COLS*ROWS,wall=document.getElementById('wall'),tiles=[];
for(let i=0;i<n;i++){const d=document.createElement('div');d.className='t dead';const im=new Image();const l=document.createElement('span');l.textContent=String(i).padStart(2,'0');d.append(im,l);wall.append(d);tiles.push({d,im,l,v:-1});}
const $=id=>document.getElementById(id);
async function tick(){try{const s=await(await fetch('/state')).json();
s.tiles.forEach((t,i)=>{const o=tiles[i];if(t.v>0&&t.v!==o.v){o.v=t.v;o.im.src='/tile/'+i+'?v='+t.v;}
o.d.classList.toggle('dead',t.v===0||t.age_ms>2000);o.l.textContent=(t.host||String(i).padStart(2,'0'))+(t.ms?'  '+t.ms+' ms':'');});
$('live').innerHTML='<b>'+s.live+'</b> of '+n+' microVMs live';$('fps').innerHTML='<b>'+s.fps.toFixed(1)+'</b> frames a second';
$('clock').innerHTML=s.full_s>0?'all '+n+' up in <b>'+s.full_s.toFixed(1)+' s</b>':'<b>'+s.since_s.toFixed(1)+' s</b> since the reset';}catch(e){}setTimeout(tick,100);}
tick();
const sc=$('sc'),g=sc.getContext('2d'),msg='   ...  greetings from vmx  ...  every tile is a Firecracker microVM, cloned from one image in a moment  ...  pull one out and watch it come back  ...  ';
let x=0;function scroll(){g.clearRect(0,0,sc.width,sc.height);g.font='bold 26px monospace';
for(let i=0;i<msg.length;i++){const px=i*17-x;if(px<-20||px>sc.width)continue;const y=30+12*Math.sin((px+x*0.6)/45);
g.fillStyle='hsl('+((i*12+x)%360)+',90%,65%)';g.fillText(msg[i],px,y);}x=(x+2)%(msg.length*17);requestAnimationFrame(scroll);}
scroll();
</script></body></html>`
