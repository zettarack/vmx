# The demoscene wall

Two small Go programs, each its own image, that `05-demoscene-wall.sh`
runs as containers on a vmx host:

- `wall/` is the wall: it hands every tile the frame's time, keeps each
  tile's newest picture, and serves one page that shows them all, with
  how many tiles are live, how many frames a second they draw together,
  and how long the whole wall took to come up after "reset the stopwatch".
  The page's "info" button opens what is going on: how the work is shared
  out, how a tile is repaired, how the whole is put together, why it is
  built this way, and where the same shape fits.
- `agent/` is a tile: told its place by `TILE` and the wall by `WALL`, it
  draws its part of a Mandelbrot zoom in a cycling palette, at most `FPS`
  frames a second (8), and sends each frame to the wall. A frame deeper in
  the zoom costs more: the wall shows real work.

## A click replaces a tile's microVM

Click a tile on the page and its microVM is destroyed and a new one
booted in its place; the tile says how long it was away. The page says
nothing of this until it is clicked, or its info is opened: it is there
to be found.

The page cannot do that by itself, and that is the point: the wall is a
program in a container, and nothing in a container commands the host. A
click only records a request on the wall (`POST /replace/N`). The keeper
is a loop on the host, run as root, which `05-demoscene-wall.sh` starts
by itself as the service `demoscene-keeper` once the wall is up: it asks
the wall what was clicked (`GET /asked`), says it has taken each (`POST
/taken/N`), removes the tile's container and runs it again with the `vmx`
command, and says how long each took (`journalctl -fu demoscene-keeper`).
It also runs again any tile that is gone or stopped, however it went
(`vmx container remove tile-07` is put right in a moment). It touches
only the tiles the script made. `systemctl stop demoscene-keeper` stops
it, and a tile pulled out then stays out; `--keep` starts it again,
`--keep-here` runs it in a terminal instead, and `--remove` stops it
before it takes the wall away.

The wall knows a new microVM by the boot id every tile sends with its
frames (`X-Boot`), made when the tile's program starts. With no keeper
watching, a click waits, and the page says so.

Build both images on any machine with Go and Podman (or Docker):

```sh
cd scene
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o scene-agent ./agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o scene-wall ./wall
podman build -f Containerfile.agent -t scene-agent:2 .
podman build -f Containerfile.wall -t scene-wall:3 .
podman save --format oci-archive -o scene-agent.tar scene-agent:2
podman save --format oci-archive -o scene-wall.tar scene-wall:3
for f in scene-agent scene-wall; do echo "$f $(tar -xOf $f.tar index.json | jq -r '.manifests[0].digest')"; done
scp scene-agent.tar scene-wall.tar root@vmx-host:/home/
```

Then on the vmx host, one command: the images from the archives, the
wall, its tiles, and the keeper.

```sh
./05-demoscene-wall.sh --from /home
```

`--from` checks each archive against the digest the script names for
it. Archives you built yourself have digests of their own: name them, as
printed above.

```sh
AGENT_ARCHIVE=/home/scene-agent.tar AGENT_DIGEST=sha256:... AGENT_IMAGE=scene-agent:2 \
WALL_ARCHIVE=/home/scene-wall.tar WALL_DIGEST=sha256:... WALL_IMAGE=scene-wall:3 \
./05-demoscene-wall.sh
```

Try either program on your own computer first: `go run ./wall` serves
<http://localhost:8090>, and `TILE=0 WALL=http://localhost:8090 go run ./agent`
draws its tile there.
