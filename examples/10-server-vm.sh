#!/usr/bin/env bash
# A server from a template, the everyday machine: debian-13-server cloned
# in an instant (a ZFS clone of the template's golden), your key put in at
# its first boot, started, and the ssh line printed once it answers. Then
# the guard you keep before an upgrade: a snapshot, and the rollback to it.
#
#   ./10-server-vm.sh            make web1 and start it
#   ./10-server-vm.sh --remove   remove the machine this script made
#
# SSH_KEY is your public key's file on this host (it is how you log in:
# no machine here has a default password). NAME (web1), TEMPLATE
# (debian-13-server), CPUS (2) and MEMORY (2G) change the rest.
# Examples:
#   SSH_KEY=/root/my-key.pub ./10-server-vm.sh
#   SSH_KEY=/root/my-key.pub NAME=db1 CPUS=4 MEMORY=8G ./10-server-vm.sh
#   SSH_KEY=/root/my-key.pub NAME=u24 TEMPLATE=ubuntu-24.04-server ./10-server-vm.sh
#   NAME=db1 ./10-server-vm.sh --remove
# Run it on the vmx host, as root.
set -euo pipefail

name=${NAME:-web1}
template=${TEMPLATE:-debian-13-server}
made=$(cd "$(dirname "$0")" && pwd)/.made/10-server-vm-$name.made

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
state() { vmx list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .state'; }
# A machine's address is its lease, found by its card's MAC.
address() {
	local mac
	mac=$(vmx list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .mac // empty')
	[ -n "$mac" ] || return 0
	vmx net leases --json | jq -r --arg m "$mac" '[.[] | select(.mac == $m)][0].address // empty'
}

if [ "${1:-}" = --remove ]; then
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	while read -r n; do
		if [ -n "$(state "$n")" ]; then vmx remove "$n"; fi
	done <"$made"
	rm -f "$made"
	exit 0
fi
[ $# -eq 0 ] || die "usage: ${0##*/} [--remove]"
[ -n "${SSH_KEY:-}" ] || die "SSH_KEY names your public key's file: SSH_KEY=/root/my-key.pub ${0##*/}"
[ -f "$SSH_KEY" ] || die "no key file at $SSH_KEY"
[ -f "$made" ] && die "this script made $name already: NAME=$name ${0##*/} --remove takes it away first"
[ -n "$(state "$name")" ] && die "a machine named $name is there already"
mkdir -p "$(dirname "$made")"

echo "== 1. $name, cloned from $template"
echo "$name" >>"$made"
vmx clone "$template" "$name" --cpus "${CPUS:-2}" --memory "${MEMORY:-2G}" --ssh-key "$SSH_KEY"
vmx start "$name"
began=$(date +%s)
addr=""
for _ in $(seq 1 120); do
	addr=$(address "$name")
	if [ -n "$addr" ] && timeout 1 bash -c "</dev/tcp/$addr/22" 2>/dev/null; then break; fi
	sleep 2
done
[ -n "$addr" ] || die "$name has no address after 4 minutes: vmx console $name shows its boot"
echo "   $name answers ssh at $addr, $(( $(date +%s) - began )) s after its start"
echo "   from this host: ssh debian@$addr   (the user is the image's own: debian on Debian, ubuntu on Ubuntu)"

echo "== 2. a snapshot before a change, and back to it"
# Taken with the machine stopped, so it holds a whole, shut-down disk. A
# snapshot of a running machine holds what its disk held at that instant,
# as after a power cut: one taken seconds into this machine's first boot,
# while cloud-init was still at work, gave a disk whose ssh never started
# again after the rollback. Before a change, stop it,
# or let it settle.
stop_it() {
	vmx stop "$name"
	for _ in $(seq 1 60); do [ "$(state "$name")" = shutoff ] && return 0; sleep 2; done
	die "$name has not stopped after 2 minutes"
}
ssh_again() {
	local began
	began=$(date +%s)
	for _ in $(seq 1 120); do
		addr=$(address "$name")
		if [ -n "$addr" ] && timeout 1 bash -c "</dev/tcp/$addr/22" 2>/dev/null; then
			echo "   $name answers ssh at $addr again, $(( $(date +%s) - began )) s after its start"
			return 0
		fi
		sleep 2
	done
	die "$name does not answer ssh 4 minutes after its start: vmx console $name shows its boot"
}
stop_it
vmx snapshot "$name" before-change
vmx start "$name"
ssh_again
vmx snapshots "$name"
echo "   (a change would go here; then, to go back:)"
stop_it
vmx rollback "$name" before-change
vmx start "$name"
ssh_again
echo
vmx list | grep -E "^NAME|^$name "
echo
echo "Take it away with: NAME=$name ${0##*/} --remove"
