#!/usr/bin/env bash
# A one-shot job, the way a function service runs one: a container made for
# the run, its own Firecracker microVM, its program run to the end, what it
# printed and how it ended kept, and the container removed after.
#
#   ./06-one-shot-job.sh                       the default job (a sum and the kernel)
#   ./06-one-shot-job.sh 'COMMAND ...'         your own shell command, in alpine
#
# IMAGE (alpine) runs it; KEEP=1 leaves the container to read again
# (vmx container logs NAME). The script ends with the job's exit status.
# Examples:
#   ./06-one-shot-job.sh                       the default job
#   ./06-one-shot-job.sh 'date; uname -a'      a command of your own
#   ./06-one-shot-job.sh 'exit 3'; echo $?     the job's own status, 3
#   IMAGE=debian:13 ./06-one-shot-job.sh 'cat /etc/os-release'   in another image
#   NAME=report KEEP=1 ./06-one-shot-job.sh 'df -h'              kept: vmx container logs report
# Run it on the vmx host, as root.
set -euo pipefail

image=${IMAGE:-alpine}
name=${NAME:-job-$(date +%H%M%S)}
# shellcheck disable=SC2016 # expanded by the job's own shell, in the container
job=${1:-'echo "6 times 7 is $((6 * 7)), worked out on $(uname -r)"; ls /'}

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
vmx container list --json >/dev/null 2>&1 || die "this vmx has no containers: upgrade it to a release that has 'vmx container'"
state() { vmx container list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .state'; }
[ -z "$(state "$name")" ] || die "a container named $name is there already"

vmx container pull "$image" >/dev/null
began=$(date +%s%N)
# Everything after -- is the command, as after the image in docker run.
vmx container run "$image" "$name" -- sh -c "$job" >/dev/null
for _ in $(seq 1 300); do
	[ "$(state "$name")" = stopped ] && break
	sleep 1
done
took=$(( ($(date +%s%N) - began) / 1000000 ))
[ "$(state "$name")" = stopped ] || die "$name has not ended after 300 s: vmx container logs $name shows where it is"
log=$(vmx container logs "$name")
status=$(printf '%s\n' "$log" | sed -n 's/^vmx-init: exit status \([0-9]*\)$/\1/p' | tail -1)
echo "== $name printed"
printf '%s\n' "$log" | grep -v -E '^[0-9]{4}-[0-9]{2}-[0-9]{2}T|^\[ *[0-9]+\.[0-9]+\] |^vmx-init: ' | sed 's/\\x0d$//'
echo "== it ended with status ${status:-unknown}, $took ms after the run was asked"
if [ "${KEEP:-}" != 1 ]; then
	out=$(vmx container remove "$name" 2>&1) || die "removing $name: $out"
fi
[ -n "$status" ] || die "no exit status in $name's log"
exit "$status"
