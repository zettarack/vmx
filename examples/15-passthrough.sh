#!/usr/bin/env bash
# A device of the host given to a machine: USB by vendor and product, or a
# PCI device (a GPU, a card) by its address. The device leaves the host for
# the machine while it runs and comes back when it stops; it is given to a
# stopped machine, so the script makes one from a template first.
#
#   USB=8087:0025 ./15-passthrough.sh      a server with the USB device given, started
#   PCI=0000:05:00.0 ./15-passthrough.sh   the same with a PCI device
#   ./15-passthrough.sh --remove           the device taken back and the machine removed
#
# vmx usb and vmx pci list the host's devices, and say of each whether it
# can be given and why not. NAME (pass1), TEMPLATE (debian-13-desktop) and
# SSH_KEY (none: no login) change the rest.
#
# The guest needs the drivers for the device. The server templates run
# their distribution's cloud kernel, which has none for USB: a device given
# to a machine cloned from debian-13-server is the machine's, and the guest
# sees nothing of it (Debian 13's cloud kernel, 6.12). The desktop
# templates run the full kernel, and a server can have it with
# apt install linux-image-amd64 and a restart.
# Examples:
#   vmx usb; USB=8087:0025 ./15-passthrough.sh
#   vmx pci; PCI=0000:05:00.0 NAME=wifi1 ./15-passthrough.sh
#   USB=1b1c:1b89 SSH_KEY=/root/my-key.pub ./15-passthrough.sh   then ssh debian@ADDRESS: lsusb shows it
#   NAME=wifi1 ./15-passthrough.sh --remove
# Run it on the vmx host, as root.
set -euo pipefail

name=${NAME:-pass1}
template=${TEMPLATE:-debian-13-desktop}
made=$(cd "$(dirname "$0")" && pwd)/.made/15-passthrough-$name.made

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
state() { vmx list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .state'; }

if [ "${1:-}" = --remove ]; then
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	read -r kind dev <"$made"
	if [ -n "$(state "$name")" ]; then
		vmx stop "$name" >/dev/null 2>&1 || true # a stopped machine: nothing to stop
		for _ in $(seq 1 60); do [ "$(state "$name")" = shutoff ] && break; sleep 2; done
		vmx "$kind" take "$name" "$dev"
		vmx remove "$name"
	fi
	rm -f "$made"
	exit 0
fi
[ $# -eq 0 ] || die "usage: ${0##*/} [--remove]"
if [ -n "${USB:-}" ]; then kind=usb dev=$USB; elif [ -n "${PCI:-}" ]; then kind=pci dev=$PCI; else die "USB=VENDOR:PRODUCT or PCI=ADDRESS names the device: vmx usb, vmx pci"; fi
[ -f "$made" ] && die "this script made $name already: NAME=$name ${0##*/} --remove takes it away first"
[ -n "$(state "$name")" ] && die "a machine named $name is there already"
mkdir -p "$(dirname "$made")"

key=()
if [ -n "${SSH_KEY:-}" ]; then key=(--ssh-key "$SSH_KEY"); fi
echo "$kind $dev" >"$made"
vmx clone "$template" "$name" "${key[@]}"
# Given to the stopped machine: recorded on it, and in its definition.
vmx "$kind" give "$name" "$dev"
vmx start "$name"
echo
vmx "$kind" | grep -E "^(ADDRESS|ID)|$dev|$name"
echo
echo "$dev is $name's while it runs, and the host's again when it stops."
echo "Take it back and the machine away with: NAME=$name ${0##*/} --remove"
