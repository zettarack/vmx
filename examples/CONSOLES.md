# The consoles: jewm, the text console, the command and the API

A vmx host is run from its own screen. Four ways in, one truth behind
them: the desktop, the text console and the `vmx` command are all
clients of the daemon's API, and none of them runs a tool of its own. A
machine made in one is in the others at once, and what any of them
changes, it changes through the same job, with the same plan.

| Way in | Where | For |
|---|---|---|
| jewm, the desktop | the host's own screen | every machine at keyboard speed, their screens full size with sound |
| `vmx tui`, the text console | any terminal, over ssh too | the same, in text; a tmux console on the host's text screens |
| `vmx`, the command | any shell on the host | scripts, and every verb with `--dry-run` |
| the API | the daemon's local socket | your own tools; `--json` on every read shows its shape |

## jewm, the desktop

jewm (just enough window manager) is the host's desktop: a bar along the
top, and under it the console, full screen. It runs as the console's
user, never as root; that user changes machines through the daemon by
being in the admin group. A key is the one in its place on a US
keyboard, whatever the layout.

| Keys | |
|---|---|
| Super | every machine at once (the fly-through) |
| Super+Tab | the wall of every machine |
| Super+a | the console, from anywhere |
| Super+m, c, s, g, n, r, y, w | the menus: Machines, Create, Storage, Configure, Network, Reports, System, Power |
| Super+i | the status: the host, its machines, the desktops |
| Super+/ | every key, on the screen |
| Super+Enter | a terminal |
| Super+t | the console window |
| Super+, | the settings |
| Super+l | lock the screen |
| Super+Escape | every key to the machine shown, its own Alt+Tab and Windows key too; the same chord gives them back |
| Super+v | the desktop's clipboard to the machine shown |
| Super+Shift+v | the clipboard typed into the machine shown, as keys |
| Super+1 2 3 4 | desktop 1 to 4 (Super+arrows too) |
| Super+Shift+1 2 3 4 | the machine on screen to that desktop |
| Super+Shift+5 to 0 | pin that machine to desktop 5 to 9 or 0 |
| Ctrl+Shift+Left, Right | tile to that half |
| Alt+Tab | the next task, and its desktop |
| Super+q, Alt+F4 | close the window the keys go to |

In the console, `w` on a machine shows its screen full size, with its
sound, and Super+a brings the console back. The manual is under System,
Manual, one page per screen.

## The text console

```sh
vmx tui                    # the console, on its first tab
vmx tui microvms           # on the microVMs tab
vmx tui keys --print       # the Keys tab printed once, no console
vmx tui screen web1        # web1's screen in this terminal: sixels where the terminal draws them, else half-blocks
vmx tui screen web1 --blocks   # half-block cells always; ctrl+] is the menu, d detaches
```

One line of tabs, each with its number key:

| Tab | |
|---|---|
| 1 VMs | every machine |
| 2 Snapshots | one machine's after Enter on it, else every machine's |
| 3 Networks | the networks and the host's cards |
| 4 microVMs | the goldens and the microVMs cut from them |
| 5 Hardware | PCI and USB devices and which machine has them |
| 6 Recipes | what a machine can be built from |
| 7 Templates | the golden set and what of it is built, then every other template |
| 8 Images | the catalogue's cloud images and the ISOs on the host |
| 9 Reconcile | what libvirt, ZFS and the records disagree on |
| 0 Overview | what the host runs, its health, how many machines |
| Jobs | every job the daemon remembers; Enter follows one, X cancels a running one |
| Health | every check, with its remedy |
| Accounts | the machine's logins |
| Keys | what protects the machine: Secure Boot, the TPM, each pool's key |

| Keys | |
|---|---|
| 1 to 0, tab, [ ] | a tab by its number, or the next and previous |
| j k, ↑ ↓ | the rows |
| → or l | the row's page: its vitals over its verbs; Enter runs one |
| enter | a machine's screen; elsewhere, drill in |
| ← or h | a machine's snapshots |
| space | mark the row; a verb then runs on every marked row; ctrl+a marks all |
| / | filter the rows; Enter keeps the filter, Esc clears it |
| o | sort by the next column, then descending |
| i | show or hide the vitals pane |
| r | reload the tab now |
| : | the palette: every tab and every verb, by name |
| ctrl+j | the newest job's pane |
| ctrl+t | a shell on this host, in the console |
| ? | the keys, with every verb of this tab |
| q | quit |

A verb's key is in the menu at the foot of each tab, marked in its
label; a verb that changes something asks first, and one that erases
asks for the name to be typed.

## The command

Every verb is in `vmx --help`, and every one that changes something
takes `--dry-run`, which prints the plan and does nothing:

```sh
vmx clone debian-13-server web1 --dry-run
vmx microvm golden web1 --name web --dry-run
vmx container run nginx:alpine web-01 --port 8081:80 --dry-run
```

Every read takes `--json`, which is the API's own answer:

```sh
vmx list --json | jq -r '.[] | select(.kind == "machine") | .name'
vmx microvm list --json | jq -r '.[] | "\(.name) \(.state) \(.address)"'
vmx container list --json | jq -r '.[] | select(.state == "running") | .name'
```

A long change is a job: the command follows it to its end and says each
step; `vmx jobs` lists them, `vmx jobs ID` shows one, and
`vmx jobs cancel ID` stops one that runs.
