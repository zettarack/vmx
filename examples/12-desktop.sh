#!/usr/bin/env bash
# A Linux desktop in a machine: BASE with DESKTOP, built from the
# distribution's own packages and checked until its user "desktop" is
# logged in on its own screen. The screen is on this host's desktop
# (Machines), or in any terminal: vmx tui screen NAME.
#
#   ./12-desktop.sh [DESKTOP]   xfce unless given: kde gnome xfce i3 sway hyprland bspwm awesome
#   ./12-desktop.sh --remove    remove the machine this script made
#
# BASE (debian-13; vmx desktops says the bases each desktop is proven on)
# and NAME (desk-DESKTOP) change it.
# Examples:
#   ./12-desktop.sh                            desk-xfce, Xfce on Debian 13
#   ./12-desktop.sh kde                        desk-kde
#   BASE=fedora-44 ./12-desktop.sh gnome       GNOME on Fedora 44
#   NAME=office BASE=ubuntu-24.04 ./12-desktop.sh kde
#   NAME=office ./12-desktop.sh --remove
# Run it on the vmx host, as root.
set -euo pipefail

desktop=${1:-xfce}
base=${BASE:-debian-13}
here=$(cd "$(dirname "$0")" && pwd)

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
state() { vmx list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .state'; }

if [ "$desktop" = --remove ]; then
	shopt -s nullglob
	files=("$here"/.made/12-desktop-"${NAME:-*}".made)
	[ ${#files[@]} -gt 0 ] || die "nothing to remove"
	for f in "${files[@]}"; do
		n=$(cat "$f")
		if [ -n "$(state "$n")" ]; then vmx remove "$n"; fi
		rm -f "$f"
	done
	exit 0
fi
name=${NAME:-desk-$desktop}
made=$here/.made/12-desktop-$name.made
[ -f "$made" ] && die "this script made $name already: NAME=$name ${0##*/} --remove takes it away first"
[ -n "$(state "$name")" ] && die "a machine named $name is there already"
mkdir -p "$(dirname "$made")"
echo "$name" >"$made"
began=$(date +%s)
# The build waits until the desktop's user is logged in on its own screen.
vmx build "$name" --base "$base" --desktop "$desktop"
echo "   $name: $desktop on $base, logged in on its screen, $(( $(date +%s) - began )) s"
echo "   its screen: on this host's desktop under Machines, or in a terminal: vmx tui screen $name"
echo
echo "Take it away with: NAME=$name ${0##*/} --remove"
