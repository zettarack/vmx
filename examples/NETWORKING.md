# Networks, cards and ports

Every guest starts on `default`: a NAT network of the host's own
(virbr0, 192.168.122.0/24), whose guests reach out through the host and
are reached from the host. For a machine on your own LAN, with an address
from your LAN's DHCP, bridge a card; for a lab, make a private network;
to reach a guest on `default` from your LAN, lead a port of the host to it.

```sh
vmx net list                 # every network: mode, bridge, card, address, how many machines
vmx net leases               # who has which address on every network
vmx net leases default
```

## On your LAN: a bridge

```sh
vmx net bridge lan --on enp4s0 --dry-run      # the plan: the card goes under the bridge lan
vmx net bridge lan --on enp4s0
vmx clone debian-13-server web4 --network lan --ssh-key /root/my-key.pub   # an address from your LAN's DHCP
vmx net bridge lan2 --on enp5s0 --address 10.0.0.2/24 --gateway 10.0.0.1 --dns 10.0.0.1 --mtu 9000
vmx net bridge bond0 --on enp5s0,enp6s0 --bond-mode active-backup   # two cards, one at a time; nothing asked of the switch
vmx net bridge bond1 --on enp5s0,enp6s0 --bond-mode 802.3ad         # a LACP port channel the switch is set up for
```

The host's own address moves onto the bridge, with the same MAC and so
the same lease. The card your session came in on, and the last card with
a route out, are refused unless `--take-the-uplink`: a failed handover
would strand the host.

## VLANs

```sh
vmx net vlan lab20 --on lan --tag 20          # lan's card also carries VLAN 20, as the network lab20
vmx clone debian-13-server iot1 --network lab20 --ssh-key /root/my-key.pub
```

## A network of the host's own

```sh
vmx net private lab --subnet 10.50.0.0/24                 # NAT out through the host, nothing in without a forward
vmx net private wire --subnet 10.60.0.0/24 --isolated     # no way out at all: between machines only
vmx clone debian-13-server r1 --network lab --ssh-key /root/my-key.pub
```

The range is given, never guessed: a private range from /16 to /29, as
its network address; one that overlaps another is refused.

## More cards on a machine

```sh
vmx nic add r1 wire          # a second card, for a router or a firewall machine
vmx nic remove r1 52:54:00:12:34:56
```

The guest configures the second card itself; cloud-init brings up the
first card only.

## Ports: the host's port to a guest

```sh
vmx forward add web1 8080:80           # the host's 8080 leads to web1's 80
vmx forward add web1 5353:53 --udp
vmx forward list
vmx forward remove web1 8080
vmx container run nginx:alpine web-01 --port 8081:80      # a container's, at its run
```

A forward is for a guest on a NAT network, which has no address of its
own on your LAN; a machine on a bridge is reached at its own address,
and a forward to it is refused.

## Removing a network

```sh
vmx net remove lab --dry-run
vmx net remove lab
```

Refused while a machine is on it, and for a bridge while a VLAN rides
on it. A bridge's card asks for its own address again.
