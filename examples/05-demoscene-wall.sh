#!/usr/bin/env bash
# The demoscene wall: one picture drawn by many containers at once. The wall
# (scene/wall) is a container that hands out the frame's time and shows
# every tile in a browser; each tile (scene/agent) is a container of its own,
# a Firecracker microVM drawing its part of a Mandelbrot zoom and sending it
# back several times a second. Pull a few out and the wall shows the holes;
# run them again and they come back. With the keeper running, a click on a
# tile of the page destroys its microVM and boots a new one in its place,
# and a tile that goes any other way is run again.
#
#   ./05-demoscene-wall.sh              the wall and COLS x ROWS tiles (8 x 5 = 40), then its keeper
#   ./05-demoscene-wall.sh --from DIR   the same, with the images from the archives in DIR
#   ./05-demoscene-wall.sh --keep       start the keeper again, for the wall that is there
#   ./05-demoscene-wall.sh --keep-here  the keeper in this terminal instead, until Ctrl+C
#   ./05-demoscene-wall.sh --remove     stop the keeper; remove the wall and every tile this script made
#
# The keeper is this script, on the host, as root: the page is a program
# in a container and commands nothing. It asks the wall which tiles were
# clicked, removes each one's container and runs it again with the vmx
# command, and says how long each took. It touches only the tiles this
# script made. It runs as a service of its own, demoscene-keeper, so no
# terminal has to stay open for it: journalctl -fu demoscene-keeper shows
# what it does, and systemctl stop demoscene-keeper stops it (the wall
# stays, and a tile pulled out then stays out).
# The images come from a registry (AGENT_IMAGE, WALL_IMAGE), or from
# archives built with scene/README.md's steps: --from DIR takes
# scene-agent.tar and scene-wall.tar from DIR and checks each against its
# image's digest below; AGENT_ARCHIVE and AGENT_DIGEST, WALL_ARCHIVE and
# WALL_DIGEST name other archives. COLS (8), ROWS (5) and PORT (8090, this
# host's port for the page) change the wall.
# Examples:
#   ./05-demoscene-wall.sh                     the wall and 40 tiles, images from the registry
#   ./05-demoscene-wall.sh --from /home        the same, from /home/scene-agent.tar and /home/scene-wall.tar
#   COLS=4 ROWS=2 PORT=9090 ./05-demoscene-wall.sh      8 tiles, the page on port 9090
#   AGENT_IMAGE=ghcr.io/you/scene-agent:3 WALL_IMAGE=ghcr.io/you/scene-wall:3 ./05-demoscene-wall.sh
#   AGENT_ARCHIVE=/srv/agent.tar AGENT_DIGEST=sha256:... AGENT_IMAGE=scene-agent:3 \
#   WALL_ARCHIVE=/srv/wall.tar WALL_DIGEST=sha256:... WALL_IMAGE=scene-wall:3 ./05-demoscene-wall.sh
#   ./05-demoscene-wall.sh --keep              the keeper again, after systemctl stop demoscene-keeper
#   ./05-demoscene-wall.sh --remove            the keeper stopped, the wall and every tile removed
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
# The keeper's service: one on a host, as the wall's port is.
keeper_unit=demoscene-keeper
# The images' digests, the tiles' scene-agent:2 and the wall's scene-wall:3,
# as scene/README.md builds them: what an archive of each is checked
# against (--from).
agent_digest=sha256:a48d3773520d5f0855fa1ab88126140669890f4dfd5862d2e292e534c0207743
wall_digest=sha256:b1344efcaaf498c0763a9db117e81b2d33caf534c36d33844601e2423af2264b

die() { echo "${0##*/}: $*" >&2; exit 1; }
# --from DIR: the images from the archives in DIR, under their own names.
if [ "${1:-}" = --from ]; then
	[ $# -eq 2 ] && [ -n "$2" ] || die "usage: ${0##*/} --from DIR"
	from=${2%/}
	shift 2
	[ -f "$from/scene-agent.tar" ] && [ -f "$from/scene-wall.tar" ] || die "$from does not hold scene-agent.tar and scene-wall.tar"
	AGENT_ARCHIVE=${AGENT_ARCHIVE:-$from/scene-agent.tar} AGENT_DIGEST=${AGENT_DIGEST:-$agent_digest} AGENT_IMAGE=${AGENT_IMAGE:-scene-agent:2}
	WALL_ARCHIVE=${WALL_ARCHIVE:-$from/scene-wall.tar} WALL_DIGEST=${WALL_DIGEST:-$wall_digest} WALL_IMAGE=${WALL_IMAGE:-scene-wall:3}
fi
agent=${AGENT_IMAGE:-ghcr.io/zettarack/scene-agent:2}
wall=${WALL_IMAGE:-ghcr.io/zettarack/scene-wall:3}
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
vmx container list --json >/dev/null 2>&1 || die "this vmx has no containers: upgrade it to a release that has 'vmx container'"
has() { vmx container list --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }
address() { vmx container list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .address // empty'; }
# fact KEY: what the wall's record says of it; nothing when it does not.
fact() { [ -f "$kept" ] && awk -v k="$1" '$1 == k { print $2 }' "$kept"; }

if [ "${1:-}" = --remove ]; then
	[ $# -eq 1 ] || die "usage: ${0##*/} --remove"
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	# The images the wall was made from, as its record names them.
	agent=$(fact agent || true)
	wall=$(fact wall || true)
	mapfile -t names <"$made"
	# The records go first: a keeper still running (its service, or
	# --keep-here in a terminal) stops at its next pass and runs nothing again.
	rm -f "$made" "$kept"
	if systemctl is-active --quiet "$keeper_unit"; then
		systemctl stop "$keeper_unit" && echo "stopped the keeper ($keeper_unit)"
	fi
	# Twice: a keeper in a terminal may be in its last pass while the first
	# goes by, and run one again.
	for _ in 1 2; do
		for n in "${names[@]}"; do
			if has "$n"; then vmx container remove "$n" >/dev/null && echo "removed $n"; fi
		done
	done
	if [ -n "$agent" ] && [ -n "$wall" ]; then
		echo "The images stay: vmx container image-remove $agent (and $wall) take them away."
	else
		echo "The images stay: vmx container list --json names them, and vmx container image-remove takes one away."
	fi
	exit 0
fi

# keeper_start: the keeper as a service of its own, so no terminal has to
# stay open for it; what it does goes to the journal. One keeper at a time:
# two would each replace the same click.
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
	echo "The keeper runs as the service $keeper_unit: a tile clicked on the page is replaced, a tile gone is run again."
	echo "   what it does:  journalctl -fu $keeper_unit"
	echo "   stop it:       systemctl stop $keeper_unit   (the wall stays; a tile pulled out then stays out)"
}

if [ "${1:-}" = --keep ]; then
	[ $# -eq 1 ] || die "usage: ${0##*/} --keep"
	[ -f "$made" ] && [ -f "$kept" ] || die "no wall to keep: ${0##*/} makes one first"
	keeper_start
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
	at=$(fact at)
	n=$(fact tiles)
	[[ $n =~ ^[0-9]+$ ]] && [ -n "$agent" ] && [ -n "$at" ] || die "$kept does not say the tiles' image, the wall's address and how many tiles"
	url=http://$at:8090
	run_tile() { vmx container run "$agent" "$1" --env "TILE=$((10#${1#tile-}))" --env "WALL=$url" >/dev/null 2>&1; }
	# since NANOSECONDS: how long ago that was, in seconds and tenths.
	since() {
		local ms=$((($(date +%s%N) - $1) / 1000000))
		printf '%d.%d' $((ms / 1000)) $((ms % 1000 / 100))
	}
	echo "Keeping the wall whole: a tile clicked on the page is destroyed and a new one run in its"
	echo "place, and a tile that is gone or stopped is run again. Ctrl+C ends the keeping; the wall stays."
	trap 'echo; echo "The keeper has stopped. The wall is as it was left."; exit 0' INT TERM
	while :; do
		# The wall's record gone (--remove): there is nothing left to keep.
		[ -f "$made" ] || { echo "The wall was removed (${0##*/} --remove): the keeper stops."; exit 0; }
		# What the page asked for. A wall that does not answer is asked again.
		asked=$(curl -s --max-time 2 "$url/asked" | jq -r '.tiles[]?' 2>/dev/null) || asked=
		for i in $asked; do
			# Only a tile this script made, by its number: nothing else is
			# removed, whatever the page was sent.
			[[ $i =~ ^[0-9]+$ ]] && [ "$i" -lt "$n" ] || continue
			name=$(printf 'tile-%02d' "$i")
			grep -qx "$name" "$made" || continue
			curl -s -o /dev/null --max-time 2 -X POST "$url/taken/$i" || true
			began=$(date +%s%N)
			if has "$name" && ! vmx container remove "$name" >/dev/null 2>&1; then
				echo "   $name: not removed, and left as it is"
				continue
			fi
			gone=$(since "$began")
			again=$(date +%s%N)
			if run_tile "$name"; then
				# Started, not yet drawing: the page says how long the
				# tile was away, from the click to the new one's first frame.
				echo "   $name: destroyed in $gone s, a new one started $(since "$again") s later"
			else
				echo "   $name: destroyed, and a new one did not start: the next pass tries again"
			fi
		done
		# One that is gone or stopped, however it went. A list that was not
		# read is not taken for a host with no tiles.
		if listed=$(vmx container list --json 2>/dev/null | jq -r '.[] | "\(.name) \(.state)"'); then
			while read -r name; do
				[[ $name == tile-* ]] || continue
				state=$(awk -v n="$name" '$1 == n { print $2 }' <<<"$listed")
				case $state in
				running) ;;
				"") run_tile "$name" && echo "   $name was gone: a new one is running" ;;
				*) vmx container start "$name" >/dev/null 2>&1 && echo "   $name was $state: started" ;;
				esac
			done <"$made"
		fi
		sleep 0.3
	done
fi
[ $# -eq 0 ] || die "usage: ${0##*/} [--from DIR | --keep | --keep-here | --remove]"
[[ $cols =~ ^[0-9]+$ && $rows =~ ^[0-9]+$ ]] && [ $((cols * rows)) -ge 1 ] && [ $((cols * rows)) -le 64 ] || die "COLS x ROWS is 1 to 64 tiles"
[ -f "$made" ] && die "this script made a wall already: ${0##*/} --remove takes it away first"
mkdir -p "$(dirname "$made")"

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

echo "== the wall, on this host's port $port"
echo scene-wall >>"$made"
# Everything after -- is the program's arguments, as after the image in docker run.
vmx container run "$wall" scene-wall --port "$port:8090" -- -cols "$cols" -rows "$rows"
wall_at=$(address scene-wall)
[ -n "$wall_at" ] || die "the wall has no address"
printf 'agent %s\nwall %s\nat %s\ntiles %s\n' "$agent" "$wall" "$wall_at" "$((cols * rows))" >"$kept"
for _ in $(seq 1 30); do curl -s --max-time 2 "http://$wall_at:8090/frame" >/dev/null && break; sleep 1; done
curl -s --max-time 2 "http://$wall_at:8090/frame" >/dev/null || die "the wall does not answer at http://$wall_at:8090: vmx container logs scene-wall"

host=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") { print $(i + 1); exit }}')
echo "   open http://${host:-THIS-HOST}:$port/ in a browser now, and press 'reset the stopwatch'"
echo "== $((cols * rows)) tiles"
began=$(date +%s)
for i in $(seq 0 $((cols * rows - 1))); do
	n=$(printf 'tile-%02d' "$i")
	has "$n" && die "a container named $n is there already"
	echo "$n" >>"$made"
	vmx container run "$agent" "$n" --env "TILE=$i" --env "WALL=http://$wall_at:8090" >/dev/null
	echo "   $n running, $(( $(date +%s) - began )) s"
done
echo "== the tiles boot and start drawing"
for _ in $(seq 1 60); do
	[ "$(curl -s --max-time 2 "http://$wall_at:8090/state" | jq -r .live)" = $((cols * rows)) ] && break
	sleep 1
done
curl -s "http://$wall_at:8090/state" | jq -r --argjson n $((cols * rows)) --argjson t "$(( $(date +%s) - began ))" \
	'"the wall: \(.live) of \($n) tiles live \($t) s after the first was run, \(.fps) frames a second"'
echo
echo "== the keeper"
keeper_start
echo
echo "Click a tile on the page: its microVM is destroyed and a new one boots in its place."
echo "Pull some out:  for i in 05 06 13 14; do vmx container remove tile-\$i; done   (the keeper runs them again)"
echo "Take it all away with: ${0##*/} --remove"
