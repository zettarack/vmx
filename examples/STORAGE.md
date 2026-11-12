# Snapshots, schedules, backups and restores

Every machine's disks are ZFS zvols, so a snapshot is instant and costs
nothing until the disk changes, a clone of a snapshot is a new machine
in an instant, and a backup sends only what changed since the last one.

## Snapshots

```sh
vmx snapshot web2 before-upgrade        # every disk of web2, in one moment, labelled before-upgrade
vmx snapshot web2                       # labelled with the UTC time, e.g. 20261112T053000Z
vmx snapshots web2                      # its snapshots, newest first
vmx stop web2
vmx rollback web2 before-upgrade --dry-run   # what it would do: snapshots newer than it are lost
vmx rollback web2 before-upgrade
vmx snapshot web2 before-upgrade --delete
```

The machine must be stopped for a rollback. A label beginning `auto-`,
`upgrade-`, `clone-` or `backup-` is refused: those are vmx's own.

A snapshot is also a place to start a new machine from, while the first
one keeps running:

```sh
vmx clone web2 web2-test --at before-upgrade   # web2 as it was, as a new machine
```

## Snapshots on a schedule

```sh
vmx schedule web2 --every 1h --keep 24  # every hour, the newest 24 kept: a day of hours
vmx schedule db1 --every 30m --keep 48  # every half hour, a day
vmx schedule                            # every schedule, and when each runs next
vmx schedule web2 off
```

The daemon takes them as `auto-<UTC time>` and destroys the older
automatic ones past `--keep`; a snapshot taken by hand is never pruned.

## Backups

A backup copies the machine to another ZFS dataset: every disk,
snapshotted in one moment, its record, and its firmware state (the UEFI
variables with Secure Boot's keys, and its TPM's state), so it restores
with no other file. A dataset on another pool is the backup that
survives this pool.

```sh
zfs create tank/backups                 # once: where the backups go (on zxp, or a pool of this host)
vmx backup web2 --to tank/backups --dry-run
vmx backup web2 --to tank/backups       # the first sends each disk whole; later ones only the changes
vmx backups --from tank/backups         # what is there: name, backup, when, disks, encrypted or not
```

The machine may be running: the backup then holds what its disks held at
that instant, as after a power cut. Nothing is ever overwritten. An
encrypted machine is sent raw: its disks land encrypted, the key stays
here, and the backup holds what it cannot read.

## Restores

```sh
vmx backups --from tank/backups
vmx restore web2 --from tank/backups                       # its newest backup
vmx restore web2 --from tank/backups --at backup-20261112T053000Z
vmx restore web2 --from tank/backups --passphrase-stdin < /root/web2-pass.txt   # an encrypted one
```

The machine comes back as it was, the same ID, card, address, forwards
and schedule, with its firmware state, and is left stopped. A restore
never writes over anything: `vmx remove web2` comes first when the backup
is the one wanted.

## Templates and goldens

```sh
vmx template save web2                  # a stopped machine sealed and made a template: clones share its blocks
vmx clone web2 web3                     # a clone of the template, in an instant
vmx microvm golden web2 --name web      # a Firecracker golden from its disk
```

## The pools themselves

vmx scrubs its pools on a schedule (the root pool monthly unless set
otherwise), keeps the host its own room so a guest that fills its disk
cannot fill the pool the host lives on, and says each pool's health in
`vmx status`. `vmx host security` shows each pool's encryption and key.
