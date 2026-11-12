# Installing vmx

From bare metal to a running vmx: download one file, check it, write it
to a USB stick, boot it, answer six pages. The installer takes the disks
you point at, whole, and nothing else. It needs no network.

## What the machine needs

| | |
|---|---|
| CPU | 64-bit x86 with Intel VT-x or AMD-V turned on in the firmware |
| Firmware | UEFI. No BIOS boot. Secure Boot can stay on. |
| System disk | 8 GiB or more, taken whole. Two or more make a mirror or a raidz. |
| Memory | what your machines need, beside the host's own |
| Network | a wired card: DHCP, or a fixed address you type |
| More disks | optional, for a data pool |

## 1. Download and check

Each release is under Releases at <https://github.com/zettarack/vmx> and
at `dl.zettarack.com`: the installer ISO, `SHA256SUMS`, its signature
`SHA256SUMS.sig`, and the release key `release.pem`.

```sh
openssl pkeyutl -verify -pubin -inkey release.pem -rawin -in SHA256SUMS -sigfile SHA256SUMS.sig
sha256sum -c SHA256SUMS --ignore-missing
```

The first says `Signature Verified Successfully`; the second says `OK`
beside the ISO. The release key's public half is in this repository's
README, and every vmx image carries it in `/usr/lib/zettarack/release.pub`.

## 2. Write the stick

```sh
lsblk -d -o NAME,SIZE,MODEL                # find the stick: here it is sdX
iso=$(grep -o 'zettarack-vmx-[0-9.]*\.iso' SHA256SUMS | head -1)   # the release's ISO, by its own name
sudo dd if="$iso" of=/dev/sdX bs=4M status=progress conv=fsync
```

On Windows, Rufus in DD mode or balenaEtcher. Everything on the stick is
replaced.

## 3. Boot it

Choose the stick in the firmware's boot menu. With Secure Boot on, the
first boot queues two certificates and restarts into the firmware's key
manager, a blue screen: **Enroll MOK**, **Continue**, **Yes**, the
password **vmx**, **Reboot**. The stick starts again and installs.

On a machine with no screen, the stick's boot menu offers the same
install as text on the first serial port.

## 4. The six pages

One window, Back and Next at the foot of each page. Next stays dimmed,
with the reason beside it, until the page is complete. Every answer shown
is the installer's own.

1. **System**: the image to install, vmx.
2. **Disks**: click the tiles of the disks the system goes on: one disk,
   a mirror, raidz1 or raidz2. A disk the installer will not use is
   dimmed with its reason: the stick you booted from, a disk in use.
   Encryption is off unless you choose it.
3. **Network**: DHCP, or a fixed address on one card.
4. **Zone**: type part of a city's name and pick the time zone.
5. **Access**: the machine's name, an ssh key, and a user who logs in at
   the screen with a password. The key opens root over ssh, and that
   user too. There is no default password.
6. **Review**: the installer's own plan, every step it will take, and in
   red what each erases. Nothing has changed yet. **Erase and install**
   starts it; the steps run in view, and it ends with **Restart**.

If Secure Boot would refuse the installed system, Review says so and
offers **Repair Secure Boot** instead, with what it changes.

## 5. The first start

The machine starts locked: the password of the user from the Access
page opens it. Behind the lock is its own desktop: a bar along the top,
and under it the console, full screen.

| Keys | |
|---|---|
| Super+a | the console, from anywhere |
| Super+c | Create: a machine, a microVM, a container, a golden |
| Super+m | Machines: every machine; start, stop, snapshot |
| Super+y | System: a terminal, the settings, the manual |
| Super+r | Reports: health, jobs, events |
| Super+l | lock the screen |
| Super | every machine at once |
| Super+/ | every key |

Or over ssh, with the key from the Access page:

```sh
ssh root@vmx1
vmx status                 # every health check: ok, warn, failed, or why it did not run
vmx list
bootmass upgrade --status  # the release running and the upgrade policy
```

## 6. The first hour: the goldens

A new machine is a ZFS clone of a template's golden snapshot, made in
under a second and taking no space until it writes. So build the goldens
first, from the desktop (Super+c, Golden, Golden set, Build) or here:

```sh
vmx goldens                                    # the set, and which are built
vmx goldens build debian-13-server debian-13-desktop --ssh-key /root/my-key.pub
vmx image known                                # the distributions' images vmx knows
```

Named, it builds those; with no names, every golden of the set, which is
most of a day. Each takes minutes: the packages install inside it from
that distribution's mirrors.

Then [COMMANDS.md](COMMANDS.md) for every common command, and the
scripts in this folder ([README.md](README.md)) for whole workloads.

## Upgrades

The installed host checks for a release every 15 minutes and, by
default, stages it for the next restart, which the console asks for.
How it upgrades and goes back is in
[BOOT-ENVIRONMENTS.md](BOOT-ENVIRONMENTS.md).
