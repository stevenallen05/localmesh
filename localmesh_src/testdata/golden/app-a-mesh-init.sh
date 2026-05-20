#!/bin/sh
set -eu
# Mesh interception rules for app-a — installed by mesh-init.
iptables -t nat -F OUTPUT 2>/dev/null || true
iptables -t nat -F PREROUTING 2>/dev/null || true
iptables -t nat -A OUTPUT -p tcp --dport 50051 -j REDIRECT --to-ports 15001
iptables -t nat -A OUTPUT -p tcp ! -d 127.0.0.1/32 --syn -j REDIRECT --to-ports 15001
iptables -t nat -A PREROUTING -p tcp --dport 3000 -j REDIRECT --to-ports 15006
exit 0
