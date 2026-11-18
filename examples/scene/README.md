# The demoscene wall

Two small Go programs, each its own image, that `05-demoscene-wall.sh`
runs as containers on a vmx host:

- `wall/` is the wall: it hands every tile the frame's time, keeps each
  tile's newest picture, and serves one page that shows them all, with
  how many tiles are live, how many frames a second they draw together,
  and how long the whole wall took to come up, at its start or after
  "Reset the wall".
  The page's "info" button opens what is going on: how the work is shared
  out, how a tile is repaired, what its disk can do, how the whole is put
  together, why it is built this way, and where the same shape fits.
- `agent/` is a tile: told its place by `TILE` and the wall by `WALL`, it
  draws its part of a Mandelbrot zoom in a cycling palette, at most `FPS`
  frames a second (8), and sends each frame to the wall. A frame deeper in
  the zoom costs more: the wall shows real work. It counts the frames it
  has drawn on its own disk (`/var/lib/scene/frames`, written and flushed
  every second) and, given a volume (`VOLUME`, the directory vmx mounts it
  on, which it checks is a mount), on that volume too; the page shows both
  in the tile's corner. The disk is the machine's and is made again from
  the image with it; the volume outlives it. It draws in the colours
  `LOOK` names: `blue` (also when it is not set), black through deep blue
  and sky to white; `green`, the same through green, which the second image
  sets (`Containerfile.green`), a new version for the wall's Update to roll
  out, blue to green, and its Roll back to take back; `classic`, a cycling rainbow. A tile
  whose microVM is named `tile-N` draws place N, whatever `TILE` says, so a
  clone named for a new place draws that place. It draws only while
  somebody watches: with no page asking the wall how it is for three
  seconds (a page asks ten times a second), the wall holds each tile's ask
  for a frame up to four seconds and answers 204, nothing to draw, so the
  tiles take next to no processor; the moment a page asks, every waiting
  tile is let go and the picture moves again within a frame.

## What a tile shows

The page's **info** button (or the page's address ending `#info`) opens a
legend of every state a tile can be in, drawn with the page's own tiles.
In short:

| On a tile | What it says |
|---|---|
| `disk 1,204` / `volume 3,388`, in the corner | the frames it has drawn, as its disk counts them (new with each machine) and as its volume counts them (outlives the machine) |
| blue or green | the version of its program it runs: blue the first, green the new one Update rolls out; while the wall runs both, each tile names its own, and the line above the wall counts each |
| `tile-07  140 KiB  23 ms` along the bottom | its name, the disk it holds of its own, how long its last frame took |
| a pink border | chosen: the buttons act on it |
| yellow | a press being done, or waiting its turn (`+1`: one more behind it); a grey picture until a new machine draws |
| mint, `new in 2.3 s` | for four seconds: a new boot has the place, and how long it was without a picture |
| cyan | for eight seconds: what the keeper said became of the last press, a refusal with why |
| `up in 0.62 s` | for ten seconds: its kernel starting to its program running |
| dim, its name in red | no frame for two seconds: killed, stopped or removed |

| Button | The disk's count | The volume's count |
|---|---|---|
| Snapshot | goes on; the snapshot keeps it | goes on; the snapshot keeps it |
| Restore | back to the snapshot's | back to the snapshot's |
| Rebuild, Update, Roll back | starts again from nothing | goes on |
| Kill | goes on from the last second written | the same |
| Clone | each clone goes on from the tile's | the same, on the clone's copy |
| Remove | gone | gone |
| Reset the wall | starts again from nothing | starts again, on a new volume |

## Choose tiles, then a button

Click a tile on the page to choose it and again to let it go, or drag
across tiles to choose every one you pass; **All** and **None** choose
every tile or none, and Escape lets them all go. The buttons under the
wall do their verb to every tile chosen, at once:

| Button | What becomes of each tile chosen |
|---|---|
| **Snapshot** | its disk and its volume as they are now, at one moment, in one zfs call (`vmx container snapshot`) |
| **Restore** | its disk and its volume put back to its newest snapshot together: the microVM stopped, both returned and booted again, in under a second; both counts in its corner go back to what they were (`vmx container rollback --restart`). A snapshot a clone stands on is newer than it: the restore is refused until the clone is removed, and the tile says so. With one tile chosen, its snapshots are listed under the buttons, oldest first: a press on one restores the tile to that one, and the snapshots newer than it go with it |
| **Rebuild** | its microVM and its disk destroyed and a new one booted from the image on the same volume: the disk's count starts again from nothing and the volume's goes on, as a program's data does when the program is put on a new machine, its snapshots with the old one; the tile says how long it was away. An empty place gets a tile again |
| **Update** | its microVM and its disk destroyed and a new one booted from the wall's other image, `scene-agent:7-green` (the same program, drawing in green where the first draws in blue: a new version), on the same volume, eight at a time over the wall while the rest draw, so green rolls over the picture a row at a time; the disk's count starts again and the volume's goes on, and its snapshots go with the old machine. A tile on the new version already says so. A rolling update: the old version is not kept running beside the new, and each tile is without a picture for a second or two while its machine is replaced |
| **Roll back** | the same, back to the first image, `scene-agent:7`: green to blue, eight at a time. It changes the program and keeps the data (the volume's count goes on); Restore is the other way back, the data to a snapshot with the program kept. A tile on the first version says there is nothing to roll back to |
| **Kill** | its microVM ended as a crash or a power cut would end it: the process killed outright, with no shutdown. Nothing asks for it back: the keeper finds it stopped, as it would a machine that died any other way, and starts it again with its disk and its volume as they were; the tile says how long its place was without a picture |
| **Clone** N | as many new microVMs from its disk and its volume as they are, at one moment, as the box beside the button says, 1 to 1024 (`vmx container clone`, sixteen at a time): each a ZFS clone, its volume one too (vmx names the copy NEW-VOLUME), so it copies nothing and takes no space until it writes, named for the next places after the picture's own; the wall grows, and each clone goes on counting from the counts it was cloned with. The wall's places, the host's memory (the keeper leaves it 2 GiB) and the addresses the host's network gives out end it, whichever comes first, and the tile says how many were made and why it stopped; a clone that gets no address is taken away again |
| **Remove** | the tile taken away with its volume and their snapshots; its place stays empty. A tile a clone stands on is not removed until the clone is, and a volume a clone's copy stands on stays until the copy goes: the keeper takes it away then |
| **Reset the wall** | every tile and clone taken away with its volume and the picture's own run again from the image on new volumes; the clock says how long the whole wall took, from the press |

Each tile says on the page what became of it, and a press made while
the tile is busy waits its turn. For ten seconds after it boots, a tile
says how long its microVM took, from its kernel starting to its program
running, as the machine itself counts it (`up in 0.62 s`).

`05-demoscene-wall.sh --show` plays the wall's own show on the wall that
is there, the same take every time, for a recording: Reset the wall,
Snapshot every tile, Kill eight, Restore every tile, Rebuild eight,
Update every tile (green over the wall), Roll back every tile (blue back
over it), Clone one twelve times, Reset again. It presses the buttons as the page does,
waits each step out until the keeper has done it (a Kill until the keeper
has started every tile killed again), and pauses the same between them;
open the page first. The keeper leaves the host 2 GiB of
memory: a clone or a new tile past that is refused, on the tile.

The page cannot do any of that by itself, and that is the point: the wall
is a program in a container, and nothing in a container commands the
host. A button only leaves a request on the wall: `POST /do/VERB` with
the chosen tiles' numbers as a JSON list (VERB is `snapshot`, `restore`,
`rebuild`, `update`, `rollback`, `kill`, `clone` or `remove`; a restore to
a snapshot named adds `?at=LABEL`, a clone its count, `?n=12`), or `POST
/reset`. The keeper is a loop on
the host, run as root, which `05-demoscene-wall.sh` starts by itself as
the service `demoscene-keeper` once the wall is up. It asks the wall what
waits (`GET /asked`: each tile's next verb, and whether a reset is
asked), says it has taken each (`POST /acting/N`, `POST /reset/down`),
does it with the `vmx` command, and says what became of it (`POST
/did/N`, with a few words for the tile, and `X-Failed: 1` when it was not
done; `POST /reset/up` once every tile is down for a reset). It does
every verb on many tiles at once, a dozen presses at a time, and a
reset's removals and runs sixteen at a time; `journalctl -fu
demoscene-keeper` shows each. An older vmx made one of the runs and
clones asked for at one moment and refused the others ("run it
again"): the keeper asks again until each is made. It
also runs again any tile that is gone or stopped, however it went (`vmx
container remove tile-07` is put right in a moment), unless the page
removed it. It touches only the tiles the script made and the clones it
made of them. `systemctl stop demoscene-keeper` stops it, and nothing the
page asks is done until it starts again; `--keep` starts it again,
`--keep-here` runs it in a terminal instead, and `--remove` stops it
before it takes the wall away, the clones before the tiles they came
from.

The wall knows a new microVM by the boot id every tile sends with its
frames (`X-Boot`), made when the tile's program starts. With no keeper
watching, a request waits, and the page says so. `go test ./wall` tests
the wall's side of all of this.

The line under the clock is the keeper's too. Every two seconds it reads
the tiles' microVMs on the host (`ps`: each one's resident memory) and
their disks (`zfs list`: what each tile's disk refers to, which is what a
full copy would take, and what it holds of its own, with each image the
tiles are clones of counted once), and their volumes (16 MiB each, which
ZFS keeps room for in full, written to or not), and posts them to the wall
(`POST /host`). The page shows them, and each tile's own share beside its
name.
Forty tiles on a vmx host, measured: 263 MiB as full copies, 12 MiB as
ZFS keeps them (each tile's own 140 KiB or so, and the image they share,
once), and 5.6 GiB of memory.

## Run it

The three images come with the examples: `images/` holds their
archives, `scene-agent:7`, `scene-agent:7-green` and `scene-wall:14`, the
very ones whose digests the script pins. On the vmx host, as root, one
command, with nothing fetched: the images imported from those archives
and checked, the wall, its tiles, and the keeper. A wall that is there
is taken away first. The script prints the page's address: open it in a
browser.

```sh
cd /home/examples        # or wherever the examples are: a clone, or where the installer put them
./05-demoscene-wall.sh
./05-demoscene-wall.sh --remove && ./05-demoscene-wall.sh    # the wall made again
```

## Build the images yourself

On any machine with Go and Podman (or Docker):

```sh
cd scene
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o scene-agent ./agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o scene-wall ./wall
podman build -f Containerfile.agent -t scene-agent:7 .
podman build -f Containerfile.green -t scene-agent:7-green .
podman build -f Containerfile.wall -t scene-wall:14 .
podman save --format oci-archive -o scene-agent.tar scene-agent:7
podman save --format oci-archive -o scene-agent-green.tar scene-agent:7-green
podman save --format oci-archive -o scene-wall.tar scene-wall:14
for f in scene-agent scene-agent-green scene-wall; do echo "$f $(tar -xOf $f.tar index.json | jq -r '.manifests[0].digest')"; done
scp scene-agent.tar scene-agent-green.tar scene-wall.tar root@vmx-host:/home/
```

Then on the vmx host, the archives you built in place of the ones in
`images/`: `--from DIR` takes them from DIR and checks each against the
digest the script names for it. Archives you built yourself have digests
of their own (the build stamps its time in them): name them, as printed
above.

```sh
AGENT_ARCHIVE=/home/scene-agent.tar AGENT_DIGEST=sha256:... AGENT_IMAGE=scene-agent:7 \
NEXT_ARCHIVE=/home/scene-agent-green.tar NEXT_DIGEST=sha256:... NEXT_IMAGE=scene-agent:7-green \
WALL_ARCHIVE=/home/scene-wall.tar WALL_DIGEST=sha256:... WALL_IMAGE=scene-wall:14 \
./05-demoscene-wall.sh
```

Try either program on your own computer first: `go run ./wall` serves
<http://localhost:8090>, and `TILE=0 WALL=http://localhost:8090 go run ./agent`
draws its tile there (its count is kept only where it may write
`/var/lib/scene`, as in its microVM; `VOLUME` names a directory only when
it is a mount point, so leave it unset there).
