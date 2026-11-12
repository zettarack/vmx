#!/usr/bin/env bash
# A machine from an installer ISO, installed by hand on its screen: the
# Debian netinst image fetched and checked against Debian's own SHA256SUMS,
# a blank disk, and the ISO as a CD the machine boots while its disk is
# empty (once a system is on the disk, the disk boots first). Any ISO works
# the same way: put it in vmx's ISO folder with vmx isos fetch.
#
#   ./13-iso-install.sh          fetch the ISO if it is not here, make the machine, start it
#   ./13-iso-install.sh --eject  take the CD out, once the system is installed
#   ./13-iso-install.sh --remove remove the machine (the ISO stays)
#
# NAME (inst1), DISK (20G), ISO (debian-netinst.iso: the name vmx keeps it
# under; one already in vmx isos is used as it is) and MIRROR (Debian's
# current netinst folder) change it.
# Examples:
#   ./13-iso-install.sh                        inst1, from Debian's newest netinst
#   NAME=inst2 DISK=40G ./13-iso-install.sh
#   ISO=my-os.iso NAME=other ./13-iso-install.sh     an ISO fetched before (vmx isos)
#   MIRROR=https://cdimage.debian.org/debian-cd/current/amd64/iso-cd ./13-iso-install.sh
#   NAME=inst2 ./13-iso-install.sh --eject
#   NAME=inst2 ./13-iso-install.sh --remove
# Run it on the vmx host, as root.
set -euo pipefail

name=${NAME:-inst1}
iso=${ISO:-debian-netinst.iso}
mirror=${MIRROR:-https://cdimage.debian.org/debian-cd/current/amd64/iso-cd}
made=$(cd "$(dirname "$0")" && pwd)/.made/13-iso-install-$name.made
isos=/var/lib/zettarack/vmx/isos

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
state() { vmx list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .state'; }

case "${1:-}" in
--remove)
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	if [ -n "$(state "$name")" ]; then vmx remove "$name"; fi
	rm -f "$made"
	exit 0 ;;
--eject)
	[ -n "$(state "$name")" ] || die "no machine named $name"
	vmx eject "$name"
	exit 0 ;;
'') ;;
*) die "usage: ${0##*/} [--eject | --remove]" ;;
esac
[ -f "$made" ] && die "this script made $name already: NAME=$name ${0##*/} --remove takes it away first"
[ -n "$(state "$name")" ] && die "a machine named $name is there already"

if [ ! -f "$isos/$iso" ]; then
	echo "== the ISO: Debian's newest netinst, checked against its SHA256SUMS"
	line=$(curl -fsS "$mirror/SHA256SUMS" | grep -m1 -- '-amd64-netinst.iso$') || die "no netinst line in $mirror/SHA256SUMS"
	vmx isos fetch "$iso" --url "$mirror/${line##* }" --sha256 "${line%% *}"
fi
mkdir -p "$(dirname "$made")"
echo "$name" >"$made"
echo "== $name: a blank ${DISK:-20G} disk and $iso as its CD"
vmx create "$name" --iso "$iso" --disk "${DISK:-20G}"
vmx start "$name"
sleep 5
[ "$(state "$name")" = running ] || die "$name is $(state "$name") after its start"
echo "   $name runs its installer. Its screen: on this host's desktop under Machines, or vmx tui screen $name"
echo "   Once installed: NAME=$name ${0##*/} --eject"
