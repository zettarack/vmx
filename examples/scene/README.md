# The demoscene wall

Two small Go programs, each its own image, that `05-demoscene-wall.sh`
runs as containers on a vmx host:

- `wall/` is the wall: it hands every tile the frame's time, keeps each
  tile's newest picture, and serves one page that shows them all, with
  how many tiles are live, how many frames a second they draw together,
  and how long the whole wall took to come up after "reset the stopwatch".
- `agent/` is a tile: told its place by `TILE` and the wall by `WALL`, it
  draws its part of a Mandelbrot zoom in a cycling palette, at most `FPS`
  frames a second (8), and sends each frame to the wall. A frame deeper in
  the zoom costs more: the wall shows real work.

Build both images on any machine with Go and Podman (or Docker):

```sh
cd scene
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o scene-agent ./agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o scene-wall ./wall
podman build -f Containerfile.agent -t scene-agent:1 .
podman build -f Containerfile.wall -t scene-wall:1 .
podman save --format oci-archive -o scene-agent.tar scene-agent:1
podman save --format oci-archive -o scene-wall.tar scene-wall:1
for f in scene-agent scene-wall; do echo "$f $(tar -xOf $f.tar index.json | jq -r '.manifests[0].digest')"; done
scp scene-agent.tar scene-wall.tar root@vmx-host:/home/
```

Then on the vmx host, with the two digests printed above:

```sh
AGENT_ARCHIVE=/home/scene-agent.tar AGENT_DIGEST=sha256:... \
WALL_ARCHIVE=/home/scene-wall.tar WALL_DIGEST=sha256:... \
AGENT_IMAGE=scene-agent:1 WALL_IMAGE=scene-wall:1 ./05-demoscene-wall.sh
```

Try either program on your own computer first: `go run ./wall` serves
<http://localhost:8090>, and `TILE=0 WALL=http://localhost:8090 go run ./agent`
draws its tile there.
