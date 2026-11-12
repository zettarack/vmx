# vmx commands, filled in

Every common command, written out with real names, grouped by what you
are doing. Run them on the vmx host as root. The names fit together: the
key is `/root/my-key.pub`, the template `debian-13-server`, the machine
`web1`, the microVM golden `debian-13`, the microVM `demo-01`, the
container `web-01`. Add `--dry-run` to any command that changes something
to see what it would do without doing it. `vmx --help` is the whole
reference, and `man vmx` the long one.

## Before anything: a key, and what is here

```sh
# Your public key, copied to the host once, so the machines take it.
scp ~/.ssh/id_ed25519.pub root@vmx-host:/root/my-key.pub   # on your own computer

vmx help                      # every verb; vmx help container run, or vmx container --help, one verb's own
vmx status                    # the host's health, every check
vmx list                      # every template and machine
vmx goldens                   # the templates built at the first boot, and their state
vmx microvm goldens           # the microVM goldens
vmx container images          # the container images pulled
vmx jobs                      # what is running now, and what ran
vmx events                    # what happened, newest first
```

## Machines (full VMs)

```sh
# A server from a template, in an instant (a ZFS clone), with your key.
vmx clone debian-13-server web1 --ssh-key /root/my-key.pub
vmx clone debian-13-server web2 --cpus 4 --memory 8G --ssh-key /root/my-key.pub
vmx clone debian-13-server web3 --address 192.168.122.50/24 --gateway 192.168.122.1 --dns 192.168.122.1 --ssh-key /root/my-key.pub
vmx start web1
vmx list                      # its address once it has booted
ssh debian@192.168.122.50     # from the host; the user is the image's own (debian on Debian)

vmx stop web1                 # asks it to shut down
vmx reboot web1
vmx force-off web1            # pulls the plug
vmx set web1 --cpus 2 --memory 4G     # while it is stopped
vmx grow web1 root 40G        # a bigger disk (the disk's name: vmx disk list web1)
vmx disk add web1 data 100G   # a second disk
vmx disk list web1
vmx power web2 on --order 1 --delay 30     # starts with the host, first, the next 30 s after
vmx power web2 off                         # left as it is when the host starts
vmx keys web2 ctrl-alt-del                 # a key chord on its keyboard (vmx keys --help lists them)
vmx rename web1 www1
vmx remove www1
```

## Snapshots, schedules, backups

```sh
vmx snapshot web2 before-upgrade          # a snapshot with a label
vmx snapshots web2
vmx rollback web2 before-upgrade          # back to it (the machine is stopped first)
vmx snapshot web2 before-upgrade --delete
vmx clone web2 web2-test --at before-upgrade   # a new machine from a snapshot
vmx schedule web2 --every 1h --keep 24 --dry-run   # what it would do, changing nothing
vmx schedule web2 --every 1h --keep 24    # a snapshot every hour, the newest 24 kept
vmx schedule                              # every schedule, and when each runs next
vmx schedule web2 off
vmx template save web2                    # a stopped machine becomes a template
```

## Appliances from recipes

```sh
vmx recipes                               # what can be built, and the check it waits for
vmx build dns1 --recipe adguard --ssh-key /root/my-key.pub
vmx build media --recipe jellyfin --ssh-key /root/my-key.pub
vmx build site --recipe lamp --ssh-key /root/my-key.pub
```

## Desktops and servers of your own

```sh
vmx desktops                              # the desktops, and the bases each is proven on
vmx build desk1 --base debian-13 --desktop xfce
vmx build desk2 --base fedora-44 --desktop gnome
vmx build tools1 --base debian-13 --server --ssh-key /root/my-key.pub     # the admin's kit
vmx build mine --base debian-13 --script /root/setup.sh --golden          # your script, then sealed
```

## From an installer ISO

```sh
vmx isos                                  # the ISOs on this host
line=$(curl -s https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/SHA256SUMS | grep -m1 -- '-amd64-netinst.iso$')
vmx isos fetch debian-netinst.iso --url "https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/${line##* }" --sha256 "${line%% *}"
vmx create inst1 --iso debian-netinst.iso --disk 20G
vmx start inst1                           # then install it on its screen (vmx gui, or the desktop)
vmx eject inst1                           # the CD out, once it is installed
```

## Windows

```sh
# The administrator's password comes on stdin, one line; nothing else is asked.
# Up to 90 minutes, then Windows is at its logon screen.
printf 'Correct-Horse-9\n' | vmx windows win11 --edition 11 --login me
# Server: 8 or more, three of upper, lower, digits and symbols, not the user's name.
printf 'Correct-Horse-9\n' | vmx windows srv1 --edition server --login admin
```

## Devices given to a machine

```sh
vmx pci                                   # every device, and whether it can be given
vmx pci give web2 0000:05:00.0            # to the stopped machine; back when it stops
vmx pci take web2 0000:05:00.0
vmx usb
vmx usb give web2 8087:0025
vmx usb take web2 8087:0025
```

## Networks and ports

```sh
vmx net list                              # default is NAT on virbr0
vmx net leases                            # who has which address
vmx nic add web2 default
vmx forward add web2 8080:80              # this host's 8080 leads to web2's 80
vmx forward list
vmx forward remove web2 8080
```

## Firecracker microVMs

```sh
vmx microvm goldens
vmx microvm clone debian-13 demo-01 --cpus 1 --memory 512M --ssh-key /root/my-key.pub
vmx microvm start demo-01
vmx microvm list                          # its address
ssh debian@192.168.122.94                 # from the host, about a minute after the start
vmx microvm stop demo-01
vmx microvm remove demo-01

# A golden of your own, cut from any stopped machine.
vmx microvm golden web2 --name web --port 80
vmx microvm clone web web-a
vmx microvm golden-remove web             # once no microVM is cloned from it
```

## Containers (Docker and OCI images)

```sh
vmx container pull hello-world
vmx container run hello-world hello
vmx container logs hello                  # what it printed, and "vmx-init: exit status 0"
vmx container start hello                 # run it again
vmx container remove hello

vmx container pull nginx:alpine
vmx container run nginx:alpine web-01 --port 8081:80
vmx container run nginx:alpine web-02 --port 8082:80 --volume web-data:/usr/share/nginx/html --memory 512M
vmx container list                        # name, image, state, address, ports, volumes
vmx container stop web-01
vmx container start web-01
vmx container volumes
vmx container remove web-02
vmx container volume-remove web-data

# A command of your own, run to the end.
vmx container pull alpine
vmx container run alpine job-1 -- sh -c 'echo the answer is $((6 * 7))'
vmx container logs job-1

# Your own image, built elsewhere with podman or docker (IMAGES.md); the
# digest is the one the build machine printed for the archive, and nothing
# in the archive vouches for itself.
vmx container import /home/myapp.tar myapp:1 --digest sha256:73f9a3176741aaaba13b3793755910c64a0362f796511be50125df018803ab4e
vmx container run myapp:1 app-01 --env MODE=demo --port 9001:8080
vmx container image-remove myapp:1
```

## The host

```sh
vmx host security                         # Secure Boot, the TPM, and every pool's key
vmx host load                             # what the host is doing
vmx ups                                   # the UPS, if one is set up
bootmass upgrade --status                 # the release running, and the newest
bootmass be list                          # the boot environments; an upgrade is a new one
vmx tui                                   # the text console
vmx gui                                   # the desktop's window for vmx
```
