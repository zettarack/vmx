#!/usr/bin/env bash
# Your own golden, the whole way: a machine cloned from a server template
# and set up by a cloud-config of yours (golden-web.yaml: nginx and a page
# that names the machine), cut into a Firecracker golden once it has powered
# itself off, and COUNT microVMs cloned from it, each serving its own page.
# It is the template for a golden of your own: change golden-web.yaml.
#
#   ./04-golden-web.sh [COUNT]   build the base, cut the golden, start COUNT clones (3 unless given)
#   ./04-golden-web.sh --remove  remove the clones, the golden and the base
#
# BASE (debian-13-server), USER_DATA (golden-web.yaml beside this script),
# NAME (web: the base is NAME-base, the golden NAME, the clones NAME-01 and
# on), SSH_KEY (none: no login), CPUS (1) and MEMORY (512M) change it.
# Examples:
#   ./04-golden-web.sh                         web-base, the golden web, web-01 to web-03
#   ./04-golden-web.sh 10                      ten clones
#   NAME=shop ./04-golden-web.sh 2             shop-base, the golden shop, shop-01 and shop-02
#   BASE=ubuntu-24.04-server NAME=u24 ./04-golden-web.sh 2          from another template
#   USER_DATA=/home/myapp.yaml NAME=myapp ./04-golden-web.sh 3      your own cloud-config
#   SSH_KEY=/root/my-key.pub CPUS=2 MEMORY=1G ./04-golden-web.sh 2  your key, 2 CPUs and 1 GiB each
#   ./04-golden-web.sh --remove                (NAME=shop ./04-golden-web.sh --remove for shop)
# Run it on the vmx host, as root.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
template=${BASE:-debian-13-server}
user_data=${USER_DATA:-$here/golden-web.yaml}
name=${NAME:-web}
base=$name-base
# What this script made, by exact name, beside it: --remove takes only those.
made=$here/.made/04-golden-web-$name.made

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
machine_state() { vmx list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .state'; }
has_microvm() { vmx microvm list --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }
has_golden() { vmx microvm goldens --json | jq -e --arg n "$1" 'any(.[]; .name == $n)' >/dev/null; }
address() { vmx microvm list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .address // empty'; }

if [ "${1:-}" = --remove ]; then
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	# Clones first, then the golden (refused while a clone is cut from it),
	# then the machine it was cut from.
	grep '^microvm ' "$made" | while read -r _ n; do if has_microvm "$n"; then vmx microvm remove "$n"; fi; done
	grep '^golden ' "$made" | while read -r _ n; do if has_golden "$n"; then vmx microvm golden-remove "$n"; fi; done
	grep '^machine ' "$made" | while read -r _ n; do if [ -n "$(machine_state "$n")" ]; then vmx remove "$n"; fi; done
	rm -f "$made"
	exit 0
fi
count=${1:-3}
[[ $count =~ ^[0-9]+$ ]] && [ "$count" -ge 1 ] && [ "$count" -le 50 ] || die "COUNT is 1 to 50"
[ -f "$user_data" ] || die "no cloud-config at $user_data"
[ -f "$made" ] && die "this script made $name already: ${0##*/} --remove takes it away first"
[ "$(machine_state "$template")" = "" ] && ! vmx list --json | jq -e --arg n "$template" 'any(.[]; .name == $n and .kind == "template")' >/dev/null &&
	die "there is no template named $template here: vmx list shows the templates"
[ -n "$(machine_state "$base")" ] && die "a machine named $base is there already"
has_golden "$name" && die "a golden named $name is there already"
mkdir -p "$(dirname "$made")"

key=()
if [ -n "${SSH_KEY:-}" ]; then key=(--ssh-key "$SSH_KEY"); fi
echo "== 1. the base: $template, set up by $(basename "$user_data")"
echo "machine $base" >>"$made"
vmx clone "$template" "$base" --user-data "$user_data" "${key[@]}"
vmx start "$base"
echo "   it installs what the cloud-config asks and then powers itself off"
setup=$(date +%s)
for i in $(seq 1 240); do
	[ "$(machine_state "$base")" = shutoff ] && break
	sleep 5
	if [ $((i % 12)) -eq 0 ]; then echo "   still setting up, $((i * 5)) s"; fi
done
[ "$(machine_state "$base")" = shutoff ] || die "$base has not powered off after 20 minutes: vmx console $base shows where it is"
echo "   set up and powered off in $(( $(date +%s) - setup )) s"

echo "== 2. the golden, cut from it"
echo "golden $name" >>"$made"
vmx microvm golden "$base" --name "$name" --port 80

echo "== 3. $count microVMs from the golden"
for i in $(seq 1 "$count"); do
	n=$(printf '%s-%02d' "$name" "$i")
	has_microvm "$n" && die "a microVM named $n is there already"
	echo "microvm $n" >>"$made"
	vmx microvm clone "$name" "$n" --cpus "${CPUS:-1}" --memory "${MEMORY:-512M}" "${key[@]}" >/dev/null
	vmx microvm start "$n" >/dev/null
	echo "   $n started"
done

echo "== 4. each one's page, once its Debian has booted"
for i in $(seq 1 "$count"); do
	n=$(printf '%s-%02d' "$name" "$i")
	page=""
	for _ in $(seq 1 60); do
		a=$(address "$n")
		if [ -n "$a" ] && page=$(curl -s --max-time 2 "http://$a/"); then [ -n "$page" ] && break; fi
		sleep 3
	done
	echo "   $n at http://${a:-?}/ says: ${page:-nothing yet}"
done
echo
vmx microvm list
echo
echo "Change golden-web.yaml for your own application, ${0##*/} --remove, and run it again."
