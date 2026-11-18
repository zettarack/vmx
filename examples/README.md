# vmx examples

Zettarack is infrastructure built from the ground up for demanding system
operators, administrators and enthusiasts: one machine, made as fast and
as sure as it can be, run from its own screen through jewm (just enough
window manager) and the text console. vmx is its hypervisor.

**vmx runs every kind of machine on one box**: Windows and Linux desktops,
servers from templates, appliances from recipes, anything from an
installer ISO, Firecracker microVMs, and Docker and OCI images, each in a
microVM of its own. All of them live on ZFS from the boot up: the host
and everything it runs are datasets and zvols, cloned in an instant,
snapshotted and rolled back the same way.

This folder is a set of scripts that each do one whole thing, the
command sheet with every common command written out, and templates for
your own workloads. Every script was run on a vmx host before it was
published, and each says what it needs.

## Start here

The examples run on the vmx host, as root. The appliance installs
nothing, git included, so fetch them with curl, into `/home`, which
keeps its contents across upgrades:

```sh
cd /home
curl -L https://github.com/zettarack/vmx/archive/refs/heads/main.tar.gz | tar -xz
cd vmx-main/examples
./01-hello-container.sh
```

Or clone on your own computer and copy the folder over:

```sh
git clone https://github.com/zettarack/vmx
scp -r vmx/examples root@vmx-host:/home/
```

Every script takes `--remove`, which removes exactly what that script
made, by the names it wrote down beside itself in `.made/`, and nothing
else.

## The scripts

| Script | What it does |
|---|---|
| `01-hello-container.sh` | Docker's hello-world, pulled and run as a Firecracker microVM of its own; what it printed and how it ended. |
| `02-microvms.sh [COUNT]` | A row of microVMs from the `debian-13` golden every vmx builds at its first boot. |
| `03-web-containers.sh [COUNT]` | nginx, COUNT copies, each led to from a port of the host. |
| `04-golden-web.sh [COUNT]` | Your own golden the whole way: a server set up by `golden-web.yaml`, cut into a microVM golden, COUNT clones each serving its own page. |
| `05-demoscene-wall.sh [--from DIR]` | Forty containers drawing one animated picture together in your browser (`scene/`), each counting its frames on its disk and on a volume of its own that outlives its machine; its keeper runs as a service, does what the page's buttons ask (snapshot, restore, rebuild, kill, clone, remove) and replaces a tile that goes; the page's info button says how it all works. |
| `06-one-shot-job.sh ['COMMAND']` | A job run to its end in a microVM made for it, the way a function service runs one, its output and exit status kept. |
| `07-cron-container.sh` | Scheduled tasks that last: cron in a container, its work on a volume. |
| `my-image.sh` | The template for your own image: from a registry or an archive, COUNT copies, ports and environment. |
| `10-server-vm.sh` | A server from a template with your key, the ssh line once it answers, a snapshot and a rollback. |
| `11-recipe-app.sh [RECIPE]` | An application from a recipe (AdGuard Home, Jellyfin, Plex, a LAMP stack and more), built until its health page answers, and led to from a port of the host. |
| `12-desktop.sh [DESKTOP]` | A Linux desktop in a machine, logged in on its own screen: KDE, GNOME, Xfce, i3, sway and more. |
| `13-iso-install.sh` | A machine from an installer ISO: Debian's netinst fetched and checked, a blank disk, its installer on the screen. |
| `14-windows.sh` | Windows 11 or Server 2025, installed with nothing asked. |
| `15-passthrough.sh` | A USB or PCI device of the host given to a machine while it runs. |
| `16-snapshots-backups.sh` | A snapshot, a schedule, a backup to another pool, and the machine restored from it. |

## What to expect

Measured running these scripts on one vmx host (a 24-thread AMD desktop
with 64 GiB):

| | |
|---|---|
| A microVM from a golden, cloned and started | about 0.4 s each |
| Its Debian answering on the network after that | about 15 s (a web page); its ssh after about 70 s |
| A container run | the command returns in about half a second |
| A one-shot job, from asking to its end | about 8.5 s for a job that does almost nothing |
| A golden of your own (`04-golden-web.sh`): the base set up with nginx and powered off | 22 s |
| 40 containers run, sixteen at a time, each with a volume of its own made for it | all 40 drawing the wall 5.3 s after the first was run, about 300 frames a second together |
| A server from a template, from the clone to its ssh answering | 12 to 23 s |
| The same server stopped, snapshotted, started; stopped, rolled back, started | ssh again 10 s after each start |
| An application from a recipe (AdGuard Home), built until its health page answers | 33 s |
| A desktop (Xfce on Debian 13), built until its user is logged in on its screen | 165 s |
| A backup of a running machine to another pool, the machine removed, and restored from it | under 2 minutes, start to end |
| Windows 11 (`14-windows.sh`), from the ask to its logon screen, its 8.5 GiB of downloads included | 14 minutes |

## The pages

| Page | What it covers |
|---|---|
| [INSTALL.md](INSTALL.md) | From bare metal to a running vmx: the stick, the six pages, the first start, the first hour |
| [FEATURES.md](FEATURES.md) | The feature matrix: machines, microVMs and containers, what each can do and by which command |
| [COMMANDS.md](COMMANDS.md) | Every common command, written out with real names |
| [CONSOLES.md](CONSOLES.md) | jewm's keys, the text console's tabs and keys, the command, the API |
| [IMAGES.md](IMAGES.md) | Your own image, built with Podman or Docker, from a registry or as a file |
| [STORAGE.md](STORAGE.md) | Snapshots, schedules, backups, restores, templates and goldens |
| [NETWORKING.md](NETWORKING.md) | NAT, bridges to your LAN, VLANs, private networks, more cards, ports |
| [BOOT-ENVIRONMENTS.md](BOOT-ENVIRONMENTS.md) | How the host upgrades, the trial of a first boot, and going back |
| [SECURITY.md](SECURITY.md) | Secure Boot, the TPM, every pool's key, unlocking a data pool |
| [scene/README.md](scene/README.md) | The demoscene wall's two programs, and building their images |

## Every kind of machine

| Kind | Command | Starts in | Use it for |
|---|---|---|---|
| Windows | `vmx windows` | an unattended install, up to 90 minutes | Windows applications |
| Desktop | `vmx build --desktop` | a normal boot | a Linux workstation in a machine |
| Installer ISO | `vmx create --iso` | you install it on its screen | any system that boots on a PC |
| Server from a template | `vmx clone` | the clone at once, then a normal boot | ordinary Linux servers |
| Appliance from a recipe | `vmx build --recipe` | minutes; vmx waits for its health page | a packaged application in one command |
| Your own golden | `vmx build --script --golden` | as its base | your standard build, cloned many times |
| Devices given | `vmx pci give`, `vmx usb give` | the machine's own boot | GPUs and cards, USB devices |
| Firecracker microVM | `vmx microvm` | in a fraction of a second, then its system boots | many small Linux services |
| Container | `vmx container` | in under a second | an image you have, or a job to run to its end |

## Where your services and scheduled work belong

The vmx host is an appliance: a signed, read-only image whose upgrade is
a new boot environment. What is written into the host's own `/etc`,
`/root`, `/srv` or `/var/spool` belongs to the boot environment it was
written in, and an upgrade starts from a new one without it. So a
timer or a crontab made on the host does not last, and the host has no
cron. Long-running services and scheduled work go:

- in a container: a service with `--port`, a scheduler like
  `07-cron-container.sh`, its data on a named volume (`--volume`);
- in a machine or a microVM, which are ordinary mutable Linux with
  their own cron and systemd timers, set up by a cloud-config like
  `golden-web.yaml`;
- in vmx's own schedules: `vmx schedule NAME --every 1h --keep 24`
  snapshots a machine, and the pools scrub on their own schedule.

`/home`, `/var/tmp` and `/var/lib/zettarack` keep their contents across
upgrades: keep these examples in `/home`.

## The Zettarack appliances

| Appliance | Stage | What it is |
|---|---|---|
| vmx | beta, release candidate 1 next | The hypervisor: every kind of machine on one box, with its own desktop and text console. |
| zxp | alpha | The storage server: ZFS pools, shares, snapshots and replication, with a file explorer, on its own box. |
| kbx | alpha | Kubernetes as an appliance: a highly available cluster whose nodes are machines on a vmx. |

Each is one signed image on the same core, installed by Bootmass. vmx
and zxp each serve on their own; kbx runs its cluster on a vmx.

## How vmx differs from other hypervisors

Said fairly, as examples, not as a contest:

- **ZFS from the boot up, upgrades included.** The host boots from ZFS
  boot environments: an upgrade is a new one, and going back is
  choosing the old one, which happens by itself when the new one fails
  its health check. Every template, machine, microVM, container image
  and volume is a dataset or a zvol. Proxmox can run from ZFS and clone
  machines with it too; it upgrades in place, with packages.
- **The images are the product.** Goldens sealed by plan, recipes that
  wait for the application's own health page, unattended Windows,
  Firecracker microVMs cut from any stopped machine, and Docker images
  each in a microVM of its own: the host's kernel booted inside it, a
  virtual machine's wall around every container, where a container
  elsewhere is processes in the host's own running kernel.
- **An appliance you sit at.** Nothing is installed on the host; Secure
  Boot is signed end to end; an upgrade that fails its health check
  goes back by itself. You drive it from its own screen, with jewm and
  the text console, not only from a browser on another computer.
- **One box first.** vmx does not move running machines between hosts,
  balance them across a cluster or keep a shared datastore, which the
  cluster products do. kbx is the answer for many machines.

## Licence

The example code in this folder is MIT (see `LICENSE`). vmx itself is
closed source and free of charge.
