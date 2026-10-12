# vmx

vmx is the hypervisor from [Zettarack](https://zettarack.com): a workstation
and a ZFS-native KVM host in one machine. Your desktop, your machines, and a
system that rolls back. The tour is at <https://zettarack.com/vmx>.

This repository is where vmx is released. It holds no source code: vmx is
closed source and free of charge.

No release is published here yet.

## Download

Each release is published under Releases here and at `dl.zettarack.com`:
the installer ISO, `SHA256SUMS`, its signature `SHA256SUMS.sig`, and the
release key `release.pem`. The ISO installs with no network.

## Verify

```
openssl pkeyutl -verify -pubin -inkey release.pem -rawin -in SHA256SUMS -sigfile SHA256SUMS.sig
sha256sum -c SHA256SUMS --ignore-missing
```

The first says `Signature Verified Successfully`; the second says `OK`
beside the ISO. The release key's public half, which every vmx image also
carries in `/usr/lib/zettarack/release.pub`, is:

```
8FK45U8HQVYWCmyWcAuRJWsQDGBHLv5Qc26S08oMRow=
```

## Write it to a USB stick

```
sudo dd if=zettarack-vmx-<version>.iso of=/dev/sdX bs=4M status=progress conv=fsync
```

On Windows, use Rufus in DD mode or balenaEtcher.

## You'll need

- UEFI firmware
- Intel VT-x or AMD-V turned on
- A disk of 8 GiB or more for the system, and memory for your machines
- With Secure Boot on: confirm the certificates once in MokManager on the
  first boot; the password is `vmx`

## Updates

An installed vmx takes signed updates from `dl.zettarack.com`: the stable
channel by default, the beta channel for releases before they are stable.
Each upgrade is a new boot environment; a release that does not come up
healthy goes back to the one before by itself.

## Licences

vmx is Copyright (c) 2026 Zettarack. Each image also carries software made
by others, each part under its own licence: the image's `NOTICE` and
`THIRD-PARTY` (in `/usr/share/doc/zettarack`, and on the installer medium
in `zettarack/`) say which, and `NOTICE` says where the source of the
copyleft parts of that build is.

## Security

See [SECURITY.md](SECURITY.md).
