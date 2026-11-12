#!/usr/bin/env bash
# Docker's hello-world, run as a Firecracker microVM of its own: vmx pulls
# the image, makes it a golden on ZFS, boots a microVM from a clone of it,
# and keeps what the program printed.
#
#   ./01-hello-container.sh            pull it, run it, show what it printed
#   ./01-hello-container.sh --remove   remove the container and the image
#
# Examples:
#   ./01-hello-container.sh                    hello-world as the container "hello"
#   NAME=hello2 ./01-hello-container.sh        the same, as the container "hello2"
#   NAME=hello2 ./01-hello-container.sh --remove
# Run it on the vmx host, as root.
set -euo pipefail

name=${NAME:-hello}
image=hello-world

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
vmx container list --json >/dev/null 2>&1 || die "this vmx has no containers: upgrade it to a release that has 'vmx container'"
has() { vmx container list --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }
state() { vmx container list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .state'; }

if [ "${1:-}" = --remove ]; then
	if has "$name"; then vmx container remove "$name"; fi
	# The image's field is "image", as vmx container images --json gives it.
	if vmx container images --json | jq -e 'any(.[]; .image == "docker.io/library/hello-world:latest")' >/dev/null; then
		vmx container image-remove "$image"
	fi
	exit 0
fi
[ $# -eq 0 ] || die "usage: ${0##*/} [--remove]"
has "$name" && die "a container named $name is there already: ${0##*/} --remove takes it away"

vmx container pull "$image"
vmx container run "$image" "$name"
# hello-world prints and ends: wait for its microVM to stop.
for _ in $(seq 1 60); do
	[ "$(state "$name")" = stopped ] && break
	sleep 1
done
echo
echo "== what it printed (vmx container logs $name)"
# The console holds the kernel's and Firecracker's lines too; the
# program's own are the rest.
vmx container logs "$name" | grep -v -E '^[0-9]{4}-[0-9]{2}-[0-9]{2}T|^\[ *[0-9]+\.[0-9]+\] |^vmx-init: (started|[0-9]+ modules|eth0|running)' | sed 's/\\x0d$//'
echo
echo "Run it again with: vmx container start $name; then: vmx container logs $name"
echo "Take it away with: ${0##*/} --remove"
