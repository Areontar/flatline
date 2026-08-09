#!/usr/bin/env bash
# recon-scan.sh <host> - fast full-port sweep, then deep scan of open ports only.
set -u
HOST="${1:-}"
if [ -z "$HOST" ]; then
  echo "usage: $0 <host-or-ip>" >&2
  exit 2
fi

echo "[*] phase 1: fast full TCP port sweep of $HOST"
FAST=$(nmap -Pn -T4 -p- --min-rate 2000 "$HOST" 2>/dev/null)
echo "$FAST"

PORTS=$(echo "$FAST" | awk -F/ '/^[0-9]+\/tcp[[:space:]]+open/{printf "%s%s",sep,$1; sep=","}')
if [ -z "$PORTS" ]; then
  echo "[!] no open TCP ports found. Consider a UDP scan:"
  echo "    nmap -Pn -sU --top-ports 20 $HOST"
  exit 0
fi

echo
echo "[*] phase 2: version + default-script scan of open ports: $PORTS"
nmap -Pn -sV -sC -p "$PORTS" "$HOST"
