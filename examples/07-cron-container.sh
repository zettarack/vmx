#!/usr/bin/env bash
# Scheduled tasks that last: cron in a container, its crontab given at the
# run and its work written to a named volume, which stays when the container
# goes and survives the host's upgrades. (A timer or crontab written on the
# vmx host itself is in its boot environment, and an upgrade starts a new
# one without it: scheduled work belongs in a container, a machine, or
# vmx's own schedules.)
#
#   ./07-cron-container.sh             run cron with the example crontab, show its first ticks
#   ./07-cron-container.sh --remove    remove the container and its volume
#
# CRONTAB is the crontab (one line a job); NAME (cron), VOLUME (cron-data)
# and IMAGE (alpine) change the rest.
# Examples:
#   ./07-cron-container.sh                     cron with the example crontab, every minute
#   CRONTAB='*/5 * * * * date >> /data/ticks.log' ./07-cron-container.sh      every five minutes
#   NAME=backup-cron VOLUME=backup-data ./07-cron-container.sh               other names
#   IMAGE=alpine:3.20 ./07-cron-container.sh   a given alpine
#   ./07-cron-container.sh --remove            (NAME=backup-cron VOLUME=backup-data ... for those)
# Run it on the vmx host, as root.
set -euo pipefail

image=${IMAGE:-alpine}
name=${NAME:-cron}
volume=${VOLUME:-cron-data}
# shellcheck disable=SC2016 # expanded by cron's shell, in the container
crontab=${CRONTAB:-'* * * * * echo "tick at $(date +%T): $(cat /proc/loadavg)" >> /data/ticks.log'}

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
vmx container list --json >/dev/null 2>&1 || die "this vmx has no containers: upgrade it to a release that has 'vmx container'"
has() { vmx container list --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }
has_volume() { vmx container volumes --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }

if [ "${1:-}" = --remove ]; then
	if has "$name"; then vmx container remove "$name"; fi
	if has_volume "$volume"; then vmx container volume-remove "$volume"; fi
	exit 0
fi
[ $# -eq 0 ] || die "usage: ${0##*/} [--remove]"
has "$name" && die "a container named $name is there already: ${0##*/} --remove takes it away"

vmx container pull "$image"
# The container's program: write the crontab, start busybox's crond in the
# background, and follow the log the jobs write, so that
# 'vmx container logs' shows each tick.
# shellcheck disable=SC2016 # expanded by the container's shell
program='printf "%s\n" "$CRONTAB" > /etc/crontabs/root && crond -b -l 8 && touch /data/ticks.log && exec tail -F /data/ticks.log'
vmx container run "$image" "$name" --volume "$volume:/data" --env "CRONTAB=$crontab" -- sh -c "$program"
echo
echo "== cron runs; the first ticks come within a minute or two"
for _ in $(seq 1 30); do
	vmx container logs "$name" | grep -q '^tick at' && break
	sleep 5
done
vmx container logs "$name" | grep '^tick at' | tail -3
echo
echo "The ticks are on the volume $volume (vmx container volumes); the container's log: vmx container logs $name"
echo "Take it away with: ${0##*/} --remove"
