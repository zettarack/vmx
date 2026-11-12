# What protects a vmx host

A vmx host is signed from its first instruction: the firmware's Secure
Boot starts shim, shim starts a boot menu signed with Zettarack's key,
and the menu starts a signed system image that is read-only. Nothing
unsigned boots, and nothing is installed on the host afterwards: an
upgrade is a new signed image beside the running one
([BOOT-ENVIRONMENTS.md](BOOT-ENVIRONMENTS.md)).

## Access

There is no default password. The installer's Access page takes an ssh
key, which opens root over ssh, and a user who logs in at the screen with
a password of their choosing. A secret (a passphrase, a password) is read
from stdin, never from an argument, where `ps` would show it.

## What protects this machine, at a glance

```sh
vmx host security           # Secure Boot, the boot menu's signer, shim's keys, the TPM, each pool's key
vmx host security --json    # the daemon's answer whole
```

It says:

- **Secure Boot**: on or off, what shim does with this machine's boot
  menu (or would, once Secure Boot is switched on), the certificate that
  signed the menu and the key list it was found in, and every key in
  shim's list by name.
- **The TPM**: whether there is one, and what unlocks the root pool when
  the machine starts: the TPM, with the boot path the passphrase is
  sealed for, or the passphrase typed, and why.
- **Every imported pool**: its encryption (off, or the cipher), whether
  its key is loaded, and where the key comes from at start:

| From | |
|---|---|
| the TPM | the root pool's, unsealed with nobody typing |
| typed at boot | the root pool's passphrase, asked for at each boot |
| typed by hand | a data pool's passphrase: the pool stays locked after a restart until it is unlocked |
| a file | a file on this machine the boot reads: what the installer keeps for a data pool it made encrypted |
| elsewhere | somewhere this appliance does not read at boot |

A part the daemon could not read is said as not read, with why: nothing
is printed as a no that was not read as one.

The same is the **Keys** tab of the text console (`vmx tui keys`) and
the **Boot and keys** page of the desktop's Settings (System, Boot and
keys).

## Unlocking an encrypted data pool

```sh
vmx host security                                                  # vault: NOT LOADED, typed by hand
vmx host security unlock vault --passphrase-stdin --dry-run < /root/vault-pass.txt
vmx host security unlock vault --passphrase-stdin < /root/vault-pass.txt
vmx host security unlock vault --passphrase-stdin --at-boot < /root/vault-pass.txt   # and from now on, at every boot
```

The key is loaded from the passphrase and every filesystem that can mount
is mounted, as the boot mounts it; what does not mount is said, each with
why (the boot mounts nothing of a data pool under the system's own
directories, `/var` and `/home` among them). A wrong passphrase is
refused by ZFS, and nothing changes.

`--at-boot` also keeps the passphrase where the installer keeps a data
pool's, a file on the state dataset that root alone reads, and tells the
pool so: it unlocks with the machine from then on. The key is kept only
where ZFS then opens it as the boot will; a keep that would not hold at
the next boot fails, and is undone.

Refused: the pool the system runs from (its passphrase is typed at each
boot or unsealed by the TPM, and never kept in a file); with
`--at-boot`, a pool that keeps its key somewhere of its owner's own; a
pool that is not imported, or not encrypted.

From the console, `u` on a locked pool's row of the Keys tab unlocks it
and `b` unlocks it and keeps its key for the boot; the passphrase is
typed into a box that is wiped.

## The machines

Every machine boots UEFI with Secure Boot and a TPM of its own (swtpm)
unless its template says otherwise; a backup carries the machine's UEFI
variables and TPM state with its disks, so a restored machine comes back
to its BitLocker or its TPM-sealed disk. Every microVM and container runs
under Firecracker's jailer, as a user of its own, in a chroot of its own.

## Reporting a problem

See [SECURITY.md](../SECURITY.md) at the top of this repository.
