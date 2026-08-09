#!/usr/bin/env bash
# pwn-triage.sh <binary> - static triage of an executable for pwn/rev.
set -u
B="${1:-}"
if [ -z "$B" ] || [ ! -f "$B" ]; then
  echo "usage: $0 <binary>" >&2
  exit 2
fi
FLAG_RE='(flag|ctf|hal)\{[^}]{1,120}\}'

echo "=== file ==="
file "$B"

echo; echo "=== checksec (protections) ==="
if command -v checksec >/dev/null; then
  checksec --file="$B" 2>/dev/null || checksec "$B" 2>/dev/null
else
  pwn checksec "$B" 2>/dev/null || echo "(checksec unavailable)"
fi

echo; echo "=== flag-shaped strings ==="
strings -n 5 "$B" | grep -aoiE "$FLAG_RE" | sort -u || echo "(none)"

echo; echo "=== interesting strings ==="
strings -n 4 "$B" | grep -aiE 'flag|ctf|hal|/bin/sh|system|gets|strcpy|sprintf|%n|%x|%s|password|secret|win|backdoor' | sort -u | head -40

echo; echo "=== symbols / functions ==="
nm "$B" 2>/dev/null | grep -iE ' t | T ' | head -40 || echo "(stripped or no symbols)"

echo; echo "=== disassembly of main (intel) ==="
objdump -d -M intel "$B" 2>/dev/null | awk '/<main>:/{p=1} p{print} /ret/{if(p)c++} c>0 && /^$/{exit}' | head -80

echo; echo "[*] For dynamic analysis: gdb -q $B   |   For ROP: ROPgadget --binary $B"
