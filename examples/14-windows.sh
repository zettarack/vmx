#!/usr/bin/env bash
# Windows, installed with nothing asked: Microsoft's evaluation ISO and the
# Windows driver disc fetched the first time, setup answered from start to
# finish, and the machine left at its logon screen, nobody logged on. Up to
# 90 minutes, then up to 15 more for its first start from its own disk.
#
#   PASSWORD_FILE=/root/win.pass ./14-windows.sh   Windows 11 Enterprise (a 90-day evaluation) as win11
#   ./14-windows.sh --remove                       remove the machine (the ISOs stay)
#
# PASSWORD_FILE holds the administrator's password, one line; it is read on
# stdin, never as an argument. EDITION (11, or server: Windows Server 2025,
# 180 days, which can be licensed), LOGIN (me) and NAME (win11) change it.
# Server wants 8 or more characters with three of upper case, lower case,
# digits and symbols, and not the user's name.
# Examples:
#   printf 'Correct-Horse-9\n' > /root/win.pass; chmod 600 /root/win.pass
#   PASSWORD_FILE=/root/win.pass ./14-windows.sh
#   PASSWORD_FILE=/root/win.pass EDITION=server LOGIN=admin NAME=srv1 ./14-windows.sh
#   NAME=srv1 ./14-windows.sh --remove
# Run it on the vmx host, as root.
set -euo pipefail

name=${NAME:-win11}
made=$(cd "$(dirname "$0")" && pwd)/.made/14-windows-$name.made

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
state() { vmx list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .state'; }

if [ "${1:-}" = --remove ]; then
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	if [ -n "$(state "$name")" ]; then vmx remove "$name"; fi
	rm -f "$made"
	exit 0
fi
[ $# -eq 0 ] || die "usage: ${0##*/} [--remove]"
[ -n "${PASSWORD_FILE:-}" ] && [ -f "$PASSWORD_FILE" ] || die "PASSWORD_FILE names a file holding the administrator's password, one line"
[ -f "$made" ] && die "this script made $name already: NAME=$name ${0##*/} --remove takes it away first"
[ -n "$(state "$name")" ] && die "a machine named $name is there already"
mkdir -p "$(dirname "$made")"
echo "$name" >"$made"
began=$(date +%s)
vmx windows "$name" --edition "${EDITION:-11}" --login "${LOGIN:-me}" <"$PASSWORD_FILE"
echo "   $name is at its logon screen, $(( ($(date +%s) - began) / 60 )) minutes after the ask"
echo "   its screen: on this host's desktop under Machines (Super+Escape gives it every key), or vmx tui screen $name"
