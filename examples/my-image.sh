#!/usr/bin/env bash
# Your own application, from your own image, COUNT copies: the template to
# copy and change. Build the image anywhere podman or docker runs (see
# IMAGES.md), then either push it to a registry or carry the archive here.
#
# From a registry:
#   IMAGE=ghcr.io/you/myapp:1 PORT=8080 COUNT=3 ./my-image.sh
# From an archive made with podman save --format oci-archive (or docker save):
#   ARCHIVE=/home/myapp.tar DIGEST=sha256:... IMAGE=myapp:1 PORT=8080 COUNT=3 ./my-image.sh
#   (DIGEST is the one the build machine printed; nothing in an archive vouches for itself)
#
#   ./my-image.sh --remove   remove the containers this script made (the image stays)
#
# PORT is the port your application listens on inside (none: no port led to
# it); FIRST_PORT (9001) is this host's first port for it; ENV="K=V K2=V2"
# is its environment; PREFIX (app), CPUS (1) and MEMORY (256M) the rest.
# Examples:
#   IMAGE=ghcr.io/you/myapp:1 PORT=8080 ./my-image.sh                one copy, on this host's 9001
#   IMAGE=ghcr.io/you/myapp:1 PORT=8080 COUNT=3 FIRST_PORT=8001 ./my-image.sh   on 8001 to 8003
#   ARCHIVE=/home/myapp.tar DIGEST=sha256:73f9... IMAGE=myapp:1 PORT=8080 ./my-image.sh
#   IMAGE=myapp:1 ENV="MODE=demo LEVEL=info" PREFIX=demo CPUS=2 MEMORY=512M COUNT=2 ./my-image.sh
#   IMAGE=myapp:1 ./my-image.sh                no port: a worker that serves nothing
#   PREFIX=demo ./my-image.sh --remove         the copies this script made as demo-01 and on
# Run it on the vmx host, as root.
set -euo pipefail

image=${IMAGE:-}
prefix=${PREFIX:-app}
first=${FIRST_PORT:-9001}
# What this script made, by exact name, beside it: --remove takes only those.
made=$(cd "$(dirname "$0")" && pwd)/.made/my-image-$prefix.made

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
vmx container list --json >/dev/null 2>&1 || die "this vmx has no containers: upgrade it to a release that has 'vmx container'"
has() { vmx container list --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }

if [ "${1:-}" = --remove ]; then
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	while read -r name; do
		if has "$name"; then vmx container remove "$name"; fi
	done <"$made"
	rm -f "$made"
	exit 0
fi
[ -n "$image" ] || die "IMAGE names your image: IMAGE=ghcr.io/you/myapp:1 ${0##*/}"
count=${COUNT:-1}
[[ $count =~ ^[0-9]+$ ]] && [ "$count" -ge 1 ] && [ "$count" -le 50 ] || die "COUNT is 1 to 50"
[ -f "$made" ] && die "this script made $prefix containers already: ${0##*/} --remove takes them away first"

if [ -n "${ARCHIVE:-}" ]; then
	[ -n "${DIGEST:-}" ] || die "an archive needs DIGEST, as the machine that built it printed it"
	vmx container import "$ARCHIVE" "$image" --digest "$DIGEST"
else
	vmx container pull "$image"
fi

args=(--cpus "${CPUS:-1}" --memory "${MEMORY:-256M}")
for kv in ${ENV:-}; do args+=(--env "$kv"); done
mkdir -p "$(dirname "$made")"
for i in $(seq 1 "$count"); do
	name=$(printf '%s-%02d' "$prefix" "$i")
	has "$name" && die "a container named $name is there already"
	echo "$name" >>"$made"
	port=()
	if [ -n "${PORT:-}" ]; then port=(--port "$((first + i - 1)):$PORT"); fi
	vmx container run "$image" "$name" "${args[@]}" "${port[@]}"
done
echo
vmx container list
echo
echo "Each one's log: vmx container logs $prefix-01"
echo "Take them away with: ${0##*/} --remove"
