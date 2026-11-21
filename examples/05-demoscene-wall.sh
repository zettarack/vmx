#!/usr/bin/env bash
# The demoscene wall: forty virtual machines drawing one picture.
#
# WHAT IT IS
# One picture, hours of scenes (scene/show: fourteen families, seven minutes
# each, each with a seed of its own; the VMX intro, the wordmark with
# zettarack.com typed under it, opens the show and
# comes back after every two scenes; the page's Before, Hold, Next and a
# button for each family, or the arrows, space, 1 to 0, -, =, [ and ], move the show for
# every tile at once, and its extras lay stars, rasters, scanlines, a
# vignette or grain over every scene),
# cut into a grid of places, COLS x ROWS
# (8 x 5 = 40). Each tile (scene/agent) is a container of its own: an
# ordinary container image, the same one for every tile, which vmx runs as
# a Firecracker microVM with a kernel and a disk of its own. No container
# engine runs on the host. A tile asks the wall for the frame's time, draws
# its own square of the picture at that moment and sends it back, up to
# eight frames a second; a deep fractal or a raymarched solid costs more
# to draw than a plasma, so
# the frames a second the page shows are real work. Each tile counts the
# frames it has drawn twice, each written and flushed every second: on its
# own disk, which is the machine's and is made again from the image with
# it, and on its volume, a 16 MiB ZFS volume of its own (vmx container run
# --volume) that outlives the machine. Both show in its corner of the page:
# the two counts are how a disk and a volume put back, carried into a
# clone, or kept across a new machine are seen. For ten seconds after it
# boots, a tile says how long its microVM took, from its kernel starting
# to its program running, as the machine itself counts it. It draws only
# while somebody watches (POWER, below).
# The wall (scene/wall) is one more container: it hands out the frame's
# time, keeps each tile's newest square, and serves the page that shows
# them all.
# It is the shape of one service spread over many replicas, each with a
# name and a part of the work of its own, a coordinator putting the parts
# together and a controller running again any replica that goes (the
# keeper, below): what forty pods of a Kubernetes StatefulSet are, but each
# replica here a whole machine, whose disk is snapshotted, put back or
# cloned in under a second by ZFS.
#
# THE PAGE
# The picture; a clock that times the wall coming up, at its start or from
# a press of Reset the wall; how many tiles are live and how many frames a
# second they draw together; the count in each tile's corner; and a line
# under the clock with what the host holds of the wall, as the keeper reads
# it every two seconds: the microVMs running and the memory they take, and
# their disks two ways, as full copies would take them and as ZFS keeps
# them (each tile's own blocks, and the image's once). Each tile shows the
# disk it holds of its own. The info button opens the account of it on the
# page: how the work is shared out, how a tile is repaired, what its disk
# can do, and why it is built this way.
#
# THE BUTTONS
# A click chooses a tile and another lets it go, a drag chooses every tile
# it passes, All and None choose every tile or none, and Escape lets them
# all go; choosing puts cfg's buttons away, so every row can be chosen, and
# cfg, the count chosen beside it, brings them back. The buttons under the
# wall (cfg opens them) do their verb to every tile chosen,
# at once, by the keeper with the vmx command, and each tile says on the
# page what became of it and how long it took:
#   Snapshot        its disk and its volume as they are, at one moment, in
#                   one zfs call (vmx container snapshot; the snapshot is
#                   named for the time, in UTC)
#   Restore         the microVM stopped, its disk and its volume put back
#                   to the newest snapshot taken with Snapshot together and
#                   booted again, in under a second: both counts in its
#                   corner go back (vmx
#                   container rollback --restart). A snapshot a clone stands
#                   on is newer than that: refused until the clone is gone.
#                   With one tile chosen, the page lists its snapshots under
#                   the buttons (the keeper asks vmx container snapshots): a
#                   press on one restores the tile to that one, and the
#                   snapshots newer than it go with it, as the tile says
#   Update          its microVM and its disk destroyed and a new one booted
#                   from the wall's other image, scene-agent:21-green (the
#                   same program drawing in green where the first draws in
#                   blue: a new version), on the
#                   same volume, eight at a time over the wall while the
#                   rest draw, so the new version rolls over the picture a
#                   row at a time; the disk's count starts again and the
#                   volume's goes on, as data does through an upgrade; its
#                   snapshots go with the old machine. A tile on the new
#                   version already says so. A Rebuild, a clone or a tile
#                   run again keeps the version it has
#   Roll back       the same, back to the first image, scene-agent:21: green
#                   to blue, eight at a time over the wall. It changes the
#                   program and keeps the data (the volume's count goes on);
#                   Restore is the other way back, the data to a snapshot
#                   with the program kept. A tile on the first version says
#                   there is nothing to roll back to
#   Rebuild         its microVM and its disk destroyed and a new one booted
#                   from the image on the same volume, timed: the disk's
#                   count starts again from nothing and the volume's goes
#                   on, as a program's data does when the program is put on
#                   a new machine, its snapshots with the old one; an empty
#                   place gets a tile again
#   Kill            its microVM ended as a crash would end it: the process
#                   killed outright (SIGKILL), no shutdown. Nothing asks
#                   for it back: the keeper finds it stopped and starts it
#                   again with its disk and its volume as they were, and the
#                   tile says how long it was away
#   Clone N         N new microVMs from its disk and its volume as they are,
#                   at one moment (vmx container
#                   clone), N from the box beside the button (1 to 1024),
#                   sixteen at a time, named for the next places after the
#                   picture's own: the wall grows. Each is a ZFS clone, so
#                   nothing is copied and it takes no space until it writes,
#                   its volume a clone too (vmx names it NEW-VOLUME), and
#                   each goes on counting from the counts it was cloned
#                   with. The wall's places (1024), the host's memory (the
#                   keeper leaves it 2 GiB) and the addresses the host's
#                   network gives out end it, whichever comes first: a clone
#                   that gets no address is taken away again, and the tile
#                   says how many were made and why it stopped
#   Remove          the tile taken away with its volume, and their snapshots;
#                   its place stays empty. A tile a clone stands on is not
#                   removed until the clone is
#   Reset the wall  every tile and clone taken away with its volume and the
#                   picture's own run again from the image on new volumes,
#                   timed from the press
#   Shut down       pressed twice: the wall taken away as --remove takes it,
#                   every tile, clone and volume, the wall and its keeper;
#                   the images stay, and the page says the wall is gone.
#                   This script on the host starts it again.
#
# THE KEEPER
# The page is a program in a container and commands nothing; nothing in a
# container commands the host. A button only leaves a request on the wall.
# The keeper is this script on the host, as root, run as the service
# demoscene-keeper: three times a second it asks the wall what waits, does
# each with the vmx command (a dozen tiles' verbs at once; a clone's or a
# reset's tiles sixteen at a time), and says on the wall and in its log
# what became of each (journalctl -fu demoscene-keeper). It also runs again
# any tile that is gone and starts any that has stopped, however that came
# about (vmx container remove tile-05 by hand: it comes back), and tries
# again five seconds later when that fails. It touches only the tiles this
# script made and the clones it made of them, and runs no new microVM past
# the host's last 2 GiB of memory. An older vmx made one of the runs and
# clones asked for at one moment and refused the others ("run it again"):
# each is asked again until it is made.
# systemctl stop demoscene-keeper stops it: the wall stays, and nothing the
# page asks is done until --keep starts it again. After the host restarts,
# the containers that were running start with it, the wall's among them,
# but the keeper does not (it is a service of this run, not the host's):
# --keep starts it again, and it runs again any tile that is not back.
#
# WHAT THIS SCRIPT CHECKS AS IT GOES
# Before anything: vmx, jq and flock are here and this vmx has containers;
# COLS x ROWS is 1 to 64 tiles; this script has no wall here already; no
# container has a tile's name. With --from: both archives are in DIR, and
# vmx imports each only if it is the image its digest below names. Then:
# the wall starts and answers within 30 seconds; the tiles start sixteen at
# a time, each timed from the first, and one that does not stops the script
# with its name; it waits up to a minute for every tile to draw, and says
# how many do, how long the whole wall took and how many frames a second it
# draws. --remove stops the keeper, then removes only what this script
# made, by its exact name, newest first (a clone before the tile it stands
# on), sixteen at a time, each tile's volume after it; the images stay, and
# it says how to remove them.
#
# THE TESTS
# scene/wall's own tests (cd scene && go test ./wall: 18 of them) hold the
# page's side: each verb waits for the keeper in the order it was pressed,
# a press made twice waits once, a bad request is refused and keeps
# nothing, a rebuild is timed from the press, a restore is the same machine
# started again, a verb taken and never answered is handed out again, a
# clone's place is told before it draws, a count of clones is bounded, a
# reset takes the wall down, times it coming back and ends if it stalls,
# and what the host says is checked before it is shown.
#
# MEASURED (a vmx host of 24 threads and 64 GB, 2026-11-17)
# Forty tiles started in 3.6 s and all drawing at 18.6 s (the host's network
# gives new machines their addresses a few at a time: most of the wait, and
# most of a clone's time); Reset the wall, 7.0 s from the press, 10.5 s from
# eighty tiles; Clone 10, 21.9 s; Clone 30, 60.6 s; forty tiles' disks, 263
# MiB as full copies and 12 MiB as ZFS keeps them; their memory, 5.6 GiB.
#
# POWER
# The tiles draw only while somebody watches: a page open (it asks the
# wall how it is ten times a second), or this script while it waits for
# them to start. With nobody for three seconds the wall holds each tile's
# ask for a frame, and the tiles draw nothing and take next to no
# processor; a page opened lets every tile go at once, and the picture
# moves again within a frame. The microVMs keep running and their memory
# stays taken: --remove takes the wall away.
#
# USAGE
#   ./05-demoscene-wall.sh --from DIR   the wall and COLS x ROWS tiles (8 x 5 = 40), the images from the archives in DIR, then its keeper
#   ./05-demoscene-wall.sh              the same, the images from the archives beside this script (scene/images);
#                                       with AGENT_IMAGE, NEXT_IMAGE and WALL_IMAGE set, pulled from their registry
#   ./05-demoscene-wall.sh --keep       start the keeper again, for the wall that is there
#   ./05-demoscene-wall.sh --keep-here  the keeper in this terminal instead, until Ctrl+C
#   ./05-demoscene-wall.sh --show       the wall's own show, the same take every time, for a recording:
#                                       Reset, Snapshot every tile, Kill eight (waited out until
#                                       the keeper has them back), Restore every tile,
#                                       Rebuild eight, Update every tile, Roll back every tile, Clone one twelve
#                                       times, Reset; open the page first
#   ./05-demoscene-wall.sh --demo       the wall (made first when there is none) explained from one container up, for a recording to cut from:
#                                       its words under the picture on every page open on the wall, with the
#                                       numbers it measured; the keeper stopped and started, a tile removed
#                                       by hand, snapshot and restore, the rest run, the wall doubled and
#                                       doubled again, a reset, a kill, the new version and back, rebuilds,
#                                       clones; cards beside it: the image's way to a machine, a vmx job's
#                                       steps as vmx ran them, the doublings' queue, jewm's keys; HOLD (7)
#                                       seconds a line; open the page first, in a window of the shape to record
#   ./05-demoscene-wall.sh --remove     stop the keeper; remove every tile, clone, volume and the wall this script made
#
# The images come with the script: scene/images holds the archives of
# the three, scene-agent:21, its next version scene-agent:21-green and
# scene-wall:38, which it takes with no argument and checks against the
# digests below, so a clone of the examples, or a host whose installer put
# them on it, runs the wall with no registry and no network.
# scene/README.md builds them again from their sources. --from DIR takes
# scene-agent.tar, scene-agent-green.tar and scene-wall.tar from another
# DIR, checked the same way; AGENT_ARCHIVE and AGENT_DIGEST, NEXT_ARCHIVE
# and NEXT_DIGEST, WALL_ARCHIVE and WALL_DIGEST name other archives; and
# AGENT_IMAGE, NEXT_IMAGE and WALL_IMAGE alone pull the images from a
# registry of your own. COLS (8),
# ROWS (5) and PORT (8090, this host's port for the page) change the wall.
# It prints the page's address: open it in a browser.
# Examples:
#   ./05-demoscene-wall.sh                     the wall and 40 tiles, from the archives in scene/images
#   ./05-demoscene-wall.sh --from /home        the same, from /home/scene-agent.tar, scene-agent-green.tar and scene-wall.tar
#   ./05-demoscene-wall.sh --remove && ./05-demoscene-wall.sh --from /home     the wall that is there, made again
#   COLS=4 ROWS=2 PORT=9090 ./05-demoscene-wall.sh --from /home     8 tiles, the page on port 9090
#   AGENT_IMAGE=ghcr.io/you/scene-agent:21 NEXT_IMAGE=ghcr.io/you/scene-agent:21-green WALL_IMAGE=ghcr.io/you/scene-wall:38 ./05-demoscene-wall.sh
#   AGENT_ARCHIVE=/srv/agent.tar AGENT_DIGEST=sha256:... AGENT_IMAGE=scene-agent:21 \
#   NEXT_ARCHIVE=/srv/green.tar NEXT_DIGEST=sha256:... NEXT_IMAGE=scene-agent:21-green \
#   WALL_ARCHIVE=/srv/wall.tar WALL_DIGEST=sha256:... WALL_IMAGE=scene-wall:38 ./05-demoscene-wall.sh
#   ./05-demoscene-wall.sh --keep              the keeper again, after systemctl stop demoscene-keeper
#   ./05-demoscene-wall.sh --remove            the keeper stopped, the wall, its tiles, clones and volumes removed
# Run it on the vmx host, as root.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
cols=${COLS:-8}
rows=${ROWS:-5}
port=${PORT:-8090}
# What this script made, by exact name, beside it: --remove takes only those.
made=$here/.made/05-demoscene-wall.made
# What the wall is, for --keep and --remove: its images, its address, its tiles.
kept=$here/.made/05-demoscene-wall.kept
# The keeper's own: the lock the list is written under, and a file for each
# tile it has a job in hand for (and .reset while it makes the wall again).
book=$here/.made/05-demoscene-wall.lock
busy=$here/.made/05-demoscene-wall.busy
# The wall's places, its own and its clones' (scene/wall's maxTiles).
most=1024
# The tiles a launch could not start, while it waits for the rest.
failed=$here/.made/05-demoscene-wall.failed
# The volume each tile counts on beside its disk (scene/agent's VOLUME), a
# line NAME VOLUME for each one not named for itself: the picture's own
# tile-NN has the volume tile-NN, and a clone vmx's copy of its source's,
# NEW-VOLUME, recorded as it is made. A volume holds two counts: 16 MiB,
# the least vmx makes, is room enough.
vols=$here/.made/05-demoscene-wall.volumes
vol_size=16M
# The image each tile runs that is not the tiles' first, a line NAME IMAGE:
# an Update moves a tile to the wall's other image, a new version of its
# program, and a Rebuild, a clone or a run again keeps the version it has.
imgs=$here/.made/05-demoscene-wall.images
# The volumes tiles left behind, one a line: vmx keeps a volume while a
# clone's copy of it stands (a clone rebuilt is on a new disk and still on
# its copy), and the keeper takes it away once the copy has gone.
orphans=$here/.made/05-demoscene-wall.orphans
# What the keeper and the wall say to each other, the keeper's side: the
# wall says its own in every answer (scene/wall's protocol), and a wall of
# another is asked nothing. 2 since 2026-11-18: a restore is asked as
# restore, and rollback is a roll back to the first version; an older
# wall's "rollback" was a restore, and taken for a roll back it would have
# put a new machine in the place, its snapshots gone with the old one.
protocol=2
# The keeper's service: one on a host, as the wall's port is.
keeper_unit=demoscene-keeper
# The images' digests, the tiles' scene-agent:21 and its second version
# scene-agent:21-green, and the wall's scene-wall:38,
# as scene/README.md builds them: what an archive of each is checked
# against (--from).
agent_digest=sha256:f0a237a3c51c517255f9355801fc55d453a794dfbf52c0835841e5ccc4c43a17
wall_digest=sha256:f6381a3689e0822ecf91a05340e87410e72ac9d9027958249dc0c8647355e887
next_digest=sha256:b8f4bff38a102aeedd2ec6af6a2aea529dadfd80f89f1b60789cba59aeffd85f

die() { echo "${0##*/}: $*" >&2; exit 1; }
# With no argument and no image named in the environment, the images are
# the archives beside this script, as --from would take them from there:
# carried in the examples, so a clone, or a host whose installer put the
# examples on it, runs the wall with nothing fetched (2026-11-18: the
# wall ran only from archives a person carried in, its registry
# unpublished).
images=$here/scene/images
if [ $# -eq 0 ] && [ -z "${AGENT_IMAGE:-}${NEXT_IMAGE:-}${WALL_IMAGE:-}${AGENT_ARCHIVE:-}${NEXT_ARCHIVE:-}${WALL_ARCHIVE:-}" ] &&
	[ -f "$images/scene-agent.tar" ] && [ -f "$images/scene-agent-green.tar" ] && [ -f "$images/scene-wall.tar" ]; then
	set -- --from "$images"
fi
# --from DIR: the images from the archives in DIR, under their own names.
if [ "${1:-}" = --from ]; then
	[ $# -eq 2 ] && [ -n "$2" ] || die "usage: ${0##*/} --from DIR"
	from=${2%/}
	shift 2
	if ! [ -f "$from/scene-agent.tar" ] || ! [ -f "$from/scene-agent-green.tar" ] || ! [ -f "$from/scene-wall.tar" ]; then
		# The examples keep the three in scene/images: --from the examples'
		# own directory found nothing there, and said only that
		# (2026-11-18); with none, the script takes those beside it.
		if [ -f "$from/scene/images/scene-agent.tar" ]; then
			die "$from does not hold scene-agent.tar, scene-agent-green.tar and scene-wall.tar; $from/scene/images does: ${0##*/} with no argument takes those beside it, or --from $from/scene/images"
		fi
		die "$from does not hold scene-agent.tar, scene-agent-green.tar and scene-wall.tar"
	fi
	AGENT_ARCHIVE=${AGENT_ARCHIVE:-$from/scene-agent.tar} AGENT_DIGEST=${AGENT_DIGEST:-$agent_digest} AGENT_IMAGE=${AGENT_IMAGE:-scene-agent:21}
	NEXT_ARCHIVE=${NEXT_ARCHIVE:-$from/scene-agent-green.tar} NEXT_DIGEST=${NEXT_DIGEST:-$next_digest} NEXT_IMAGE=${NEXT_IMAGE:-scene-agent:21-green}
	WALL_ARCHIVE=${WALL_ARCHIVE:-$from/scene-wall.tar} WALL_DIGEST=${WALL_DIGEST:-$wall_digest} WALL_IMAGE=${WALL_IMAGE:-scene-wall:38}
fi
agent=${AGENT_IMAGE:-ghcr.io/zettarack/scene-agent:21}
next=${NEXT_IMAGE:-ghcr.io/zettarack/scene-agent:21-green}
wall=${WALL_IMAGE:-ghcr.io/zettarack/scene-wall:38}
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
command -v flock >/dev/null || die "this needs flock (util-linux)"
vmx container list --json >/dev/null 2>&1 || die "this vmx has no containers: upgrade it to a release that has 'vmx container'"
has() { vmx container list --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }
address() { vmx container list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .address // empty'; }
# vol_of NAME: the volume tile NAME counts on: the one recorded for it, else
# its own name. has_vol VOLUME: vmx has a volume of that name.
vol_of() { local v=; if [ -f "$vols" ]; then v=$(awk -v n="$1" '$1 == n { print $2; exit }' "$vols"); fi; echo "${v:-$1}"; }
has_vol() { vmx container volumes --json | jq -e --arg v "$1" 'any(.[]?; .name == $v)' >/dev/null; }
# take_copies VOLUME: the copies vmx made of VOLUME for clones (NEW-VOLUME
# by its rule, NEW a place of the wall's) that no container has, taken away
# deepest first: vmx keeps a volume while a copy stands on it. For a copy no
# record names (a keeper stopped while vmx made it, 2026-11-20). Never
# while a wall runs: a tile being made again has no container for a moment.
take_copies() {
	local list c
	# vmx's list not read: the volume's own removal after fails, and says so.
	list=$(vmx container volumes --json) || return 0
	while read -r c; do
		[ -n "$c" ] || continue
		# One that will not go is said by its source's refusal after.
		if vmx container volume-remove "$c" >/dev/null 2>&1; then echo "   $c, a clone's copy no container had, on $1: taken away"; fi
	done < <(jq -r --arg v "$1" '.[]? | select((.container // "") == "" and (.name | test("^tile-[0-9]+-")) and (.name | endswith("-" + $v))) | .name' <<<"$list" | awk '{ print length($0), $0 }' | sort -rn | cut -d' ' -f2-)
}
# quiet ARGS...: vmx ARGS, what it says of each step kept back unless it
# fails; then all of it is shown. A new container an older vmx refused
# because another asked for at the same moment took the uid it planned
# ("run it again") is asked again; a newer vmx takes another uid itself.
quiet() {
	local out
	for _ in $(seq 1 64); do
		if out=$(vmx "$@" 2>&1 >/dev/null); then return 0; fi
		case $out in *"run it again"*) ;; *) break ;; esac
	done
	printf '%s\n' "$out" >&2
	return 1
}
# fact KEY: what the wall's record says of it; nothing when it does not.
fact() { [ -f "$kept" ] && awk -v k="$1" '$1 == k { print $2 }' "$kept"; }

# makers: the other copies of this script that made a wall on this host,
# each by its directory, one a line: the running keeper's (its description
# names the directory it runs from) and any whose record is under /home,
# where the examples are kept. A wall is one to a host, as its port and
# its keeper's name are, and only the copy that made it has its names: the
# copy in /home/examples said "nothing to remove" while the wall made from
# /home/show/examples ran (2026-11-18).
makers() {
	local d
	d=$(systemctl show -p Description --value "$keeper_unit" 2>/dev/null || true)
	{
		case $d in
		"the demoscene wall's keeper, /"*)
			d=${d#"the demoscene wall's keeper, "}
			if [ -f "$d/.made/05-demoscene-wall.made" ]; then echo "$d"; fi
			;;
		esac
		# A directory find cannot read holds no record this script wrote:
		# it writes them as root, which reads everything under /home.
		find /home -maxdepth 4 -path '*/.made/05-demoscene-wall.made' -printf '%h\n' 2>/dev/null | sed 's|/\.made$||' || true
	} | { grep -vxF -- "$here" || true; } | sort -u
}

# by_another WHAT: refuse WHAT, naming the command that takes away the
# wall another copy made here, when one did; otherwise nothing.
by_another() {
	local others
	others=$(makers)
	[ -n "$others" ] || return 0
	if [ "$(wc -l <<<"$others")" -eq 1 ]; then
		die "$1: the wall on this host was made by another copy of this script, and only it knows the wall's names: $others/${0##*/} --remove takes it away"
	fi
	die "$1: another copy of this script made the wall on this host, and only it knows the wall's names; the copies with a wall's record: $(paste -sd ' ' <<<"$others") (the one that made this wall takes it away with --remove)"
}

# remove_all [keeper]: the wall this script made taken away by its record:
# every tile, clone and volume it names and the wall, sixteen at a time;
# the images stay. --remove calls it once the keeper is stopped; the
# keeper calls it itself (keeper) when the page asks for a shutdown, once
# what it had in hand is done.
remove_all() {
	local agent next wall n v k running there vthere left present volumes
	local -a names all cleft vleft
	# The images the wall was made from, as its record names them.
	agent=$(fact agent || true)
	next=$(fact next || true)
	wall=$(fact wall || true)
	# The records next, under the list's lock: a keeper in a terminal
	# (--keep-here) writes nothing more to them and stops at its next pass,
	# once what it has in hand is done.
	exec 8>"$book"
	flock 8
	mapfile -t names <"$made"
	rm -f "$made" "$kept"
	flock -u 8
	# The keeper itself, shutting the wall down, waits for nobody: it may be
	# the keeper in a terminal.
	if [ "${1:-}" != keeper ]; then
		for _ in $(seq 1 150); do
			pgrep -f -- '05-demoscene-wall\.sh --keep-here$' >/dev/null || break
			sleep 0.2
		done
	fi
	# Sixteen at a time, newest first: a clone made from a tile goes before
	# the tile, which is not removed while a clone stands on its disk; one
	# refused for that, its clone still going, goes in the next round.
	# Each tile's volume goes after it (its snapshots on it go with the
	# tile), counted apart: one a clone's copy still stands on goes in a
	# later round, once the copy has.
	all=("${names[@]}")
	present=$(vmx container list --json | jq -r '.[].name')
	volumes=$(vmx container volumes --json | jq -r '.[]?.name')
	there=0 vthere=0
	for n in "${names[@]}"; do
		if grep -qx -- "$n" <<<"$present"; then there=$((there + 1)); fi
		if grep -qx -- "$(vol_of "$n")" <<<"$volumes"; then vthere=$((vthere + 1)); fi
	done
	left=$here/.made/05-demoscene-wall.left
	for _ in 1 2 3 4 5 6; do
		: >"$left"
		running=0
		for ((k = ${#names[@]} - 1; k >= 0; k--)); do
			n=${names[k]}
			v=$(vol_of "$n")
			grep -qx -- "$n" <<<"$present" || grep -qx -- "$v" <<<"$volumes" || continue
			if [ "$running" -ge 16 ]; then
				wait -n || true
				running=$((running - 1))
			fi
			(
				if grep -qx -- "$n" <<<"$present" && ! vmx container remove "$n" >/dev/null 2>&1; then
					echo "$n" >>"$left"
				elif grep -qx -- "$v" <<<"$volumes" && ! vmx container volume-remove "$v" >/dev/null 2>&1; then
					echo "$n" >>"$left"
				fi
			) </dev/null &
			running=$((running + 1))
		done
		wait
		mapfile -t names <"$left"
		present=$(vmx container list --json | jq -r '.[].name')
		volumes=$(vmx container volumes --json | jq -r '.[]?.name')
		[ ${#names[@]} -gt 0 ] || break
	done
	rm -f "$left"
	# A volume a clone's copy still stands on, the copy no container has
	# and no record names: the copies first, then the volume, now that the
	# wall's containers are gone (2026-11-20).
	for n in "${names[@]}"; do
		v=$(vol_of "$n")
		if grep -qx -- "$n" <<<"$present" || ! grep -qx -- "$v" <<<"$volumes"; then continue; fi
		take_copies "$v"
		# A failure is said below, as a volume not removed.
		vmx container volume-remove "$v" >/dev/null 2>&1 || true
	done
	volumes=$(vmx container volumes --json | jq -r '.[]?.name')
	# The volumes tiles left behind, now that the copies that held them
	# are gone.
	if [ -s "$orphans" ]; then
		while read -r v; do
			grep -qx -- "$v" <<<"$volumes" || continue
			vthere=$((vthere + 1))
			take_copies "$v"
			# A failure is said below, as a volume not removed.
			vmx container volume-remove "$v" >/dev/null 2>&1 || true
		done <"$orphans"
		volumes=$(vmx container volumes --json | jq -r '.[]?.name')
	fi
	cleft=()
	for n in "${all[@]}"; do
		if grep -qx -- "$n" <<<"$present"; then cleft+=("$n"); fi
	done
	# Each volume still there once, the tiles' and those left behind.
	mapfile -t vleft < <({ for n in "${all[@]}"; do vol_of "$n"; done; cat "$orphans" 2>/dev/null || true; } | sort -u | grep -Fxf <(printf '%s\n' "$volumes") || true)
	echo "removed the wall and its tiles: $((there - ${#cleft[@]})) containers, $((vthere - ${#vleft[@]})) volumes"
	[ ${#cleft[@]} -eq 0 ] || echo "not removed: ${cleft[*]} (vmx container remove NAME says why)"
	# Those kept for the wall made next, which takes them away first.
	rm -rf "$busy" "$book" "$failed" "$vols" "$orphans" "$imgs"
	if [ ${#vleft[@]} -gt 0 ]; then
		printf '%s\n' "${vleft[@]}" >"$orphans"
		echo "volumes not removed: ${vleft[*]} (vmx container volume-remove NAME says why); the wall made next takes them away first"
	fi
	if [ -n "$agent" ] && [ -n "$wall" ]; then
		echo "The images stay: vmx container image-remove takes each away: $agent${next:+, $next}, $wall."
	else
		echo "The images stay: vmx container list --json names them, and vmx container image-remove takes one away."
	fi
}

if [ "${1:-}" = --remove ]; then
	[ $# -eq 1 ] || die "usage: ${0##*/} --remove"
	if [ ! -f "$made" ]; then
		by_another "nothing to remove here ($here made no wall)"
		die "nothing to remove: $made is not there"
	fi
	# The keeper first, and what it has in hand with it.
	if systemctl is-active --quiet "$keeper_unit"; then
		systemctl stop "$keeper_unit" && echo "stopped the keeper ($keeper_unit)"
	fi
	remove_all
	exit 0
fi

# keeper_start: the keeper as a service of its own, so no terminal has to
# stay open for it; what it does goes to the journal. One keeper at a time:
# two would each do the same press.
keeper_start() {
	if systemctl is-active --quiet "$keeper_unit"; then
		echo "The keeper runs already: journalctl -fu $keeper_unit shows what it does."
		return 0
	fi
	if pgrep -f -- '05-demoscene-wall\.sh --keep-here$' >/dev/null; then
		echo "A keeper runs in a terminal (--keep-here): it keeps the wall; Ctrl+C there, then ${0##*/} --keep, makes it a service."
		return 0
	fi
	systemd-run --quiet --collect --unit="$keeper_unit" --description="the demoscene wall's keeper, $here" \
		"$here/${0##*/}" --keep-here
	echo "The keeper runs as the service $keeper_unit: it does what the page's buttons ask, and runs again a tile that is gone."
	echo "   what it does:  journalctl -fu $keeper_unit"
	echo "   stop it:       systemctl stop $keeper_unit   (the wall stays; nothing the page asks is done until it starts again)"
}

if [ "${1:-}" = --keep ]; then
	[ $# -eq 1 ] || die "usage: ${0##*/} --keep"
	[ -f "$made" ] && [ -f "$kept" ] || die "no wall to keep: ${0##*/} makes one first"
	keeper_start
	exit 0
fi

# drive WHAT: what --show and --demo share: the wall that is there, its
# address and its tiles, and the page's buttons pressed as the page presses
# them (POST /do/VERB with the tiles' numbers), each waited out until the
# keeper has done it.
drive() {
	[ -f "$made" ] && [ -f "$kept" ] || die "no wall to $1: ${0##*/} makes one first"
	at=$(fact at)
	n=$(fact tiles)
	[[ $n =~ ^[0-9]+$ ]] && [ -n "$at" ] || die "$kept does not say the wall's address and how many tiles"
	url=http://$at:8090
	systemctl is-active --quiet "$keeper_unit" || pgrep -f -- '05-demoscene-wall\.sh --keep-here$' >/dev/null \
		|| die "no keeper runs, and nothing pressed would be done: ${0##*/} --keep first"
	mapfile -t every < <(seq 0 $((n - 1)))
}
# state: the wall as the page sees it. Five seconds at most: a host drawing
# 160 tiles answered past two (2026-11-19).
state() { curl -sf --max-time 5 "$url/state"; }
step() { printf '%s  %s\n' "$(date +%T)" "$*"; }
tile_name() { printf 'tile-%02d' "$1"; }
# since NANOSECONDS: how long ago that was, in seconds and tenths.
since() {
	local ms=$((($(date +%s%N) - $1) / 1000000))
	printf '%d.%d' $((ms / 1000)) $((ms % 1000 / 100))
}
# press VERB [N] TILE...: a button, as the page presses it.
press() {
	local what=$1 q=
	shift
	if [ "$what" = clone ]; then q="?n=$1"; shift; fi
	curl -sf -o /dev/null --max-time 2 -X POST --data-binary "[$(IFS=,; echo "$*")]" "$url/do/$what$q" || die "the wall did not take $what"
}
# settle SECS: until no tile has a verb in hand or waiting, and every
# place with a tile draws, SECS at most.
settle() {
	for _ in $(seq 1 $(($1 * 5))); do
		state | jq -e '[.tiles[] | select(.act or .asked or (.v > 0 and .age_ms > 2000))] | length == 0' >/dev/null && return 0
		sleep 0.2
	done
	step "  still busy after $1 s: on to the next step"
}
# gens: how many microVMs each place has had, by its number.
gens() { state | jq -c '[.tiles[].gen]'; }
# drawn GENS SECS TILE...: whether each tile named has drawn as a new boot
# since GENS was read (its place has had one more microVM), within SECS.
drawn() {
	local g=$1 secs=$2 list
	shift 2
	list="[$(IFS=,; echo "$*")]"
	for _ in $(seq 1 $((secs * 5))); do
		state | jq -e --argjson g "$g" --argjson t "$list" '. as $s | all($t[]; ($s.tiles[.].gen // 0) > ($g[.] // 0) and $s.tiles[.].v > 0 and $s.tiles[.].age_ms <= 2000)' >/dev/null && return 0
		sleep 0.2
	done
	return 1
}
# back GENS TILE...: until each tile named has drawn as a new boot since
# GENS was read, 60 s at most: a killed tile is started again by the
# keeper, which the show waits out, so the next step does not start on a
# tile still down. 2026-11-18: the Kill step moved on in 5 s, its eight
# still down for the Restore.
back() {
	local g=$1
	shift
	drawn "$g" 60 "$@" || step "  not every tile killed was back after 60 s: on to the next step"
}
# reset: the wall made again, waited out until it is whole.
reset() {
	curl -sf -o /dev/null --max-time 2 -X POST "$url/reset" || die "the wall did not take the reset"
	for _ in $(seq 1 25); do [ "$(state | jq -r '.reset')" != "" ] && break; sleep 0.2; done
	for _ in $(seq 1 600); do [ "$(state | jq -r '.reset')" = "" ] && break; sleep 0.2; done
	step "  the wall is whole: $(state | jq -r '"\(.full_n) tiles up in \(.full_s * 10 | floor / 10) s"')"
}

# --show: the wall's own show, for a recording: the same take every time,
# on the wall that is there, pressed as the page presses its buttons, each
# step waited out until the keeper has done it, with the same pauses
# between. Open the page first: what it shows is what is recorded.
if [ "${1:-}" = --show ]; then
	[ $# -eq 1 ] || die "usage: ${0##*/} --show"
	drive show
	some=()
	for k in 2 9 13 20 27 30 34 38; do [ "$k" -lt "$n" ] && some+=("$k"); done
	step "the show, on $url ($n tiles); Ctrl+C ends it, and the wall stays as it is"
	step "Reset the wall";                reset; sleep 4
	step "Snapshot every tile";           press snapshot "${every[@]}"; settle 60; sleep 4
	step "Kill ${#some[@]} tiles outright";  g=$(gens) || die "the wall did not say how it is"; press kill "${some[@]}"; back "$g" "${some[@]}"; settle 60; sleep 4
	step "Restore every tile";            press restore "${every[@]}"; settle 120; sleep 4
	step "Rebuild ${#some[@]} tiles: a new disk each, the same volume"; press rebuild "${some[@]}"; settle 120; sleep 6
	step "Update every tile: green rolls over the wall"; press update "${every[@]}"; settle 240; sleep 6
	step "Roll back every tile: blue rolls back over it"; press rollback "${every[@]}"; settle 240; sleep 4
	step "Clone tile 0, twelve times";    press clone 12 0; settle 180; sleep 6
	step "Reset the wall";                reset
	step "the show is over"
	exit 0
fi

# --demo: the wall explained from one container up, for a recording to cut
# from. Each step is done as an operator would do it: the keeper stopped
# and started with systemctl, the tile removed with vmx by hand, the rest
# pressed as the page presses its buttons; and what each one shows is said
# under the picture on every page open on the wall (POST /demo), with the
# numbers this run measured, the page framing the tiles it is about. Open
# the page first, full screen or in a window of the shape to record
# (1:1, 4:5 or 16:9: the page lays itself out for it). HOLD (7) is the
# seconds each line stays. It leaves the wall whole, with its keeper.
if [ "${1:-}" = --demo ]; then
	[ $# -eq 1 ] || die "usage: ${0##*/} --demo"
	# HOLD read before anything is made: a wrong one refused the demo only
	# after the wall had been made for it (2026-11-19, HOLD=1).
	hold=${HOLD:-7}
	[[ $hold =~ ^[0-9]+$ ]] && [ "$hold" -ge 2 ] && [ "$hold" -le 60 ] || die "HOLD is the seconds a line stays, 2 to 60: not $hold"
	# One demo at a time: a second, started while one ran, pressed buttons
	# and said its lines beside it, and every page showed both (2026-11-20:
	# a take was recorded over a run already going). The lock is this run's
	# until it ends, however it ends; the file says whose it is. Opened to
	# append, so that a second run does not empty it before it is refused.
	mkdir -p "$(dirname "$made")"
	demo_lock=$here/.made/05-demoscene-wall.demo
	exec 7>>"$demo_lock"
	flock -n 7 || die "a demo runs on this wall already ($(cat "$demo_lock" 2>/dev/null || echo "its run unknown")): wait for it, or Ctrl+C it"
	echo "pid $$, started $(date +%T)" >"$demo_lock"
	# No wall here yet: made first, as this script with no argument makes
	# it (the wall and its page alone, no demo, is that default).
	if [ ! -f "$made" ] || [ ! -f "$kept" ]; then
		echo "No wall here yet: it is made first."
		"$here/${0##*/}" || die "the wall was not made, and there is nothing to show"
	fi
	drive demo
	systemctl is-active --quiet "$keeper_unit" || die "the keeper runs in a terminal, not as the service $keeper_unit: Ctrl+C there, then ${0##*/} --keep (the demo stops it and starts it again)"
	doc=$(state) || die "the wall at $url does not answer"
	cols=$(jq -r '.cols // empty' <<<"$doc")
	rows=$(jq -r '.rows // empty' <<<"$doc")
	[[ $cols =~ ^[0-9]+$ ]] && [[ $rows =~ ^[0-9]+$ ]] || die "this wall is older than --demo: ${0##*/} --remove, then make it again"
	[ "$cols" -ge 3 ] && [ "$rows" -ge 3 ] || die "the demo wants a wall of 3 x 3 tiles at least, not $cols x $rows"
	# The places: the one the demo starts from (the middle: tile-19 of 8 x
	# 5), the three it puts beside it, the rest; eight to kill and eight to
	# rebuild, spread over the lower half, where most scenes are brightest.
	mid_row=$(((rows - 1) / 2))
	mid_col=$(((cols - 1) / 2))
	h=$((mid_row * cols + mid_col))
	hname=$(tile_name "$h")
	trio=($((h + 1)) $((h + cols)) $((h + cols + 1)))
	rest=()
	for k in "${every[@]}"; do [ "$k" -ne "$h" ] && [ "$k" -ne "${trio[0]}" ] && [ "$k" -ne "${trio[1]}" ] && [ "$k" -ne "${trio[2]}" ] && rest+=("$k"); done
	low_row=$((rows / 2))
	low=$((low_row * cols))
	eight=() other8=()
	for ((k = low + 1; k < n && ${#eight[@]} < 8; k += 3)); do [ "$k" -ne "$h" ] && eight+=("$k"); done
	for ((k = low + 2; k < n && ${#other8[@]} < 8; k += 3)); do [ "$k" -ne "$h" ] && other8+=("$k"); done
	# word N: N in words up to twelve, else in figures; Word, capitalised.
	word() { local w=(no one two three four five six seven eight nine ten eleven twelve); if [ "$1" -lt ${#w[@]} ]; then echo "${w[$1]}"; else echo "$1"; fi; }
	Word() { local s; s=$(word "$1"); echo "${s^}"; }
	csv() { local IFS=,; echo "$*"; }
	# bytes N: N bytes in the units ZFS counts in, as the page writes them.
	bytes() { awk -v b="$1" 'BEGIN { split("B KiB MiB GiB TiB", u, " "); k = 1; while (b >= 1024 && k < 5) { b /= 1024; k++ } printf ((k > 1 && b < 10) ? "%.1f %s" : "%.0f %s"), b, u[k] }'; }
	# marks NAME...: the marks named, laid over the one place framed, with
	# their words, as JSON.
	marks() {
		jq -cn --arg img "${agent##*/}" --arg vol "$(vol_of "$hname")" '
			{container: "container", vm: "Firecracker microVM · its own kernel",
			 golden: ("disk: a ZFS clone of the golden of " + $img), volume: ("volume " + $vol + " · ZFS · outlives the machine"),
			 counts: "frames drawn: on its disk, then on its volume"} as $all
			| [$ARGS.positional[] | {(.): $all[.]}] | add' --args "$@"
	}
	# tell BIG SMALL [FOCUS [MARKS [CARD]]]: what every page open on the
	# wall shows: BIG, and SMALL under it; FOCUS the places framed, comma
	# separated (none: the whole wall); MARKS laid over the one framed; CARD
	# a card beside it (the wall's demoCard, as JSON). With plain=true the
	# tiles' names and counts are put away (the wordmark's title and the
	# end), the numbers being what the rest is about.
	plain=false
	tell() {
		local body
		body=$(jq -cn --arg b "$1" --arg s "$2" --arg f "${3:-}" --argjson m "${4:-null}" --argjson p "$plain" --argjson c "${5:-null}" \
			'{big: $b, small: $s, focus: ($f | split(",") | map(select(. != "") | tonumber)), marks: $m, plain: $p, card: $c}') || die "the demo's words did not make JSON: $1"
		curl -sf -o /dev/null --max-time 2 -X POST --data-binary "$body" "$url/demo" || step "  the wall did not take the line: $1"
		step "$1  |  $2"
	}
	# job_card KIND TARGET: the card of the newest vmx job of KIND on TARGET
	# that ended well, its steps as vmx ran them, each with how long it took
	# and what it does; nothing when vmx's list has none (it keeps its last
	# 256 jobs, and a busy host moves past one in a minute).
	# shellcheck disable=SC2016 # $kind, $target and $j are jq's, not the shell's
	job_jq='
		def ts: (.[0:19] + "Z" | fromdateiso8601) + ((.[19:] | capture("^\\.(?<f>[0-9]+)") // {f: "0"}) | "0." + .f | tonumber);
		def gloss: {
			"pool.room": ["the pool under its limit", "pool"],
			"clone.snapshot": ["its disk and volumes, one moment", "disk"],
			"microvm.dataset": ["a dataset for its disk", "disk"],
			"microvm.clone": ["its disk, a ZFS clone", "disk"],
			"microvm.record": ["its record: uid, MAC, tap", "record"],
			"microvm.tap": ["its own network tap", "vm"],
			"microvm.jail": ["its jail, read only", "vm"],
			"microvm.run": ["Firecracker starts it", "vm"],
			"microvm.intent": ["started with the host", "record"],
			"microvm.halt": ["asked to shut down, stopped if it will not", "vm"],
			"microvm.untap": ["its tap taken away", "vm"],
			"microvm.unjail": ["its jail taken away", "vm"],
			"microvm.destroy": ["its dataset and its disk destroyed", "disk"],
			"microvm.forget": ["its record and seed removed", "record"],
			"container.volumes": ["where its volumes live", "volume"],
			"container.forwards": ["its ports, from the host", "vm"]
		};
		def note_of($n): if ($n | startswith("clone.volume.")) then "its volume, a ZFS clone"
			elif ($n | startswith("container.volume.")) then "its volume, made for it" else (gloss[$n][0] // "") end;
		def tone_of($n): if ($n | startswith("clone.volume.")) or ($n | startswith("container.volume.")) then "volume" else (gloss[$n][1] // "") end;
		first(.jobs[] | select(.kind == $kind and .target == $target and .state == "done")) as $j
		| [$j.log[] | select(.text | test("^\\[[0-9]+/[0-9]+\\] [^ ]+: (started|done)$"))
			| {n: (.text | capture("^\\[[0-9]+/[0-9]+\\] (?<n>[^ ]+): ").n), w: (.text | capture(": (?<w>started|done)$").w), at: (.at | ts)}] as $ev
		| [$ev | group_by(.n)[] | {n: .[0].n, s: (map(select(.w == "started"))[0].at), e: (map(select(.w == "done"))[0].at)} | select(.s != null and .e != null)] | sort_by(.s) as $steps
		| {kind: "job",
			title: ("job " + $j.id[0:8] + " · " + $j.kind + " · " + $j.target)[0:120],
			rows: [$steps[0:12][] | {key: .n[0:48], note: note_of(.n), tone: tone_of(.n), secs: ((.e - .s) * 1000 | round / 1000)}],
			foot: ("done in " + ((($j.finished | ts) - ($j.started | ts)) * 100 | round / 100 | tostring) + " s · a plan first, each step checked; a step that fails undoes the ones before it")}'
	job_card() {
		# No card is the one harmless end: vmx not answering, or the job gone
		# from its list; the line is said without it.
		vmx jobs --json 2>/dev/null | jq -c --arg kind "$1" --arg target "$2" "$job_jq" 2>/dev/null || true
	}
	# pipe_card NAME: the stages from the image to NAME's microVM, each as
	# vmx and ZFS hold it now: the image, its digest and size; the golden
	# NAME's disk is a ZFS clone of, and what it holds; NAME's own disk and
	# what NAME wrote to it; NAME's microVM. Nothing when one is not read.
	pipe_card() {
		local img disk origin used gold
		img=$(vmx container images --json 2>/dev/null | jq -c --arg i "${agent##*/}" '[.[] | select((.image | split("/") | last) == $i)][0] // empty' 2>/dev/null) || img=
		IFS=$'\t' read -r disk origin used < <(zfs list -Hp -o name,origin,used -t volume 2>/dev/null | awk -F '\t' -v n="$1" '$1 ~ ("/vms/" n "/root$") { print; exit }') || disk=
		[ -n "$img" ] && [ -n "$disk" ] && [[ $origin == *@* ]] || return 0
		gold=$(zfs list -Hp -o referenced "$origin" 2>/dev/null) || return 0
		jq -cn --argjson img "$img" --arg disk "$disk" --arg origin "$origin" --arg used "$(bytes "$used")" --arg gold "$(bytes "$gold")" --arg size "$(bytes "$(jq -r '.size_bytes' <<<"$img")")" --arg name "$1" '
			{kind: "pipe", rows: [
				{key: "OCI image", value: ($img.image | split("/") | last), note: ("the same image Docker runs · " + $img.digest[7:19] + " · " + $size), via: "imported once, every blob checked against its digest", tone: "image"},
				{key: "ZFS golden", value: $origin, note: ("made once, read only: its layers one ext4, on a zvol, " + $gold), via: "zfs clone, one for each tile", tone: "disk"},
				{key: "ZFS clone", value: $disk, note: ("this tile'"'"'s own disk: it holds what the tile wrote, " + $used), via: "booted", tone: "disk"},
				{key: "Firecracker microVM", value: $name, note: "its own kernel, in its own jail, on its own tap", tone: "vm"}]}'
	}
	# said_like I WORDS SECS: whether place I's tile says something that
	# starts with WORDS (what the keeper said of its last verb), within SECS.
	said_like() {
		for _ in $(seq 1 $(($3 * 5))); do
			state | jq -e --argjson i "$1" --arg w "$2" '(.tiles[$i].said // "") | startswith($w)' >/dev/null && return 0
			sleep 0.2
		done
		return 1
	}
	# roll VERB LOOK BIG: VERB pressed for every tile of the picture, and
	# the line under BIG counting the tiles that draw in LOOK, until all do
	# (took: how long that took), 240 s at most.
	roll() {
		local k=-1 c s0
		s0=$(date +%s%N)
		press "$1" "${every[@]}"
		for _ in $(seq 1 480); do
			c=$(state | jq --arg l "$2" --argjson n "$n" '[.tiles[:$n][] | select(.look == $l and .v > 0 and .age_ms <= 2000)] | length') || c=$k
			if [ "$c" != "$k" ]; then
				k=$c
				tell "$3" "eight machines at a time, while the rest keep drawing · $k of $n $2"
			fi
			if [ "$k" -ge "$n" ]; then took=$(since "$s0"); return 0; fi
			sleep 0.5
		done
		return 1
	}
	# The end, however it comes: the pages back to the wall, and the
	# keeper running again if the demo had stopped it.
	# shellcheck disable=SC2329 # called by the trap on EXIT, below
	demo_end() {
		curl -s -o /dev/null --max-time 2 -X DELETE "$url/demo" || true
		systemctl is-active --quiet "$keeper_unit" || keeper_start >/dev/null
	}
	trap demo_end EXIT
	trap 'exit 130' INT TERM
	took=
	step "the demo, on $url: $n tiles; it starts from $hname. Open the page first."

	# Made ready: the wall made again, drawing the vmx intro (the wordmark
	# landing out of falling bits, zettarack.com under it) under the title.
	plain=true
	tell "One picture. $n machines. One box." "a demo of vmx · the wall is being made ready"
	reset
	curl -sf -o /dev/null --max-time 2 -X POST "$url/show/vmx" || step "  the show did not go to the vmx intro"
	tell "One picture. $n machines. One box." "a demo of vmx"
	# The title holds two lines' time, room to start a recording; nothing
	# waits for a key (2026-11-19: "get rid of that").
	sleep $((hold * 2))

	# Down to one: every tile but one taken away, and space on the picture.
	plain=false
	tell "Start with one." "every tile but $hname taken away"
	press remove "${rest[@]}" "${trio[@]}"
	for _ in $(seq 1 600); do
		state | jq -e --argjson h "$h" '.tiles | to_entries | all(.key == $h or ((.value.act // "") == "" and (.value.v == 0 or .value.age_ms > 2000)))' >/dev/null && break
		sleep 0.2
	done
	curl -sf -o /dev/null --max-time 2 -X POST "$url/show/space" || step "  the show did not go to space"
	tell "Start with one." "every tile but $hname taken away" "$h"
	sleep 3

	# One container, each part of it.
	tell "This is a container." "one small program, the same image for every tile, drawing one square of a picture" "$h" "$(marks container)"
	sleep "$hold"
	up=$(state | jq -r --argjson h "$h" '.tiles[$h].up_ms // 0')
	if [ "$up" -gt 0 ]; then line="its own Linux kernel, which had the program running $(awk -v m="$up" 'BEGIN { printf "%.2f", m / 1000 }') s after it started"; else line="its own Linux kernel, jailed"; fi
	tell "It runs in its own Firecracker microVM." "$line" "$h" "$(marks container vm)"
	sleep "$hold"
	own=$(state | jq -r --argjson h "$h" '.tiles[$h].own // 0')
	if [ "$own" -gt 0 ]; then line="the image is unpacked once, as a dataset; this disk holds only what it wrote: $(bytes "$own")"; else line="the image is unpacked once, as a dataset; this disk holds only what it wrote"; fi
	tell "Its disk is a ZFS clone of a golden image." "$line" "$h" "$(marks container vm golden)"
	sleep "$hold"
	tell "Its data is on a ZFS volume of its own." "a volume outlives the machine it is given to" "$h" "$(marks container vm golden volume)"
	sleep "$hold"
	tell "It counts every frame it draws, twice." "once on its disk, once on its volume: watch what each one survives" "$h" "$(marks container vm golden volume counts)"
	sleep "$hold"
	tell "No container engine on the host." "no Docker, no containerd: vmx runs the image in a microVM; the volume is plain ZFS, no storage plugin" "$h" "$(marks container vm golden volume counts)"
	sleep "$hold"
	# From the image to the machine, each stage as vmx and ZFS hold it.
	c=$(pipe_card "$hname")
	if [ -n "$c" ]; then
		tell "One container: an OCI image, on ZFS, in a Firecracker microVM." "the image Docker runs, made a ZFS golden once; this tile's disk a clone of it, booted with its own kernel" "$h" "$(marks container vm golden volume)" "$c"
		sleep "$hold"
	else
		step "  no card: the image or $hname's disk was not read"
	fi

	# Deleted with nobody watching; the keeper; deleted again.
	tell "Stop the keeper, and delete it." "\$ systemctl stop $keeper_unit; vmx container remove $hname" "$h"
	systemctl stop "$keeper_unit" || die "the keeper did not stop"
	vmx container remove "$hname" >/dev/null || die "vmx did not remove $hname"
	for _ in $(seq 1 50); do state | jq -e --argjson h "$h" '.tiles[$h].age_ms > 2000' >/dev/null && break; sleep 0.2; done
	tell "Gone, and nothing brings it back." "nothing is watching this wall: the machine and its disk are gone; its volume stays" "$h"
	sleep "$hold"
	tell "Start the keeper." "\$ systemctl start $keeper_unit: a small loop on this host that keeps the wall as it should be" "$h"
	g=$(gens) || die "the wall did not say how it is"
	s0=$(date +%s%N)
	keeper_start >/dev/null
	if drawn "$g" 60 "$h"; then line="a new machine in $(since "$s0") s: its disk's count started again, its volume's went on"; else line="the keeper runs it again"; fi
	tell "Back, on the same volume." "$line" "$h" "$(marks counts)"
	sleep "$hold"
	tell "Delete it again." "\$ vmx container remove $hname" "$h"
	g=$(gens) || die "the wall did not say how it is"
	s0=$(date +%s%N)
	vmx container remove "$hname" >/dev/null || die "vmx did not remove $hname"
	if drawn "$g" 60 "$h"; then
		tell "Back by itself, in $(since "$s0") s." "nothing pressed: whatever goes, the keeper runs again" "$h"
	else
		tell "Back by itself." "nothing pressed: whatever goes, the keeper runs again" "$h"
	fi
	sleep "$hold"
	# The job the removal was, as vmx ran it. (The keeper's run back is no
	# job of vmxd's to show: vmx container run carries out its plan in the
	# command's own process, and vmx jobs does not list it.)
	c=$(job_card "machine.container remove" "$hname")
	if [ -n "$c" ]; then
		tell "Every change is a job." "vmx container remove $hname, as vmx ran it: $(jq '.rows | length' <<<"$c") steps, each checked" "$h" "" "$c"
		sleep "$hold"
	else
		step "  no card: vmx jobs lists no removal of $hname that ended"
	fi

	# A snapshot, and back to it.
	tell "Snapshot it." "its disk and its volume, at one moment, in one ZFS call" "$h" "$(marks counts)"
	s0=$(date +%s%N)
	press snapshot "$h"
	if said_like "$h" "snapshot at" 30; then tell "Snapshot it." "taken in $(since "$s0") s; it holds nothing until something changes" "$h" "$(marks counts)"; fi
	sleep "$hold"
	tell "Let it run on." "both counts climb past the snapshot" "$h" "$(marks counts)"
	sleep "$hold"
	tell "Restore it." "both counts go back to the snapshot" "$h" "$(marks counts)"
	press restore "$h"
	if said_like "$h" "back to" 30; then tell "Restore it." "$(state | jq -r --argjson h "$h" '.tiles[$h].said'): its disk and its volume, together" "$h" "$(marks counts)"; fi
	sleep "$hold"

	# More from the same golden; the keeper runs the rest.
	tell "$(Word ${#trio[@]}) more, from the same golden image." "each a ZFS clone of it: nothing copied" "$(csv "$h" "${trio[@]}")"
	g=$(gens) || die "the wall did not say how it is"
	s0=$(date +%s%N)
	press rebuild "${trio[@]}"
	if drawn "$g" 60 "${trio[@]}"; then tell "$(Word ${#trio[@]}) more, from the same golden image." "$(word ${#trio[@]}) new microVMs drawing in $(since "$s0") s, each a machine of its own" "$(csv "$h" "${trio[@]}")"; fi
	sleep "$hold"
	tell "The keeper does the rest." "the page asks, the keeper runs vmx on this host: ${#rest[@]} more, at once"
	g=$(gens) || die "the wall did not say how it is"
	s0=$(date +%s%N)
	press rebuild "${rest[@]}"
	if drawn "$g" 120 "${rest[@]}"; then tell "The keeper does the rest." "${#rest[@]} more drawing in $(since "$s0") s: $n microVMs on one box"; fi
	sleep "$hold"

	# Scale: every tile cloned once, then every one of those once again,
	# each clone a place of its own below the picture, the page framing the
	# whole wall as it grows. What the host could not run (its memory, its
	# network's addresses) is said by the count, not hidden.
	for big in "Double it." "And again."; do
		have=$(state | jq '.tiles | length') || die "the wall did not say how it is"
		mapfile -t all < <(seq 0 $((have - 1)))
		# The page counts the new places itself as they come: asked, in hand,
		# done.
		q=$(jq -cn --argjson f "$have" '{kind: "queue", title: "the new places, as vmx works", from: $f, of: (2 * $f), foot: "each one a vmx job; a job holds what it touches, so two changes to one machine never run at once"}')
		tell "$big" "every tile cloned once: $have more microVMs, their disks and volumes ZFS clones, nothing copied" "" "" "$q"
		s0=$(date +%s%N)
		press clone 1 "${all[@]}"
		# Until every new place draws, or nothing has been in hand for three
		# seconds (what was made is all), ten minutes at most, by the clock:
		# a host drawing a hundred tiles makes clones a dozen a minute or
		# so, and a count of passes ran out first (2026-11-19: 68 of 80
		# made at 246 s, the rest still in hand, taken by the reset after).
		end=$(($(date +%s) + 600)) idle_at=
		while [ "$(date +%s)" -lt "$end" ]; do
			doc=$(state) || { sleep 0.5; continue; }
			[ "$(jq --argjson h "$have" '[.tiles[$h:][] | select(.v > 0 and .age_ms <= 2000)] | length' <<<"$doc")" -ge "$have" ] && break
			if jq -e '[.tiles[] | select(.act or .asked)] | length == 0' <<<"$doc" >/dev/null; then
				idle_at=${idle_at:-$(date +%s)}
				[ $(($(date +%s) - idle_at)) -ge 3 ] && break
			else
				idle_at=
			fi
			sleep 0.5
		done
		took=$(since "$s0")
		doc=$(state) || doc='{}'
		made_n=$(jq --argjson h "$have" '[.tiles[$h:][] | select(.v > 0 and .age_ms <= 2000)] | length' <<<"$doc")
		tell "$big" "$(jq '.live' <<<"$doc") microVMs on one box: $made_n new ones drawing $took s after the press" "" "" "$q"
		sleep "$hold"
	done
	# What the host holds, as the keeper last read it, its own count of
	# the microVMs with its numbers: thirty seconds at most for a reading
	# of every tile drawing. 2026-11-19: the count was taken from a read
	# of the wall that a host drawing 160 tiles answered too late, and the
	# line said "0 disks" over the right sizes; a count not read is not
	# said.
	line="every image, machine, container and volume: a dataset of its own"
	end=$(($(date +%s) + 30))
	while [ "$(date +%s)" -lt "$end" ]; do
		doc=$(state) || { sleep 1; continue; }
		read -r live tiles copies disk < <(jq -r '"\(.live // 0) \(.host.tiles // 0) \(.host.copies // 0) \(.host.disk // 0)"' <<<"$doc")
		if [ "$tiles" -gt 0 ] && [ "$copies" -gt 0 ] && [ "$disk" -gt 0 ]; then
			line="$tiles disks: $(bytes "$copies") as full copies would take them, $(bytes "$disk") as ZFS keeps them"
			[ "$tiles" -ge "$live" ] && break
		fi
		sleep 1
	done
	tell "Everything is a dataset." "$line"
	sleep "$hold"
	tell "Throw it all away." "every tile, clone and volume, and $n new machines from the image"
	reset
	tell "Throw it all away." "all $n drawing again $(state | jq -r '.full_s * 10 | floor / 10') s after the press"
	sleep "$hold"

	# A machine dies: back, and nothing else stops.
	tell "Pull the plug on $(word ${#eight[@]})." "each one killed outright, as a power cut would"
	g=$(gens) || die "the wall did not say how it is"
	s0=$(date +%s%N)
	press kill "${eight[@]}"
	if drawn "$g" 90 "${eight[@]}"; then tell "Back in seconds. The rest never stopped." "all $(word ${#eight[@]}) back in $(since "$s0") s; a kernel each, so one crash cannot reach the others"; fi
	sleep "$hold"

	# Blue and green.
	tell "What if there were a new version?" "the same program, drawing in green: the wall's other image, ${next##*/}"
	sleep "$hold"
	if roll update green "Roll it out."; then tell "Roll it out." "$n of $n green in $took s: every machine new, every volume carried over"; fi
	sleep "$hold"
	if roll rollback blue "Changed your mind? Roll back."; then tell "Changed your mind? Roll back." "$n of $n blue again in $took s: the old program, the same data"; fi
	sleep "$hold"
	tell "Two ways back." "Roll back changes the program and keeps the data; Restore puts the data back and keeps the program"
	sleep "$hold"

	# Destroyed and built again; clones.
	tell "Destroy $(word ${#other8[@]}), and build them again." "a new machine and a new disk each, on the same volume"
	g=$(gens) || die "the wall did not say how it is"
	s0=$(date +%s%N)
	press rebuild "${other8[@]}"
	if drawn "$g" 90 "${other8[@]}"; then tell "Destroy $(word ${#other8[@]}), and build them again." "rebuilt in $(since "$s0") s: their disks' counts from nothing, their volumes' carried on"; fi
	sleep "$hold"
	tell "Clone one, $(word "$cols") times." "ZFS clones of its disk and its volume: nothing copied, no space until they write"
	s0=$(date +%s%N)
	press clone "$cols" "$h"
	for _ in $(seq 1 900); do
		state | jq -e --argjson n "$n" --argjson k "$cols" '(.tiles | length) >= ($n + $k) and all(.tiles[$n:($n + $k)][]; .v > 0 and .age_ms <= 2000)' >/dev/null && {
			tell "Clone one, $(word "$cols") times." "$(word "$cols") clones drawing in $(since "$s0") s, each counting on from where $hname was"
			break
		}
		sleep 0.2
	done
	sleep "$hold"
	# One of those clones, as vmx ran it.
	c=$(job_card "machine.container clone" "$hname")
	if [ -n "$c" ]; then
		tell "A clone is a job too." "vmx container clone $hname, as vmx ran it: $(jq '.rows | length' <<<"$c") steps, nothing copied" "" "" "$c"
		sleep "$hold"
	else
		step "  no card: vmx jobs lists no clone of $hname that ended"
	fi

	# And the rest of it.
	tell "Kubernetes? Yes." "it runs in machines on vmx; kbx, in alpha, builds the nodes for you"
	sleep "$hold"
	tell "And the host rolls back too." "every upgrade is a new boot environment; going back is choosing the old one"
	sleep "$hold"
	# The host's own screen, its keys as CONSOLES.md lists them.
	tell "And on the host's own screen: jewm." "every machine at keyboard speed, through the same jobs as the API, the command and the text console" "" "" \
		'{"kind":"keys","title":"jewm, the desktop","rows":[{"key":"Super","note":"every machine at once: the fly-through"},{"key":"Super+Tab","note":"the wall of every machine"},{"key":"w","note":"a machine'"'"'s screen, full size, with its sound"},{"key":"Super+Escape","note":"every key to the machine shown, its own Alt+Tab too"},{"key":"Super+v","note":"the clipboard into the machine"}]}'
	sleep "$hold"
	curl -sf -o /dev/null --max-time 2 -X POST "$url/show/vmx" || step "  the show did not go to the vmx intro"
	plain=true
	tell "vmx" "virtual machines, microVMs and containers, on ZFS"
	# The end card stays half a minute, then the pages go back to the wall.
	sleep 30
	exit 0
fi

if [ "${1:-}" = --keep-here ]; then
	[ $# -eq 1 ] || die "usage: ${0##*/} --keep-here"
	[ -f "$made" ] && [ -f "$kept" ] || die "no wall to keep: ${0##*/} makes one first"
	# The service runs this too; anyone else waits for it to stop.
	if ! grep -q "/$keeper_unit\.service\$" /proc/self/cgroup && systemctl is-active --quiet "$keeper_unit"; then
		die "the keeper runs already as the service $keeper_unit (journalctl -fu $keeper_unit): systemctl stop $keeper_unit first to keep the wall here"
	fi
	agent=$(fact agent)
	# The tiles' second image, the one an Update moves them to; a wall made
	# before there was one has none, and its Update says so.
	next=$(fact next || true)
	at=$(fact at)
	n=$(fact tiles)
	[[ $n =~ ^[0-9]+$ ]] && [ -n "$agent" ] && [ -n "$at" ] || die "$kept does not say the tiles' image, the wall's address and how many tiles"
	url=http://$at:8090
	# No job is in hand as the keeper starts.
	rm -rf "$busy"
	mkdir -p "$busy"

	# listed NAME: this script keeps it.
	listed() { grep -qx -- "$1" "$made" 2>/dev/null; }
	# list_add NAME, list_drop NAME: the list written under its lock, and
	# only while the wall's record is there (--remove takes it first).
	list_add() { (flock 8 && if [ -f "$made" ]; then echo "$1" >>"$made"; fi) 8>"$book"; }
	list_drop() { (flock 8 && if [ -f "$made" ]; then sed -i "/^$1\$/d" "$made"; fi) 8>"$book"; }
	# vol_set NAME VOLUME, vol_drop NAME: the volumes' record written under
	# the list's lock, and only while the wall's record is there.
	vol_set() { (flock 8 && if [ -f "$made" ]; then { grep -v "^$1 " "$vols" 2>/dev/null || true; echo "$1 $2"; } >"$vols.new" && mv "$vols.new" "$vols"; fi) 8>"$book"; }
	vol_drop() { (flock 8 && if [ -f "$vols" ]; then sed -i "/^$1 /d" "$vols"; fi) 8>"$book"; }
	# img_of NAME: the image tile NAME runs, the tiles' first when none is
	# recorded; img_set NAME IMAGE records it (none for the first), under
	# the list's lock and while the wall's record is there.
	img_of() { local i=; if [ -f "$imgs" ]; then i=$(awk -v n="$1" '$1 == n { print $2; exit }' "$imgs"); fi; echo "${i:-$agent}"; }
	img_set() {
		(flock 8 && if [ -f "$made" ]; then
			{ grep -v "^$1 " "$imgs" 2>/dev/null || true; [ "$2" = "$agent" ] || echo "$1 $2"; } >"$imgs.new" && mv "$imgs.new" "$imgs"
		fi) 8>"$book"
	}
	# room: the host keeps 2 GiB of memory; no new microVM is run past it.
	room() { awk '$1 == "MemAvailable:" { ok = $2 >= 2 * 1024 * 1024 } END { exit !ok }' /proc/meminfo; }
	# try ARGS...: vmx ARGS, the last line of what it said in $why when it
	# fails. A new one an older vmx refused because another asked for at
	# the same moment took the uid it planned is asked again, as quiet's.
	why=
	try() {
		local out
		for _ in $(seq 1 64); do
			if out=$(vmx "$@" 2>&1 >/dev/null); then return 0; fi
			case $out in *"run it again"*) ;; *) break ;; esac
		done
		why=$(tail -n 1 <<<"$out")
		why=${why#vmx: }
		[ -n "$why" ] || why="vmx $1 $2 failed and said nothing"
		return 1
	}
	# run_tile NAME: a new microVM in NAME's place, on its volume: the one
	# it counted on before when it is there (its count goes on), a new one
	# when it is not.
	run_tile() {
		try container run "$(img_of "$1")" "$1" --env "TILE=$((10#${1#tile-}))" --env "WALL=$url" \
			--env VOLUME=/data --volume "$(vol_of "$1"):/data" --volume-size "$vol_size" || { echo "   $1: $why" >&2; return 1; }
	}
	# vol_remove NAME: the volume tile NAME counted on taken away, once the
	# tile is (its snapshots on it went with the tile), and its record; it
	# fails, saying why in $why, when vmx would not: a clone's copy of it
	# that still stands, for one.
	vol_remove() {
		local v
		v=$(vol_of "$1")
		if has_vol "$v" && ! try container volume-remove "$v"; then
			# Kept, to be taken away once vmx lets it go (orphan_pass).
			(flock 8 && { grep -qx -- "$v" "$orphans" 2>/dev/null || echo "$v" >>"$orphans"; }) 8>"$book"
			return 1
		fi
		# Taken away: no longer one left behind, if an earlier round could
		# not take it. 2026-11-18: a reset's later round took tile-32's
		# volume, and the record of it stayed, until the next pass said the
		# new tile-32's volume was the old one "a tile left behind".
		if [ -s "$orphans" ]; then
			(flock 8 && { grep -vxF -- "$v" "$orphans" >"$orphans.new" || true; } && mv "$orphans.new" "$orphans") 8>"$book"
		fi
		vol_drop "$1"
	}
	# orphan_pass: each volume a tile left behind taken away once vmx lets
	# it go, and let go of once it is gone, however it went, or once a tile
	# counts on it again (a Reset that could not take it runs its tile
	# again on it).
	orphan_pass() {
		local v held line
		[ -s "$orphans" ] || return 0
		held=$(vmx container volumes --json | jq -r '.[]? | "\(.name) \(.container)"') || return 0
		while read -r v; do
			[ -n "$v" ] || continue
			line=$(awk -v v="$v" '$1 == v { print; exit }' <<<"$held")
			if [ -z "$line" ]; then
				echo "   the volume $v a tile left behind is gone"
			elif [ -n "${line#* }" ]; then
				echo "   the volume $v a tile left behind is ${line#* }'s again"
			elif try container volume-remove "$v"; then
				echo "   the volume $v a tile left behind is taken away"
			else
				continue
			fi
			(flock 8 && sed -i "/^$v\$/d" "$orphans") 8>"$book"
		done < <(cat "$orphans")
	}
	# claim_n K: up to K places after the picture's own that nothing has,
	# taken under the list's lock (on the list, and marked in hand), so
	# that clones made at once have a place each; prints their names, fewer
	# when the wall has fewer left. It fails 2 when vmx's list could not be
	# read, and 1 when the wall was taken away.
	claim_n() {
		(
			flock 8
			[ -f "$made" ] || exit 1
			present=$(vmx container list --json | jq -r '.[].name') || exit 2
			declare -A taken=()
			while read -r x; do [ -z "$x" ] || taken[$x]=1; done < <(cat "$made"; ls -A "$busy"; printf '%s\n' "$present")
			got=0 k=$n
			while [ "$got" -lt "$1" ] && [ "$k" -lt "$most" ]; do
				new=$(tile_name "$k")
				if [ -z "${taken[$new]:-}" ]; then
					: >"$busy/$new"
					echo "$new" >>"$made"
					echo "$new"
					got=$((got + 1))
				fi
				k=$((k + 1))
			done
		) 8>"$book"
	}
	# clock_of LABEL: a snapshot's label, made in UTC, as this host's time of day.
	clock_of() {
		if [[ $1 =~ ^([0-9]{4})([0-9]{2})([0-9]{2})T([0-9]{2})([0-9]{2})([0-9]{2})Z$ ]]; then
			date -d "${BASH_REMATCH[1]}-${BASH_REMATCH[2]}-${BASH_REMATCH[3]}T${BASH_REMATCH[4]}:${BASH_REMATCH[5]}:${BASH_REMATCH[6]}Z" +%H:%M:%S
		else
			echo "@$1"
		fi
	}
	# say I WORDS FAILED: what became of place I's verb, to the wall; FAILED
	# is 1 when it was not done. Three tries, five seconds each: a word the
	# wall never takes leaves the verb to be handed out again and done twice
	# (2026-11-19, a host drawing 130 tiles answered past two seconds), so a
	# word lost after them is said in the journal, and the keeper goes on.
	say() {
		local k
		for k in 1 2 3; do
			curl -sf -o /dev/null --max-time 5 -X POST -H "X-Failed: $3" --data-binary "$2" "$url/did/$1" && return 0
			[ "$k" -eq 3 ] || sleep 1
		done
		echo "   place $1: the wall did not take the keeper's word ($2): it may ask for the verb again" >&2
	}
	# clone_into SOURCE NEW STOP: NEW made a clone of SOURCE in its place
	# on the wall, and its address waited for. One the host's network has
	# no address left for stops in half a minute (vmx-init asks for one
	# that long) and is taken away again: it could never draw. One still
	# without an address after eight seconds marks itself slow
	# (STOP.slow-NEW), and no more clones start until it has one or gives
	# up: past the network's last address every clone would wait its half
	# minute. Each clone made is a line of STOP.ok; the first reason the
	# clones stop is STOP's.
	clone_into() {
		local src=$1 new=$2 stop=$3 i line state addr t v
		i=$((10#${new#tile-}))
		# Its volume will be vmx's copy of its source's, NEW-VOLUME by
		# vmx's own rule: recorded before the clone is asked, so that a
		# keeper stopped while vmx makes it leaves the copy in the record
		# for --remove. Recorded after, a demo stopped mid-clone left the
		# copies standing on their sources' volumes, and the next wall
		# would not start (2026-11-20: tile-52-tile-12 to tile-58-tile-18,
		# "a volume named tile-12 is there already"). Corrected below from
		# vmx's list; a name recorded for no volume is passed over.
		vol_set "$new" "$new-$(vol_of "$src")"
		if ! try container clone "$src" "$new"; then
			list_drop "$new"
			[ -s "$stop" ] || echo "$why" >"$stop"
			rm -f "$busy/$new"
			return 0
		fi
		# Its volume is vmx's copy of its source's, as vmx lists it:
		# recorded, so that a Rebuild of it, or a run again, keeps it.
		line=$(vmx container list --json | jq -r --arg n "$new" '.[] | select(.name == $n) | "\(.image // "")\t\(.volumes // "")"') || line=
		v=${line#*$'\t'}
		v=${v%%,*}
		v=${v%%:*}
		# vmx's list not read (a host drawing a hundred tiles answers late):
		# the copy's name by vmx's own rule, NEW-VOLUME, recorded all the
		# same. Unrecorded, the copy outlived its clone, held its source's
		# volume, and the next wall would not start (2026-11-19:
		# tile-102-tile-22 kept tile-22's volume, "a volume named tile-22
		# is there already"). A name recorded for no volume is passed over
		# when the clone is taken away.
		if [ -z "$v" ] || [ "$v" = "$line" ]; then v="$new-$(vol_of "$src")"; fi
		vol_set "$new" "$v"
		# The version its source runs, which vmx clones with it.
		local img=${line%%$'\t'*}
		if [ -n "$next" ] && [ -n "$img" ] && [ "${img##*/}" = "${next##*/}" ]; then img_set "$new" "$next"; else img_set "$new" "$agent"; fi
		say "$i" "a clone of $src" 0
		addr=
		for t in $(seq 1 45); do
			line=$(vmx container list --json | jq -r --arg n "$new" '.[] | select(.name == $n) | "\(.state) \(.address // "")"') || line=
			state=${line%% *} addr=${line#* }
			[ "$addr" != "$line" ] || addr=
			[ -z "$addr" ] || break
			[ "$state" = running ] || break
			[ "$t" -ne 8 ] || : >"$stop.slow-$new"
			sleep 1
		done
		rm -f "$stop.slow-$new"
		if [ -z "$addr" ]; then
			if try container remove "$new"; then
				vol_remove "$new" || echo "   $new: its volume stays: $why" >&2
			else
				echo "   $new: not taken away: $why" >&2
			fi
			list_drop "$new"
			say "$i" "no address left on this host's network: taken away" 1
			[ -s "$stop" ] || echo "the host's network has no address left" >"$stop"
			rm -f "$busy/$new"
			return 0
		fi
		echo "$new" >>"$stop.ok"
		rm -f "$busy/$new"
	}
	# act_on NAME I VERB [N [AT]]: one of the page's verbs on a tile, done
	# with vmx (N a clone's count, AT the snapshot a restore goes back to, -
	# for the newest); prints what became of it in a few words for the tile
	# (nothing when the tile shows it itself), and fails when it was not
	# done.
	act_on() {
		local name=$1 label s out places=() new want stop rc running pid nope
		if [ "$3" != rebuild ] && ! listed "$name"; then
			if has "$name"; then echo "not a tile this script made: left as it is"; else echo "an empty place: Rebuild puts a tile here"; fi
			return 1
		fi
		case $3 in
		snapshot)
			label=$(date -u +%Y%m%dT%H%M%SZ)
			try container snapshot "$name" "$label" || { echo "no snapshot: $why"; return 1; }
			echo "snapshot at $(clock_of "$label")"
			;;
		restore)
			# The snapshot named (the page's list of the tile's), else the
			# newest a person took, not one a clone was made from; the ones
			# newer than it go with the restore, and are counted first.
			out=$(vmx container snapshots "$name" --json 2>/dev/null | jq -c '[.[]?.name | select(startswith("clone-") | not)]') || out=
			[ -n "$out" ] || { echo "not restored: its snapshots could not be listed"; return 1; }
			label=${5:--}
			if [ "$label" = - ]; then
				label=$(jq -r 'last // empty' <<<"$out")
				[ -n "$label" ] || { echo "no snapshot to restore yet"; return 1; }
			elif ! jq -e --arg l "$label" 'index($l) != null' <<<"$out" >/dev/null; then
				echo "not restored: it has no snapshot @$label"
				return 1
			fi
			want=$(jq -r --arg l "$label" 'length - index($l) - 1' <<<"$out")
			s=$(date +%s%N)
			if ! try container rollback "$name" "$label" --restart; then
				case $why in
				*"was cloned from"*) echo "not restored: a clone stands on a newer snapshot; remove it first" ;;
				*) echo "not restored: $why" ;;
				esac
				return 1
			fi
			if [ "$want" -gt 0 ]; then
				echo "back to $(clock_of "$label") in $(since "$s") s; the $want newer went with it"
			else
				echo "back to $(clock_of "$label") in $(since "$s") s"
			fi
			;;
		rebuild)
			s=$(date +%s%N)
			if listed "$name"; then
				if has "$name" && ! try container remove "$name"; then
					echo "not rebuilt: $why"
					return 1
				fi
				echo "   $name: destroyed in $(since "$s") s; its volume stays, for the new one" >&2
			elif has "$name"; then
				echo "not a tile this script made: left as it is"
				return 1
			else
				room || { echo "not put back: the host keeps its last 2 GiB of memory"; return 1; }
				list_add "$name"
			fi
			s=$(date +%s%N)
			if ! run_tile "$name"; then
				echo "no new one started: the keeper tries again"
				return 1
			fi
			echo "   $name: a new one started in $(since "$s") s" >&2
			;;
		update | rollback)
			# update: the tile moved to the wall's new version, its second
			# image; rollback: back to the first. Its microVM and disk go
			# and a new one is run from that image on the same volume; the
			# keeper's dispatch lets eight at a time do either, so it rolls
			# over the wall while the rest draw. 2026-11-18: Update pressed
			# again rolled back; the roll back is a button of its own now
			# (the operator: "a rollback would be better suited? like a
			# button"), and Update only ever goes forward.
			if [ "$3" = update ]; then want=$next label="updated to" nope="not updated"; else want=$agent label="back to" nope="not rolled back"; fi
			[ -n "$next" ] || { echo "$nope: this wall was made without a second image; --remove and make it again"; return 1; }
			out=$(vmx container list --json | jq -r --arg n "$name" '.[] | select(.name == $n) | .image // empty') || out=
			[ -n "$out" ] || { echo "$nope: it is gone"; return 1; }
			# Already there: nothing to do, and said as not done, so the
			# wall stops waiting for a new machine.
			if [ "${out##*/}" = "${want##*/}" ]; then
				if [ "$3" = update ]; then echo "already on ${want##*/}, the new version"; else echo "nothing to roll back to: it runs ${want##*/}"; fi
				return 1
			fi
			s=$(date +%s%N)
			if ! try container remove "$name"; then
				case $why in
				*clone*) echo "$nope: a clone stands on its disk; remove the clone first" ;;
				*) echo "$nope: $why" ;;
				esac
				return 1
			fi
			img_set "$name" "$want"
			if ! run_tile "$name"; then
				echo "$nope: the new one did not start; the keeper tries again"
				return 1
			fi
			echo "$label ${want##*/} in $(since "$s") s"
			;;
		kill)
			# The machine ended as a crash or a power cut would end it: its
			# firecracker process (the one whose --id is the tile's name)
			# killed outright, with no shutdown. Nothing here starts it
			# again: the keeper's own pass finds it stopped, as it would a
			# machine that died any other way, and starts it with its disk
			# as it was; the tile then says how long it was away.
			pid=$(ps -C firecracker -o pid=,args= | awk -v n="$name" '{for (i = 2; i < NF; i++) if ($i == "--id" && $(i + 1) == n) {print $1; exit}}')
			[ -n "$pid" ] || { echo "not killed: no machine of it runs"; return 1; }
			kill -KILL "$pid" || { echo "not killed: the signal was refused"; return 1; }
			echo "killed outright at $(date +%H:%M:%S): the keeper starts it again"
			;;
		clone)
			has "$name" || { echo "nothing to clone: it is gone"; return 1; }
			want=${4:-3}
			[[ $want =~ ^[0-9]+$ ]] && [ "$want" -ge 1 ] || { echo "not cloned: $want is not a count of clones"; return 1; }
			s=$(date +%s%N) why=
			stop=$busy/.clone-$name
			: >"$stop"
			: >"$stop.ok"
			# The places first, all under one look at the list; then sixteen
			# clones at a time into them. A place not reached once the clones
			# stop (the host's memory, its network, a refusal) is let go.
			rc=0
			out=$(claim_n "$want") || rc=$?
			case $rc in
			0) ;;
			2) echo "not cloned: vmx did not list its containers"; rm -f "$stop" "$stop.ok"; return 1 ;;
			*) echo "not cloned: the wall was taken away"; rm -f "$stop" "$stop.ok"; return 1 ;;
			esac
			mapfile -t places <<<"$out"
			[ -n "$out" ] || places=()
			[ ${#places[@]} -eq "$want" ] || echo "the wall has no place left" >"$stop.short"
			running=0
			for new in "${places[@]}"; do
				if [ "$running" -ge 16 ]; then
					wait -n || true
					running=$((running - 1))
				fi
				# One slow to get its address holds the rest back.
				while compgen -G "$stop.slow-*" >/dev/null; do sleep 0.5; done
				if [ -s "$stop" ] || ! room; then
					[ -s "$stop" ] || echo "the host keeps its last 2 GiB of memory" >"$stop"
					list_drop "$new"
					rm -f "$busy/$new"
					continue
				fi
				clone_into "$name" "$new" "$stop" </dev/null &
				running=$((running + 1))
			done
			wait
			mapfile -t places <"$stop.ok"
			why=$(<"$stop")
			[ -n "$why" ] || why=$(cat "$stop.short" 2>/dev/null || true)
			rm -f "$stop" "$stop.ok" "$stop.short" "$stop".slow-*
			[ ${#places[@]} -gt 0 ] || { echo "not cloned: $why"; return 1; }
			mapfile -t places < <(printf '%s\n' "${places[@]}" | sort -V)
			if [ ${#places[@]} -le 5 ]; then
				out="cloned as ${places[*]#tile-}"
			else
				out="cloned ${#places[@]}"
			fi
			[ ${#places[@]} -eq "$want" ] || out+=" of $want"
			echo "$out in $(since "$s") s${why:+; no more: $why}"
			;;
		remove)
			# Off the list first, so its place stays empty.
			list_drop "$name"
			if has "$name" && ! try container remove "$name"; then
				list_add "$name"
				case $why in
				*clone*) echo "not removed: a clone stands on its disk; remove the clone first" ;;
				*) echo "not removed: $why" ;;
				esac
				return 1
			fi
			img_set "$name" "$agent"
			if ! vol_remove "$name"; then
				echo "removed, but its volume stays: $why"
				return 1
			fi
			echo "removed with its volume: its place stays empty"
			;;
		*)
			echo "no such verb: $3"
			return 1
			;;
		esac
	}
	# host_says: what the host holds of the wall, said to it (POST /host):
	# how many of its tiles' microVMs run and the memory they take (each
	# one's resident set, from ps), and their disks two ways, from ZFS: as
	# full copies would take them (what each refers to), and as ZFS keeps
	# them (each one's own, and each image they come from, once); each
	# tile's own by its place; and the volumes they count on, and those
	# they left behind, as ZFS keeps them. A tile's disk is the volume
	# vms/NAME/root, its volume vms/volumes/VOLUME; an image's is the
	# dataset its golden is a snapshot of.
	host_says() {
		{
			grep -x 'tile-[0-9]*' "$made" 2>/dev/null || true
			echo "--vols"
			cat "$vols" "$orphans" 2>/dev/null || true
			echo "--ps"
			ps -C firecracker -o rss=,args= 2>/dev/null || true
			echo "--zfs"
			zfs list -Hp -o name,used,referenced,origin -t volume 2>/dev/null || true
		} | awk '
			$0 == "--vols" { part = "vols"; next }
			$0 == "--ps" { part = "ps"; next }
			$0 == "--zfs" { part = "zfs"; FS = "\t"; next }
			part == "" { tile[$0] = 1; next }
			# A record is NAME VOLUME, one left behind VOLUME alone; a tile
			# with no record counts on the volume of its own name.
			part == "vols" { vol[NF > 1 ? $2 : $1] = 1; next }
			part == "ps" {
				for (i = 2; i < NF; i++) if ($i == "--id" && ($(i + 1) in tile)) { mem += $1 * 1024; vms++ }
				next
			}
			{
				used[$1] = $2
				n = split($1, p, "/")
				if (n >= 3 && p[n] == "root" && p[n - 2] == "vms" && (p[n - 1] in tile)) {
					own[p[n - 1]] = $2; copies += $3; disk += $2
					if ($4 ~ /\/images\//) { o = $4; sub(/@.*/, "", o); img[o] = 1 }
				} else if (n >= 3 && p[n - 1] == "volumes" && p[n - 2] == "vms" && ((p[n] in vol) || (p[n] in tile))) {
					vols += $2
				}
			}
			END {
				for (d in img) disk += used[d]
				printf "{\"tiles\":%.0f,\"memory\":%.0f,\"copies\":%.0f,\"disk\":%.0f,\"volumes\":%.0f,\"own\":{", vms, mem, copies, disk, vols
				sep = ""
				for (t in own) { k = substr(t, 6) + 0; printf "%s\"%d\":%.0f", sep, k, own[t]; sep = "," }
				print "}}"
			}' | curl -s -o /dev/null --max-time 2 -X POST --data-binary @- "$url/host" || true
	}
	# job I NAME VERB [N [AT]]: the verb done in the background (a clone N
	# times, a restore to snapshot AT), the tile marked busy while it runs,
	# and what became of it
	# said to the wall and the log.
	job() {
		: >"$busy/$2"
		case $3 in update | rollback) : >"$busy/.update-$2" ;; esac
		(
			if said=$(act_on "$2" "$1" "$3" "${4:-}" "${5:--}"); then f=0; else f=1; fi
			say "$1" "$said" "$f"
			echo "   $2: $3${said:+: $said}"
			rm -f "$busy/$2" "$busy/.update-$2"
		) </dev/null &
	}
	# again NAME HOW: a tile that is gone run again, or one stopped started
	# (HOW: run or start), in the background; one that does not is tried
	# again five seconds later.
	again() {
		: >"$busy/$1"
		(
			if [ "$2" = start ] && try container start "$1"; then
				echo "   $1 was stopped: started"
			elif [ "$2" = start ] && ! has "$1" && room && run_tile "$1"; then
				# Gone while it was started: the pass read it as stopped
				# while vmx was removing it (vmx container remove by hand),
				# so it is run at once, not five seconds later. 2026-11-19:
				# "no container tile-19", back in 7.8 s where 1.3 s would do.
				echo "   $1 was gone: a new one is running"
			elif [ "$2" = run ] && room && run_tile "$1"; then
				echo "   $1 was gone: a new one is running"
			else
				echo "   $1 did not $2${why:+: $why}: tried again in five seconds"
				sleep 5
			fi
			rm -f "$busy/$1"
		) </dev/null &
	}
	# reset_wall: the wall made again, as the page asked: every tile and
	# clone this script keeps taken away, and the picture's own run again.
	reset_wall() {
		local s names present left name running k
		curl -sf -o /dev/null --max-time 2 -X POST "$url/reset/down" || { rm -f "$busy/.reset"; return 0; }
		s=$(date +%s%N)
		echo "   reset: the wall is taken down"
		# What is in hand finishes first, a minute at most.
		for _ in $(seq 1 600); do
			[ -z "$(find "$busy" -mindepth 1 -maxdepth 1 ! -name '.*' -print -quit)" ] && break
			sleep 0.1
		done
		mapfile -t names < <(grep -x 'tile-[0-9]*' "$made" || true)
		# Every one at once, sixteen at a time; one a clone stands on is
		# refused until the clone is gone, and goes in the next round.
		left=$busy/.left
		for _ in 1 2 3 4 5 6; do
			[ ${#names[@]} -gt 0 ] || break
			present=$(vmx container list --json | jq -r '.[].name') || present=
			: >"$left"
			running=0
			for name in "${names[@]}"; do
				if [ "$running" -ge 16 ]; then
					wait -n || true
					running=$((running - 1))
				fi
				# Its volume after it; one a clone's copy still stands on goes
				# in a later round, once the copy has.
				(if ! grep -qx -- "$name" <<<"$present" || try container remove "$name"; then
					if vol_remove "$name"; then list_drop "$name"; else echo "$name" >>"$left"; fi
				else echo "$name" >>"$left"; fi) </dev/null &
				running=$((running + 1))
			done
			wait
			mapfile -t names <"$left"
		done
		rm -f "$left"
		# What stayed: a tile vmx would not remove is left as it is; a
		# volume it would not (its tile gone) is run on again below.
		if [ ${#names[@]} -gt 0 ]; then
			present=$(vmx container list --json | jq -r '.[].name') || present=
			local kept_c=() kept_v=()
			for name in "${names[@]}"; do
				if grep -qx -- "$name" <<<"$present"; then kept_c+=("$name"); else kept_v+=("$name"); fi
			done
			[ ${#kept_c[@]} -eq 0 ] || echo "   reset: ${kept_c[*]} not removed: left as they are"
			[ ${#kept_v[@]} -eq 0 ] || echo "   reset: the volumes of ${kept_v[*]} stay: those tiles run again on them"
		fi
		# Every tile runs the first image again.
		(flock 8 && rm -f "$imgs") 8>"$book"
		curl -sf -o /dev/null --max-time 2 -X POST "$url/reset/up" || echo "   reset: the wall did not take the end of the teardown"
		echo "   reset: the wall is down in $(since "$s") s; its $n tiles run again"
		# Sixteen at a time, as they were taken away; one that does not start
		# is run again by the keeper's next pass.
		present=$(vmx container list --json | jq -r '.[].name') || present=
		running=0
		for ((k = 0; k < n; k++)); do
			name=$(tile_name "$k")
			if listed "$name" && grep -qx -- "$name" <<<"$present"; then continue; fi
			if [ "$running" -ge 16 ]; then
				wait -n || true
				running=$((running - 1))
			fi
			: >"$busy/$name"
			listed "$name" || list_add "$name"
			(
				if ! { room && run_tile "$name"; }; then echo "   reset: $name did not start: the keeper runs it again"; fi
				rm -f "$busy/$name"
			) </dev/null &
			running=$((running + 1))
		done
		wait
		echo "   reset: the wall's $n tiles run again, $(since "$s") s after it was taken"
		rm -f "$busy/.reset"
	}
	next_host=0 next_heal=0 next_orphans=0 watched=1
	echo "Keeping the wall: what the page's buttons ask for is done here with vmx, and a tile that is gone"
	echo "or stopped is run again. Ctrl+C ends the keeping; the wall stays."
	trap 'echo; echo "The keeper has stopped. The wall is as it was left."; exit 0' INT TERM
	while :; do
		# The wall's record gone (--remove): what is in hand ends, and the
		# keeper with it.
		if [ ! -f "$made" ]; then
			wait
			echo "The wall was removed (${0##*/} --remove): the keeper stops."
			exit 0
		fi
		# What the host holds, said to the wall every two seconds while
		# somebody watches (the page shows it), every ten while nobody does.
		if [ ! -e "$busy/.host" ] && [ "$(date +%s)" -ge "$next_host" ]; then
			: >"$busy/.host"
			(host_says; rm -f "$busy/.host") </dev/null &
			next_host=$(($(date +%s) + (watched ? 2 : 10)))
		fi
		# What the page asked for. A wall that does not answer is asked again.
		doc=$(curl -s --max-time 2 "$url/asked") || doc=
		# A wall of another protocol (one from before 2 says none): what it
		# asks is not done, said once, as if it did not answer, until it is
		# made again; a tile that goes is still run again.
		p=$(jq -r '.protocol // 1' <<<"$doc" 2>/dev/null) || p=
		if [ -n "$doc" ] && [[ $p =~ ^[0-9]+$ ]] && [ "$p" != "$protocol" ]; then
			if [ ! -e "$busy/.protocol" ]; then
				: >"$busy/.protocol"
				echo "   the wall speaks protocol $p and this keeper $protocol: nothing it asks is done; ${0##*/} --remove, then make it again" >&2
			fi
			doc=
		fi
		# Shut down, asked on the page: what the keeper has in hand finishes,
		# then the wall goes as --remove takes it, and the keeper ends.
		if [ "$(jq -r '.shutdown // false' <<<"$doc" 2>/dev/null)" = true ]; then
			echo "   shutdown: asked on the page: the keeper takes the wall away"
			wait
			remove_all keeper
			echo "   shutdown: done; the keeper ends"
			exit 0
		fi
		if [ "$(jq -r '.reset // false' <<<"$doc" 2>/dev/null)" = true ] && [ ! -e "$busy/.reset" ]; then
			: >"$busy/.reset"
			reset_wall </dev/null &
		fi
		# Whether somebody watches (a wall from before says nothing of it:
		# taken as watched).
		watched=$(jq -r 'if .watched == false then 0 else 1 end' <<<"$doc" 2>/dev/null) || watched=1
		[ -n "$watched" ] || watched=1
		# While the wall is made again, nothing else is done.
		if [ ! -e "$busy/.reset" ]; then
			acts=$(jq -r '.acts[]? | "\(.tile) \(.do) \(.n // 0) \(.at // "-")"' <<<"$doc" 2>/dev/null) || acts=
			while read -r i act cnt at; do
				# A place of the wall, by its number.
				[[ $i =~ ^[0-9]+$ ]] && [ "$i" -lt "$most" ] || continue
				name=$(tile_name "$i")
				# One job a tile, a dozen at once: what is left is asked
				# again at the next pass.
				[ ! -e "$busy/$name" ] || continue
				[ "$(jobs -rp | wc -l)" -lt 12 ] || break
				# An update or a roll back rolls over the wall eight at a time
				# while the rest draw: one past that waits for the next pass.
				if { [ "$act" = update ] || [ "$act" = rollback ]; } && [ "$(find "$busy" -maxdepth 1 -name '.update-*' | wc -l)" -ge 8 ]; then continue; fi
				curl -s -o /dev/null --max-time 2 -X POST "$url/acting/$i" || continue
				job "$i" "$name" "$act" "$cnt" "$at"
			done <<<"$acts"
			# The snapshots a page asked for, of each tile at most every two
			# seconds: vmx lists them and the wall is given them as vmx
			# does, less a clone's own, which no restore goes back to.
			for i in $(jq -r '.snaps[]?' <<<"$doc" 2>/dev/null); do
				[[ $i =~ ^[0-9]+$ ]] && [ "$i" -lt "$most" ] || continue
				name=$(tile_name "$i")
				listed "$name" || continue
				stamp=$busy/.snaps-$name
				if [ -e "$stamp" ] && [ $(($(date +%s) - $(stat -c %Y "$stamp"))) -lt 2 ]; then continue; fi
				: >"$stamp"
				(
					if list=$(vmx container snapshots "$name" --json 2>/dev/null | jq -c '[.[]? | select(.name | startswith("clone-") | not)]'); then
						curl -s -o /dev/null --max-time 2 -X POST --data-binary "$list" "$url/snaps/$i" || true
					fi
				) </dev/null &
			done
			# One that is gone or stopped, however it went, and not in hand:
			# the list read first, then what is in hand, then vmx's list. A job
			# that ends while vmx's list is read leaves a tile as the list
			# caught it, a clone's new place not yet made or a tile stopped for
			# its restore, and in hand as the pass began it is passed over; the
			# next pass sees it as it is. (Read in the other order, a restore's
			# tile was started over its own restart and a clone's place run
			# over its clone: "already exists".) A list that was not read is
			# not taken for a host with no tiles.
			# A list taken away since the top of the pass is the wall removed:
			# the next pass stops the keeper.
			# Every second while somebody watches, every five while nobody does:
			# each read of vmx's list has vmxd look at every microVM's unit, and
			# three times a second it cost the host about a processor with the
			# tiles idle (2026-11-18).
			# The volumes tiles left behind, every half minute.
			if [ -s "$orphans" ] && [ "$(date +%s)" -ge "$next_orphans" ]; then
				next_orphans=$(($(date +%s) + 30))
				orphan_pass
			fi
			if [ "$(date +%s)" -ge "$next_heal" ]; then
				next_heal=$(($(date +%s) + (watched ? 1 : 5)))
				mapfile -t names <"$made" || { sleep 0.3; continue; }
				held=$(ls -A "$busy")
				if now=$(vmx container list --json 2>/dev/null | jq -r '.[] | "\(.name) \(.state)"'); then
					for name in "${names[@]}"; do
						if [[ $name != tile-* ]] || [ -e "$busy/$name" ] || grep -qx -- "$name" <<<"$held"; then continue; fi
						state=$(awk -v n="$name" '$1 == n { print $2 }' <<<"$now")
						case $state in
						running) ;;
						"") again "$name" run ;;
						*) again "$name" start ;;
						esac
					done
				fi
			fi
		fi
		# A press comes from a page that is open: the keeper asks three
		# times a second then, and once a second while nobody watches.
		if [ "$watched" = 1 ]; then sleep 0.3; else sleep 1; fi
	done
fi
[ $# -eq 0 ] || die "usage: ${0##*/} [--from DIR | --keep | --keep-here | --show | --demo | --remove]"
[[ $cols =~ ^[0-9]+$ && $rows =~ ^[0-9]+$ ]] && [ $((cols * rows)) -ge 1 ] && [ $((cols * rows)) -le 64 ] || die "COLS x ROWS is 1 to 64 tiles"
[ -f "$made" ] && die "this script made a wall already: ${0##*/} --remove takes it away first"
mkdir -p "$(dirname "$made")"
# Before anything is made: the volumes a wall before this one left behind
# (its --remove could not take them, a vmx whose removal kept a container's
# snapshots on them, for one) taken away now, with any clone's copy that no
# container has standing on them, those that will not go kept for the next
# time; then no container or volume of a name the wall will
# use may be there, which it would run beside or count on. A wall refused
# here has made nothing (2026-11-18: one refused after the wall's own
# container was run left half a wall).
if [ -s "$orphans" ]; then
	: >"$orphans.new"
	while read -r v; do
		[ -n "$v" ] || continue
		if vmx container volume-remove "$v" >/dev/null 2>&1 || { take_copies "$v" && vmx container volume-remove "$v" >/dev/null 2>&1; }; then
			echo "   the volume $v the wall before left behind is taken away"
		elif vmx container volumes --json | jq -e --arg v "$v" 'any(.[]?; .name == $v)' >/dev/null; then
			echo "$v" >>"$orphans.new"
		fi
	done <"$orphans"
	mv "$orphans.new" "$orphans"
	[ -s "$orphans" ] || rm -f "$orphans"
fi
present=$(vmx container list --json | jq -r '.[].name')
volumes=$(vmx container volumes --json | jq -r '.[]?.name')
if grep -qx scene-wall <<<"$present"; then
	by_another "no wall made"
	die "a container named scene-wall is there already"
fi
for i in $(seq 0 $((cols * rows - 1))); do
	n=$(printf 'tile-%02d' "$i")
	! grep -qx -- "$n" <<<"$present" || die "a container named $n is there already"
	! grep -qx -- "$n" <<<"$volumes" || die "a volume named $n is there already: its counts would go on from it (vmx container volumes; vmx container volume-remove $n takes it away)"
done

# get IMAGE ARCHIVE DIGEST: import the archive when one is given, else pull.
get() {
	if [ -n "$2" ]; then
		[ -n "$3" ] || die "$2 needs its digest, as the machine that built it printed it"
		vmx container import "$2" "$1" --digest "$3"
	else
		vmx container pull "$1"
	fi
}
echo "== the images"
get "$wall" "${WALL_ARCHIVE:-}" "${WALL_DIGEST:-}"
get "$agent" "${AGENT_ARCHIVE:-}" "${AGENT_DIGEST:-}"
get "$next" "${NEXT_ARCHIVE:-}" "${NEXT_DIGEST:-}"

echo "== the wall, on this host's port $port"
echo scene-wall >>"$made"
# Everything after -- is the program's arguments, as after the image in docker run.
quiet container run "$wall" scene-wall --port "$port:8090" -- -cols "$cols" -rows "$rows" || die "the wall did not start (above, what vmx said); ${0##*/} --remove takes away what was made"
wall_at=$(address scene-wall)
[ -n "$wall_at" ] || die "the wall has no address"
echo "   the wall runs at $wall_at; this host's port $port leads to it"
printf 'agent %s\nnext %s\nwall %s\nat %s\ntiles %s\n' "$agent" "$next" "$wall" "$wall_at" "$((cols * rows))" >"$kept"
rm -f "$imgs"
for _ in $(seq 1 30); do curl -s --max-time 2 "http://$wall_at:8090/frame" >/dev/null && break; sleep 1; done
curl -s --max-time 2 "http://$wall_at:8090/frame" >/dev/null || die "the wall does not answer at http://$wall_at:8090: vmx container logs scene-wall"

host=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") { print $(i + 1); exit }}')
echo "   open http://${host:-THIS-HOST}:$port/ in a browser now: its clock times the wall coming up"
echo "== $((cols * rows)) tiles, sixteen at a time"
: >"$vols"
began=$(date +%s%N)
: >"$failed"
running=0
for i in $(seq 0 $((cols * rows - 1))); do
	n=$(printf 'tile-%02d' "$i")
	echo "$n" >>"$made"
	if [ "$running" -ge 16 ]; then
		wait -n || true
		running=$((running - 1))
	fi
	(
		if quiet container run "$agent" "$n" --env "TILE=$i" --env "WALL=http://$wall_at:8090" \
			--env VOLUME=/data --volume "$n:/data" --volume-size "$vol_size"; then
			ms=$((($(date +%s%N) - began) / 1000000))
			printf '   %s running, %d.%d s\n' "$n" $((ms / 1000)) $((ms % 1000 / 100))
		else
			echo "$n" >>"$failed"
		fi
	) </dev/null &
	running=$((running + 1))
done
wait
if [ -s "$failed" ]; then
	die "$(tr '\n' ' ' <"$failed")did not start (above, what vmx said); ${0##*/} --remove takes away what was made"
fi
rm -f "$failed"
echo "== the tiles boot and start drawing"
for _ in $(seq 1 300); do
	[ "$(curl -s --max-time 2 "http://$wall_at:8090/state" | jq -r .live)" = $((cols * rows)) ] && break
	sleep 0.2
done
ms=$((($(date +%s%N) - began) / 1000000))
curl -s "http://$wall_at:8090/state" | jq -r --argjson n $((cols * rows)) --arg t "$((ms / 1000)).$((ms % 1000 / 100))" \
	'"the wall: \(.live) of \($n) tiles live \($t) s after the first was run, \(.fps) frames a second"'
echo
echo "== the keeper"
keeper_start
echo
echo "On the page: click or drag to choose tiles, then a button in cfg, under the wall: Snapshot, Restore,"
echo "Rebuild, Kill, Clone (as many as the box beside it says) or Remove. Reset the wall makes the whole wall again."
echo "Each tile says what became of it, and the line under the clock what the host holds of the wall."
echo "Pull some out:  for i in 05 06 13 14; do vmx container remove tile-\$i; done   (the keeper runs them again)"
echo "Take it all away with: ${0##*/} --remove"
