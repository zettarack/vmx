// Command scene-wall is the demoscene wall: it hands every tile the
// frame's time, keeps each tile's newest picture, and shows them all as
// one picture in a browser, with how many tiles are live, how many frames
// a second they draw together, and how long the whole wall took to come
// up after a reset.
//
// A click on a tile asks for its microVM to be replaced. The wall cannot
// do that: it is a program in a container, and nothing in a container
// commands the host. It keeps the request (GET /asked lists them) for the
// keeper, a loop on the host (05-demoscene-wall.sh, run as the service
// demoscene-keeper) that destroys the tile's container and runs a new one,
// and it knows the new microVM by the boot id every tile sends with its
// frames, so the page can say how long the tile was away. The page does
// not say that a tile can be clicked until its info button is pressed: it
// is there to be found, and the info says how the work is shared out, how
// a tile is repaired and how the whole is put together. Otherwise it
// speaks only when a click finds no keeper watching, so that a click that
// does nothing says why.
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
)

type tile struct {
	png  []byte
	ver  int
	at   time.Time
	host string
	ms   int
	// boot is the id its microVM sends with every frame, made when the
	// tile's program starts: another id is another microVM. born is when
	// the one drawing now sent its first frame, and gen how many have had
	// this place.
	boot string
	born time.Time
	gen  int
	// asked is when the page asked for it to be replaced (zero when it
	// has not), and taken whether the keeper has said it is doing so.
	asked, takenAt time.Time
	taken          bool
	// took is how long the place went without a picture the last time
	// its microVM was replaced: from the click, or with no click from
	// the old one's last frame, to the new one's first.
	took time.Duration
}

type wall struct {
	mu               sync.Mutex
	cols, rows, w, h int
	tiles            []tile
	epoch, start     time.Time
	frames           []time.Time
	full             time.Duration
	// replaced counts the microVMs that took over a place from another;
	// watched is when the keeper last asked what to replace.
	replaced int
	watched  time.Time
}

const (
	liveFor = 2 * time.Second
	// A keeper that has not asked for this long is not watching.
	keeperFor = 3 * time.Second
	// A request the keeper took and did not finish in this long may be
	// asked again: the keeper was stopped half way, or its run failed.
	takenFor = 20 * time.Second
)

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
	host, boot := r.Header.Get("X-Host"), r.Header.Get("X-Boot")
	if len(host) > 40 {
		host = host[:40]
	}
	if len(boot) > 40 {
		boot = boot[:40]
	}
	now := time.Now()
	s.mu.Lock()
	t := &s.tiles[i]
	switch {
	case t.ver == 0:
		log.Printf("tile %d is up (%s, %s)", i, host, r.RemoteAddr)
		t.gen, t.born = 1, now
	case boot != "" && t.boot != "" && boot != t.boot:
		// Another microVM has the place. How long the wall went without
		// it: since the click that asked, or since the old one's last
		// frame when nobody asked here (it was removed some other way).
		from := t.at
		if !t.asked.IsZero() {
			from = t.asked
		}
		t.took, t.gen, t.born = now.Sub(from), t.gen+1, now
		t.asked, t.taken = time.Time{}, false
		s.replaced++
		log.Printf("tile %d is a new microVM, %.1f s after the old one (%s)", i, t.took.Seconds(), host)
	}
	t.boot = boot
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

// replace is the page's click on a tile: its microVM is asked to be
// replaced. Asked once: a second click while the first is waiting changes
// nothing, unless the keeper took it and has not finished in takenFor.
func (s *wall) replace(w http.ResponseWriter, r *http.Request) {
	i, ok := s.index(r)
	if !ok {
		http.Error(w, "no such tile", http.StatusNotFound)
		return
	}
	now := time.Now()
	s.mu.Lock()
	t := &s.tiles[i]
	switch {
	case t.asked.IsZero():
		t.asked, t.taken = now, false
		log.Printf("tile %d: a new microVM is asked for", i)
	case t.taken && now.Sub(t.takenAt) > takenFor:
		t.taken = false
		log.Printf("tile %d: asked for again, %.0f s after the keeper took it", i, now.Sub(t.takenAt).Seconds())
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// asked is the keeper's question: the tiles whose microVM the page asked
// to have replaced and nobody has taken yet. Asking it is how the wall
// knows a keeper is watching.
func (s *wall) asked(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.watched = time.Now()
	out := []int{}
	for i, t := range s.tiles {
		if !t.asked.IsZero() && !t.taken {
			out = append(out, i)
		}
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"tiles": out}); err != nil {
		log.Printf("asked: %v", err)
	}
}

// take is the keeper saying it is replacing a tile: the request is not
// handed out again while it works.
func (s *wall) take(w http.ResponseWriter, r *http.Request) {
	i, ok := s.index(r)
	if !ok {
		http.Error(w, "no such tile", http.StatusNotFound)
		return
	}
	s.mu.Lock()
	if t := &s.tiles[i]; !t.asked.IsZero() {
		t.taken, t.takenAt = true, time.Now()
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *wall) state(w http.ResponseWriter, _ *http.Request) {
	type one struct {
		V    int    `json:"v"`
		Age  int64  `json:"age_ms"`
		Host string `json:"host"`
		Ms   int    `json:"ms"`
		// Asked: the page asked for a new microVM here and it has not
		// drawn yet. Gen: how many microVMs have had the place. Born:
		// how long the one drawing now has been drawing. Took: how long
		// the place was without a picture when it took over.
		Asked bool  `json:"asked"`
		Gen   int   `json:"gen"`
		Born  int64 `json:"born_ms"`
		Took  int64 `json:"took_ms"`
	}
	now := time.Now()
	s.mu.Lock()
	out := make([]one, len(s.tiles))
	live := 0
	for i, t := range s.tiles {
		out[i] = one{V: t.ver, Host: t.host, Ms: t.ms, Age: now.Sub(t.at).Milliseconds(),
			Asked: !t.asked.IsZero(), Gen: t.gen, Took: t.took.Milliseconds()}
		if !t.born.IsZero() {
			out[i].Born = now.Sub(t.born).Milliseconds()
		}
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
		"since_s": now.Sub(s.start).Seconds(), "full_s": s.full.Seconds(),
		"replaced": s.replaced, "keeper": !s.watched.IsZero() && now.Sub(s.watched) <= keeperFor}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		log.Printf("state: %v", err)
	}
}

func (s *wall) reset(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.tiles = make([]tile, s.cols*s.rows)
	s.start, s.full, s.frames, s.replaced = time.Now(), 0, nil, 0
	s.mu.Unlock()
	log.Printf("reset: the stopwatch starts")
	w.WriteHeader(http.StatusNoContent)
}

func (s *wall) page(w http.ResponseWriter, _ *http.Request) {
	// NTILES is the count the info text gives. The page's words keep clear
	// of the five names in capitals, which the replacer takes as its own.
	r := strings.NewReplacer("NTILES", strconv.Itoa(s.cols*s.rows), "COLS", strconv.Itoa(s.cols), "ROWS", strconv.Itoa(s.rows), "TW", strconv.Itoa(s.w), "TH", strconv.Itoa(s.h))
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
	mux.HandleFunc("POST /replace/{i}", s.replace)
	mux.HandleFunc("GET /asked", s.asked)
	mux.HandleFunc("POST /taken/{i}", s.take)
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
.asked img{opacity:.3;filter:grayscale(1)}
.asked span{color:#ffe53c}
.asked{outline:2px solid #ffe53c;outline-offset:-2px}
.fresh{outline:2px solid #3cffb4;outline-offset:-2px}
.fresh span{color:#3cffb4}
#hint{margin:8px 0 0;font-size:13px;color:#8d90c4}
#hint b{color:#ffe53c}
button{margin-top:12px;background:#151330;color:#cfd2ff;border:1px solid #3a3870;padding:6px 16px;font:inherit;cursor:pointer}
#buttons{display:flex;gap:12px}
#info{box-sizing:border-box;width:min(94vw,980px);margin:16px 0 32px;padding:12px 22px 16px;background:#0b0a16;border:1px solid #2a2850;font-size:14px;line-height:1.55}
#info[hidden]{display:none}
#info h2{margin:16px 0 4px;font-size:15px;color:#ffe53c;letter-spacing:1px}
#info h2:first-child{margin-top:4px}
#info p{margin:4px 0}
#info ul{margin:4px 0;padding-left:20px}
#info li{margin:3px 0}
#info b{color:#fff}
#info code{color:#3cffb4}
</style></head><body>
<h1>FIRECRACKER WALL</h1>
<div id="bar"><span id="live"></span><span id="fps"></span><span id="clock"></span><span id="new"></span></div>
<div id="wall"></div>
<div id="hint"></div>
<div id="buttons"><button onclick="fetch('/reset',{method:'POST'})">reset the stopwatch</button><button id="infobtn" aria-expanded="false" aria-controls="info">info</button></div>
<div id="info" hidden>
<h2>What you are looking at</h2>
<p>One picture, a Mandelbrot zoom, drawn by <b>NTILES</b> copies of one small program. Each copy is a container of its own, and each container is a Firecracker microVM with its own Linux kernel. One more container, this wall, puts the pieces together and serves this page.</p>
<h2>How the work is shared out</h2>
<p>The wall hands out the time of the next frame. Each tile knows only its number and where the wall is: it asks for the time, draws its own part of the zoom for that moment, sends the picture back, and goes again, up to eight times a second. The wall keeps every tile's newest picture, and this page shows them side by side. Deeper in the zoom a frame costs more to draw, so the frames a second above are real work, shared by NTILES machines. No tile knows any other: when one goes, the rest carry on, and its place stays dark until it is back.</p>
<h2>How it is repaired</h2>
<p>Click a tile: its microVM is destroyed and a new one boots in its place. The page cannot do that by itself, and that is the point: it is a program in a container, and nothing in a container commands the host. A click only leaves a request on the wall. The keeper, a small loop on the host (<code>05-demoscene-wall.sh</code>, running as the service <code>demoscene-keeper</code>), asks the wall what was clicked, removes that tile's container and runs a new one from the same image. It also runs again any tile that is gone or stopped, however it went. Every tile sends an id made when its program starts, so the wall can tell a new machine from the old one, and the tile says how long it was away. With no keeper watching, a click waits, and the page says so.</p>
<h2>How it is put together</h2>
<p>This runs on a vmx host. Each image is pulled once, or imported from an archive, by its digest, and unpacked onto a ZFS volume kept as its golden. Every container is a ZFS clone of that golden, so a new one copies nothing and takes no space until it writes. It boots in a Firecracker microVM, confined by Firecracker's jailer, running its own copy of the kernel the host runs, with a small init that starts the image's program. The tiles talk to the wall on the host's private network; the wall's port is forwarded from the host, which is how this page reaches you.</p>
<h2>Why build it this way</h2>
<ul>
<li><b>A kernel each.</b> A crash, a kernel bug or a runaway program in one tile stays in that tile. Containers that share one kernel cannot promise that.</li>
<li><b>Nothing is fixed in place.</b> A tile that misbehaves is thrown away and a new one made from the same golden, so repairing a tile and starting a new one are the same act.</li>
<li><b>Identical by construction.</b> Every tile runs the same image, named by its digest, so a replacement is the same bytes as the one it replaced.</li>
<li><b>Cheap to throw away.</b> A new tile is a clone and a boot; each replaced tile shows how long it was away.</li>
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
const n=COLS*ROWS,wall=document.getElementById('wall'),tiles=[];
for(let i=0;i<n;i++){const d=document.createElement('div');d.className='t dead';const im=new Image();const l=document.createElement('span');l.textContent=String(i).padStart(2,'0');d.append(im,l);
d.onclick=()=>{d.classList.add('asked');fetch('/replace/'+i,{method:'POST'});};wall.append(d);tiles.push({d,im,l,v:-1});}
const $=id=>document.getElementById(id);
async function tick(){try{const s=await(await fetch('/state')).json();
s.tiles.forEach((t,i)=>{const o=tiles[i];if(t.v>0&&t.v!==o.v){o.v=t.v;o.im.src='/tile/'+i+'?v='+t.v;}
const name=t.host||String(i).padStart(2,'0'),fresh=t.gen>1&&t.born_ms<4000&&!t.asked;
o.d.classList.toggle('dead',t.v===0||t.age_ms>2000);o.d.classList.toggle('asked',t.asked);o.d.classList.toggle('fresh',fresh);
o.l.textContent=t.asked?name+'  a new one is coming':fresh?name+'  new in '+(t.took_ms/1000).toFixed(1)+' s':name+(t.ms?'  '+t.ms+' ms':'');});
$('live').innerHTML='<b>'+s.live+'</b> of '+n+' microVMs live';$('fps').innerHTML='<b>'+s.fps.toFixed(1)+'</b> frames a second';
$('clock').innerHTML=s.full_s>0?'all '+n+' up in <b>'+s.full_s.toFixed(1)+' s</b>':'<b>'+s.since_s.toFixed(1)+' s</b> since the reset';
$('new').innerHTML=s.replaced>0?'<b>'+s.replaced+'</b> replaced':'';
$('hint').innerHTML=!s.keeper&&s.tiles.some(t=>t.asked)?'<b>no keeper is watching</b>: a click waits until the host runs 05-demoscene-wall.sh --keep':'';}catch(e){}setTimeout(tick,100);}
const ib=$('infobtn'),info=$('info');
function showInfo(on){info.hidden=!on;ib.setAttribute('aria-expanded',on?'true':'false');if(on)info.scrollIntoView({behavior:'smooth',block:'start'});}
ib.onclick=()=>showInfo(info.hidden);
document.addEventListener('keydown',e=>{if(e.key==='Escape'&&!info.hidden)showInfo(false);});
tick();
</script></body></html>`
