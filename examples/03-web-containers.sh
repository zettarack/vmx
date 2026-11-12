#!/usr/bin/env bash
# Web servers as containers: the official nginx image, COUNT copies, each a
# Firecracker microVM of its own, each led to from a port of this host.
#
#   ./03-web-containers.sh [COUNT]   COUNT copies (2 unless given), on ports 8081 and up
#   ./03-web-containers.sh --remove  remove the ones this script made
#
# IMAGE (nginx:alpine), PREFIX (web) and FIRST_PORT (8081) change them.
# Examples:
#   ./03-web-containers.sh                     web-01 and web-02 on ports 8081 and 8082
#   ./03-web-containers.sh 5                   web-01 to web-05 on 8081 to 8085
#   PREFIX=site FIRST_PORT=9080 ./03-web-containers.sh 3  site-01 to site-03 on 9080 to 9082
#   IMAGE=nginx:1.27-alpine ./03-web-containers.sh 1      a given nginx
#   ./03-web-containers.sh --remove            every container this script made
# Run it on the vmx host, as root.
set -euo pipefail

image=${IMAGE:-nginx:alpine}
prefix=${PREFIX:-web}
first=${FIRST_PORT:-8081}
# What this script made, by exact name, beside it: --remove takes only those.
made=$(cd "$(dirname "$0")" && pwd)/.made/03-web-containers.made

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
vmx container list --json >/dev/null 2>&1 || die "this vmx has no containers: upgrade it to a release that has 'vmx container'"
has() { vmx container list --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }
address() { vmx container list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .address // empty'; }

if [ "${1:-}" = --remove ]; then
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	while read -r name; do
		if has "$name"; then vmx container remove "$name"; fi
	done <"$made"
	rm -f "$made"
	exit 0
fi
count=${1:-2}
[[ $count =~ ^[0-9]+$ ]] && [ "$count" -ge 1 ] && [ "$count" -le 50 ] || die "COUNT is 1 to 50"
[ -f "$made" ] && die "this script made containers already: ${0##*/} --remove takes them away first"
mkdir -p "$(dirname "$made")"

vmx container pull "$image"
for i in $(seq 1 "$count"); do
	name=$(printf '%s-%02d' "$prefix" "$i")
	has "$name" && die "a container named $name is there already"
	echo "$name" >>"$made"
	# --port HOST:CONTAINER: this host's port leads to the container's 80.
	vmx container run "$image" "$name" --port "$((first + i - 1)):80"
done

host=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") { print $(i + 1); exit }}')
echo
echo "== each one answering"
for i in $(seq 1 "$count"); do
	name=$(printf '%s-%02d' "$prefix" "$i")
	addr=$(address "$name")
	code=000
	for _ in $(seq 1 30); do
		code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 2 "http://$addr/" || true)
		[ "$code" = 200 ] && break
		sleep 1
	done
	echo "$name: nginx answers $code at http://$addr/ (from this host); from your network: http://${host:-THIS-HOST}:$((first + i - 1))/"
done
echo
vmx container list
echo
echo "Each one's log: vmx container logs $prefix-01"
echo "Take them away with: ${0##*/} --remove"
