# Boot environments: how a vmx host upgrades, and goes back

A vmx host is an appliance: a signed, read-only system image on ZFS. It
has no package manager. An upgrade does not change the running system;
it makes a new one beside it, a **boot environment**, and the host
restarts into it. Going back is choosing the old one. Nothing an upgrade
does touches a running machine: only the restart stops the machines,
cleanly, and starts them again after it.

## What is in one

Each boot environment is a ZFS dataset under `rpool/ROOT`, cloned from
the release's image, with this machine's identity carried into it: its
name, ids, network, users, host keys and data pools.

```sh
bootmass be list                 # every boot environment: the running one, the one the next boot starts, its image
findmnt -no SOURCE /             # the one running now, e.g. rpool/ROOT/vmx-0.57.81
```

What is not in a boot environment, and so is the same in every one, is
on datasets of its own: the machines and their disks, the images and
templates, `/var/lib/zettarack` (the host's state), `/home` and
`/var/tmp`. What is written into `/etc`, `/root`, `/srv` or `/var/spool`
belongs to the boot environment it was written in.

## How an upgrade runs

```sh
bootmass upgrade --status        # the release running, any staged, the policy, the last check
bootmass upgrade --check         # what an automatic upgrade would do now, changing nothing
bootmass upgrade --auto --download    # fetch and stage the newest release now
```

1. **The release is checked.** The release index is believed only when
   its signature verifies against the release key the running image
   carries (`/usr/lib/zettarack/release.pub`), never when it is older
   than one already seen, and never past its signed validity. The image
   is believed only with the size and digest the index gives. A host up
   to five releases behind fetches only the changes.
2. **The new environment is made beside the running one**, the running
   one snapshotted first and kept.
3. **Its boot menu goes on every root disk** beside the one that boots
   the machine now, to be started once at the next boot. A menu that
   cannot start restarts the machine into the menu that worked before.
4. **The next boot starts it**, once.
5. **The first boot is on trial.** Five minutes after it, the health
   check (`bootmass doctor`) runs: if it passes, the release is kept and
   old environments are pruned; if something fails that was not failing
   before the upgrade, the environment it came from is activated and the
   host restarts into it, and that release is never installed
   automatically again. `bootmass upgrade --status` says when the host
   is on trial and where a restart would go back to.

On a host the TPM unlocks, the upgrade first approves the boot path the
new boot files will measure, so the next boot asks for no passphrase.

## When upgrades happen: the policy

A timer asks every 15 minutes (at a minute fixed for this machine) and
15 minutes after boot. What it does is the policy:

```sh
bootmass upgrade --mode stage            # fetch and stage; it boots at the next restart, which the console asks for (the default)
bootmass upgrade --mode automatic --window 04:00   # stage and restart into it at 04:00, this machine's time
bootmass upgrade --mode automatic --window any     # restart as soon as it is staged
bootmass upgrade --mode notify           # say a release is there; fetch nothing until asked
bootmass upgrade --ring early            # take a release as soon as it is offered
bootmass upgrade --ring normal           # take it at this machine's place in the rollout, up to 36 hours (the default)
bootmass upgrade --channel stable        # the LTS line: kernel 6.12, OpenZFS 2.3, fixes only
bootmass upgrade --channel beta          # the new line: kernel 7.2, OpenZFS 2.4
```

## Going back

```sh
bootmass be list
bootmass be activate vmx-0.57.80 --dry-run   # what it would do
bootmass be activate vmx-0.57.80             # the next boot starts it; nothing changes until then
```

Activating the running one again cancels. The boot menu shows every
environment too, and starting an older one there goes back for that
boot. An environment that is running, set to boot next, or something
was cloned from cannot be removed.

```sh
bootmass be prune --dry-run      # what would go: the newest 5 stay, and the running one and the next always
bootmass be prune --keep 3
bootmass be destroy vmx-0.57.70 --dry-run
```

## An upgrade by hand, from a file

For a host that reaches no network: the image and its description,
carried in.

```sh
bootmass upgrade --image /home/vmx-0.57.81.zfs --dry-run   # the steps, changing nothing
bootmass upgrade --image /home/vmx-0.57.81.zfs             # the .json description sits beside it
```

Its first boot is on trial like any upgrade's; the restart is yours.
