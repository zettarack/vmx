# What vmx does: the feature matrix

Three families of guest, and what each can do. Every "yes" names the
command that does it; every "no" means vmx has no verb for it.

- **Machine**: a full KVM virtual machine with UEFI and Secure Boot. A
  server from a template, a desktop, an appliance from a recipe, your own
  golden, an install from an ISO, Windows: all are machines.
- **microVM**: a Firecracker microVM cut from a stopped machine's disk,
  run under the jailer as its own user.
- **Container**: a Docker or OCI image, each run in a Firecracker microVM
  of its own on the host's kernel.

## The guest families

| | Machine | microVM | Container |
|---|---|---|---|
| Made from | a template, an ISO, a recipe, Windows' installer | a golden cut from a stopped machine | an image pulled or imported |
| Made by | `vmx clone`, `vmx build`, `vmx create`, `vmx windows` | `vmx microvm clone` | `vmx container run` |
| Its disk | a ZFS clone of the template's golden (`create`: a new zvol) | a ZFS clone of the golden's snapshot | a ZFS clone of the image's golden |
| Starts in | a normal boot | a fraction of a second, then its system boots | under a second, straight into its program |
| Its own kernel | yes | yes, the golden's | its own instance of the host's kernel, booted in its own microVM |
| Operating systems | Linux, Windows, BSD, anything that boots on a PC | Linux | Linux images |
| A screen | yes: the desktop, `vmx tui screen NAME` | no | no |
| A serial console | `vmx console NAME` | no | its console as a log: `vmx container logs NAME` |
| Your ssh key at the first boot | `--ssh-key` (not on Windows or an ISO install) | `--ssh-key` | not a login: it runs a program |
| A fixed address | `--address A/P --gateway G --dns D` | no (a lease on the default network) | no (a lease on the default network) |
| Ports from the host | `vmx forward add NAME 8080:80` | no | `--port 8080:80` |
| CPUs and memory | `--cpus`, `--memory`; `vmx set` when stopped | `--cpus`, `--memory` | `--cpus`, `--memory` (1 and 256M by default) |
| More disks, bigger disks | `vmx disk add`, `vmx grow` | no | named volumes: `--volume NAME:/PATH` |
| More network cards | `vmx nic add NAME NETWORK` | no | no |
| Snapshots and rollback | `vmx snapshot`, `vmx rollback` | no | no (its volumes are kept apart from it) |
| Scheduled snapshots | `vmx schedule NAME --every 1h --keep 24` | no | no |
| Backup and restore | `vmx backup NAME --to DATASET`, `vmx restore` | no | no |
| Clone it | `vmx clone NAME NEW` (not Windows), `--at SNAPSHOT` | from its golden, as often as you like | `vmx container run` again |
| Become a template | `vmx template save NAME` | no | no |
| Cut a microVM golden from it | `vmx microvm golden NAME` | no | no |
| PCI and USB devices | `vmx pci give`, `vmx usb give`; the guest needs the drivers (the server templates' cloud kernels have none for USB) | no | no |
| Starts with the host | `vmx power NAME on --order N --delay S` | yes, when it was running | yes, when it was running |
| Runs a job to its end | no | no | yes: its exit status is in its log |
| Shown in | the desktop, `vmx tui` (VMs), `vmx list` | `vmx tui microvms`, `vmx microvm list` | `vmx container list` |

## The kinds of machine

| Kind | Made by | What is particular to it |
|---|---|---|
| Server from a template | `vmx clone debian-13-server web1` | a seed with its name and your key; the template's distribution, ready at its first boot |
| Desktop | `vmx build desk1 --base debian-13 --desktop xfce` | logs in by itself on its own screen as the user `desktop`; KDE, GNOME, Xfce, i3, sway, Hyprland, bspwm, awesome, on the bases each is proven on (`vmx desktops`) |
| Appliance from a recipe | `vmx build dns1 --recipe adguard` | installed at its first boot; vmx reads its health page every 5 s until it answers (`vmx recipes`) |
| Your own golden | `vmx build mine --base debian-13 --script setup.sh --golden` | your script at the first boot, then sealed: cloud-init state, machine-id, host keys and logins' keys removed |
| Server with the admin's kit | `vmx build tools1 --base debian-13 --server` | curl, jq, tmux, vim, rsync, bash-completion and wireguard-tools, by the family's names |
| From an ISO | `vmx create inst1 --iso debian-netinst.iso --disk 20G` | a blank disk and the ISO as a CD it boots while the disk is empty; you install it on its screen |
| Windows | `printf 'pw\n' \| vmx windows win11 --edition 11 --login me` | installed with nothing asked: Windows 11 Enterprise (a 90-day evaluation) or Server 2025 (180 days, can be licensed); up to 90 minutes |

## What every guest stands on

| | |
|---|---|
| Storage | ZFS: every template, machine, microVM, image and volume a dataset or zvol; clones in an instant, taking no space until they write |
| The host | a signed, read-only image on ZFS boot environments: an upgrade is a new one, going back is choosing the old one, by itself when the new one fails its health check ([BOOT-ENVIRONMENTS.md](BOOT-ENVIRONMENTS.md)) |
| Secure Boot | the host's boot is signed end to end; machines boot UEFI with Secure Boot unless a template says otherwise |
| Keys | the root pool can be unlocked by the TPM; `vmx host security` shows what protects the machine and each pool's key |
| Health | `vmx status`: every check ok, warn, failed, or not applying with why; a check that did not run fails |
| Jobs | every long change is a job with a plan; `--dry-run` shows the plan and changes nothing |
| Events | `vmx events`; sent on with `vmx events webhook --format ntfy\|gotify\|discord\|json` |
| Consoles | jewm on the host's screen, `vmx tui`, the `vmx` command, the API ([CONSOLES.md](CONSOLES.md)) |
| Networks | NAT on `default`, bridges, VLANs and private networks (`vmx net`) |
| Offline | the installer needs no network; images and releases can be carried in as files |
