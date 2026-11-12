#!/usr/bin/env bash
# A machine's data kept: a snapshot by hand, a schedule that snapshots it
# by itself, a backup to a dataset on another pool, and the machine
# restored from that backup after it was removed, the same machine again.
#
#   BACKUP_TO=tank/backups ./16-snapshots-backups.sh   all of it, on a machine of its own
#   ./16-snapshots-backups.sh --remove                 remove the machine (the backup stays)
#
# BACKUP_TO is a ZFS dataset that exists, not where the machines live: one
# on another pool is the backup that survives this pool. NAME (keep1) and
# TEMPLATE (debian-13-server) change the rest.
# Examples:
#   zfs create tank/backups; BACKUP_TO=tank/backups ./16-snapshots-backups.sh
#   BACKUP_TO=tank/backups NAME=db1 ./16-snapshots-backups.sh
#   NAME=db1 ./16-snapshots-backups.sh --remove; vmx backups --from tank/backups; zfs destroy -r tank/backups/ID
# Run it on the vmx host, as root.
set -euo pipefail

name=${NAME:-keep1}
template=${TEMPLATE:-debian-13-server}
made=$(cd "$(dirname "$0")" && pwd)/.made/16-snapshots-backups-$name.made

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
state() { vmx list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .state'; }
stopped() { for _ in $(seq 1 60); do [ "$(state "$1")" = shutoff ] && return 0; sleep 2; done; die "$1 has not stopped"; }

if [ "${1:-}" = --remove ]; then
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	if [ -n "$(state "$name")" ]; then vmx remove "$name"; fi
	echo "The backup stays in $(cat "$made"), under the machine's ID: vmx backups --from $(cat "$made") shows it, and zfs destroy -r $(cat "$made")/ID takes it away."
	rm -f "$made"
	exit 0
fi
[ $# -eq 0 ] || die "usage: ${0##*/} [--remove]"
[ -n "${BACKUP_TO:-}" ] || die "BACKUP_TO names a dataset on another pool: BACKUP_TO=tank/backups ${0##*/}"
zfs list -H -o name "$BACKUP_TO" >/dev/null || die "no dataset $BACKUP_TO: zfs create $BACKUP_TO first"
[ -f "$made" ] && die "this script made $name already: NAME=$name ${0##*/} --remove takes it away first"
[ -n "$(state "$name")" ] && die "a machine named $name is there already"
mkdir -p "$(dirname "$made")"
echo "$BACKUP_TO" >"$made"

echo "== 1. a machine, and a snapshot of it"
vmx clone "$template" "$name"
vmx start "$name"
vmx snapshot "$name" first
echo "== 2. a schedule: a snapshot every hour, the newest 24 kept"
vmx schedule "$name" --every 1h --keep 24
vmx schedule | grep -E "^NAME|^$name "
echo "== 3. a backup to $BACKUP_TO, while it runs"
vmx backup "$name" --to "$BACKUP_TO"
vmx backups --from "$BACKUP_TO" | grep -E "^NAME|$name"
echo "== 4. the machine removed, and restored from its backup"
vmx schedule "$name" off
vmx stop "$name"
stopped "$name"
vmx remove "$name"
vmx restore "$name" --from "$BACKUP_TO"
vmx start "$name"
vmx snapshots "$name"
echo
vmx list | grep -E "^NAME|^$name "
echo
echo "Take it away with: NAME=$name ${0##*/} --remove (the backup stays in $BACKUP_TO, under the machine's ID)"
