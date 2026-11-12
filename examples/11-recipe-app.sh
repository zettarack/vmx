#!/usr/bin/env bash
# An application from a recipe, in one command: vmx fetches the recipe's
# base template if it is not here, clones a machine from it with the
# recipe's own cloud-config, grows its disk, starts it, and reads the
# application's health page every 5 s until it answers. Then a port of this
# host leads to the application, so your network reaches it.
#
#   ./11-recipe-app.sh [RECIPE]   build RECIPE (adguard unless given) and lead a port to it
#   ./11-recipe-app.sh --remove   remove the machine and the port this script made
#
# The recipes: adguard icecast jellyfin lamp plex rdp-desktop sdr seedbox
# syncthing tvheadend vdi-desktop web-stack (vmx recipes says what each is).
# NAME (the recipe's name), PORT (this host's port: the application's own
# unless given) and SSH_KEY (none: no login) change it.
# Examples:
#   ./11-recipe-app.sh                         AdGuard Home, its wizard on this host's 3000
#   ./11-recipe-app.sh jellyfin                Jellyfin on 8096
#   NAME=dns2 PORT=3300 ./11-recipe-app.sh adguard        a second one, on 3300
#   SSH_KEY=/root/my-key.pub ./11-recipe-app.sh lamp       with your key, for ssh
#   NAME=dns2 ./11-recipe-app.sh --remove
# Run it on the vmx host, as root.
set -euo pipefail

recipe=${1:-adguard}
name=${NAME:-$recipe}
here=$(cd "$(dirname "$0")" && pwd)

die() { echo "${0##*/}: $*" >&2; exit 1; }
command -v vmx >/dev/null || die "there is no vmx command here: run this on the vmx host"
command -v jq >/dev/null || die "this needs jq"
state() { vmx list --json | jq -r --arg n "$1" '.[] | select(.name == $n) | .state'; }

if [ "$recipe" = --remove ]; then
	name=${NAME:-}
	if [ -z "$name" ]; then
		f=$(find "$here/.made" -maxdepth 1 -name '11-recipe-app-*.made' 2>/dev/null | head -1)
		[ -n "$f" ] || die "nothing to remove"
		name=$(head -1 "$f")
	fi
	made=$here/.made/11-recipe-app-$name.made
	[ -f "$made" ] || die "nothing to remove: $made is not there"
	port=$(sed -n 2p "$made")
	if [ -n "$port" ] && [ -n "$(state "$name")" ]; then
		# The one forward this script made, by its machine and port.
		vmx forward remove "$name" "$port" || echo "${0##*/}: the forward $port to $name was not removed (above); vmx forward list shows the rest" >&2
	fi
	if [ -n "$(state "$name")" ]; then vmx remove "$name"; fi
	rm -f "$made"
	exit 0
fi
made=$here/.made/11-recipe-app-$name.made
app=$(vmx recipes --json | jq -c --arg r "$recipe" '.[] | select(.name == $r)')
[ -n "$app" ] || die "no recipe named $recipe: vmx recipes lists them"
[ -f "$made" ] && die "this script made $name already: NAME=$name ${0##*/} --remove takes it away first"
[ -n "$(state "$name")" ] && die "a machine named $name is there already"
app_port=$(printf '%s' "$app" | jq -r '.health.port // empty')
port=${PORT:-$app_port}
mkdir -p "$(dirname "$made")"

key=()
if [ -n "${SSH_KEY:-}" ]; then key=(--ssh-key "$SSH_KEY"); fi
echo "== $(printf '%s' "$app" | jq -r .title): $(printf '%s' "$app" | jq -r .summary)"
printf '%s\n%s\n' "$name" "$port" >"$made"
began=$(date +%s)
# The build waits for the application's own health page.
vmx build "$name" --recipe "$recipe" "${key[@]}"
echo "   built and answering, $(( $(date +%s) - began )) s"
if [ -n "$app_port" ]; then
	# A forward leads this host's port to the machine on its NAT network.
	vmx forward add "$name" "$port:$app_port"
	host=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") { print $(i + 1); exit }}')
	echo "   from your network: http://${host:-THIS-HOST}:$port$(printf '%s' "$app" | jq -r '.health.path // "/"')"
fi
echo
echo "Take it away with: NAME=$name ${0##*/} --remove"
