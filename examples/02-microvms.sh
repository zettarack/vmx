#!/usr/bin/env bash
# A row of Firecracker microVMs from the debian-13 golden every vmx builds
# at its first boot: each a ZFS clone of the golden, made in a moment and
# started in a second; Debian inside then boots as it does anywhere.
#
#   ./02-microvms.sh [COUNT]   make COUNT microVMs (3 unless given) and start them
#   ./02-microvms.sh --remove  remove the ones this script made
#
# SSH_KEY=/root/my.pub adds your key, for: ssh debian@ADDRESS (from the host).
# PREFIX (micro), GOLDEN (debian-13), CPUS (1) and MEMORY (512M) change the rest.
# Examples:
#   ./02-microvms.sh                           micro-01 to micro-03
#   ./02-microvms.sh 10                        micro-01 to micro-10
#   SSH_KEY=/root/my-key.pub ./02-microvms.sh 2           with your key: ssh debian@ADDRESS
#   PREFIX=api CPUS=2 MEMORY=1G ./02-microvms.sh 4        api-01 to api-04, 2 CPUs and 1 GiB each
#   GOLDEN=debian-13 PREFIX=api ./02-microvms.sh 1        from the golden named debian-13
#   ./02-microvms.sh --remove                  every microVM this script made
# Run it on the vmx host, as root.
set -euo pipefail

prefix=${PREFIX:-micro}
golden=${GOLDEN:-debian-13}
# What this script made, by exact name, beside it: --remove takes only those.
made=$(cd "$(dirname "$0")" && pwd)/.made/02-microvms.made

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
has() { vmx microvm list --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }
address() { vmx microvm list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .address // empty'; }

if [ "${1:-}" = --remove ]; then
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	while read -r name; do
		if has "$name"; then vmx microvm remove "$name"; fi
	done <"$made"
	rm -f "$made"
	exit 0
fi
count=${1:-3}
[[ $count =~ ^[0-9]+$ ]] && [ "$count" -ge 1 ] && [ "$count" -le 99 ] || die "COUNT is 1 to 99"
vmx microvm goldens --json | jq -e --arg g "$golden" 'any(.[]; .name == $g)' >/dev/null ||
	die "there is no microVM golden named $golden here: vmx microvm goldens lists them"
[ -f "$made" ] && die "this script made microVMs already: ${0##*/} --remove takes them away first"

key=()
if [ -n "${SSH_KEY:-}" ]; then key=(--ssh-key "$SSH_KEY"); fi
mkdir -p "$(dirname "$made")"
start=$(date +%s%N)
# Two digits whatever the count: seq -w pads only to the widest number.
for i in $(seq 1 "$count"); do
	name=$(printf '%s-%02d' "$prefix" "$i")
	has "$name" && die "a microVM named $name is there already"
	echo "$name" >>"$made"
	vmx microvm clone "$golden" "$name" --cpus "${CPUS:-1}" --memory "${MEMORY:-512M}" "${key[@]}" >/dev/null
	vmx microvm start "$name" >/dev/null
	echo "$name started, $(( ($(date +%s%N) - start) / 1000000 )) ms after the first clone"
done
echo
echo "== their addresses, as they come"
# Two digits whatever the count: seq -w pads only to the widest number.
for i in $(seq 1 "$count"); do
	name=$(printf '%s-%02d' "$prefix" "$i")
	for _ in $(seq 1 30); do [ -n "$(address "$name")" ] && break; sleep 1; done
done
vmx microvm list
echo
echo "Debian takes about a minute inside each before ssh answers."
[ ${#key[@]} -gt 0 ] && echo "Then, from this host: ssh debian@$(address "$prefix-01")"
echo "Take them away with: ${0##*/} --remove"
