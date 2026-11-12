#!/usr/bin/env bash
# The demoscene wall: one picture drawn by many containers at once. The wall
# (scene/wall) is a container that hands out the frame's time and shows
# every tile in a browser; each tile (scene/agent) is a container of its own,
# a Firecracker microVM drawing its part of a Mandelbrot zoom and sending it
# back several times a second. Pull a few out and the wall shows the holes;
# run them again and they come back.
#
#   ./05-demoscene-wall.sh            the wall and COLS x ROWS tiles (8 x 5 = 40)
#   ./05-demoscene-wall.sh --remove   remove the wall and every tile this script made
#
# The images come from a registry (AGENT_IMAGE, WALL_IMAGE), or from
# archives built with scene/README.md's steps: AGENT_ARCHIVE and
# AGENT_DIGEST, WALL_ARCHIVE and WALL_DIGEST. COLS (8), ROWS (5) and PORT
# (8090, this host's port for the page) change the wall.
# Examples:
#   ./05-demoscene-wall.sh                     the wall and 40 tiles, images from the registry
#   COLS=4 ROWS=2 PORT=9090 ./05-demoscene-wall.sh      8 tiles, the page on port 9090
#   AGENT_IMAGE=ghcr.io/you/scene-agent:2 WALL_IMAGE=ghcr.io/you/scene-wall:2 ./05-demoscene-wall.sh
#   AGENT_ARCHIVE=/home/scene-agent.tar AGENT_DIGEST=sha256:1a0c... AGENT_IMAGE=scene-agent:1 \
#   WALL_ARCHIVE=/home/scene-wall.tar WALL_DIGEST=sha256:590e... WALL_IMAGE=scene-wall:1 ./05-demoscene-wall.sh
#   ./05-demoscene-wall.sh --remove            the wall and every tile
# Run it on the vmx host, as root.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
agent=${AGENT_IMAGE:-ghcr.io/zettarack/scene-agent:1}
wall=${WALL_IMAGE:-ghcr.io/zettarack/scene-wall:1}
cols=${COLS:-8}
rows=${ROWS:-5}
port=${PORT:-8090}
# What this script made, by exact name, beside it: --remove takes only those.
made=$here/.made/05-demoscene-wall.made

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
vmx container list --json >/dev/null 2>&1 || die "this vmx has no containers: upgrade it to a release that has 'vmx container'"
has() { vmx container list --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }
address() { vmx container list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .address // empty'; }

if [ "${1:-}" = --remove ]; then
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	while read -r n; do
		if has "$n"; then vmx container remove "$n" >/dev/null && echo "removed $n"; fi
	done <"$made"
	rm -f "$made"
	echo "The images stay: vmx container image-remove $agent (and $wall) take them away."
	exit 0
fi
[ $# -eq 0 ] || die "usage: ${0##*/} [--remove]"
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
echo "Pull some out:  for i in 05 06 13 14; do vmx container remove tile-\$i; done"
echo "Put them back:  for i in 05 06 13 14; do vmx container run $agent tile-\$i --env TILE=\$((10#\$i)) --env WALL=http://$wall_at:8090; done"
echo "Take it all away with: ${0##*/} --remove"
