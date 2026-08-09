#!/usr/bin/env bash
# stego-triage.sh <file> - run the standard forensics/stego tool chain over a
# file and surface anything flag-shaped. No internet needed.
set -u
F="${1:-}"
if [ -z "$F" ] || [ ! -f "$F" ]; then
  echo "usage: $0 <file>" >&2
  exit 2
fi
FLAG_RE='(flag|ctf|hal)\{[^}]{1,120}\}'
hit() { grep -aoiE "$FLAG_RE" 2>/dev/null | sort -u | sed 's/^/>>> FLAG CANDIDATE: /'; }

echo "=== file type ==="
file "$F"

echo; echo "=== exiftool metadata ==="
command -v exiftool >/dev/null && exiftool "$F" 2>/dev/null

echo; echo "=== strings (flag-shaped only) ==="
strings -n 5 "$F" | grep -aiE "$FLAG_RE" | sort -u || echo "(none)"

echo; echo "=== strings mentioning flag/ctf/key/pass ==="
strings -n 5 "$F" | grep -aiE 'flag|ctf|hal|secret|password|passwd|key' | head -30

echo; echo "=== binwalk (embedded files) ==="
if command -v binwalk >/dev/null; then
  binwalk "$F"
  echo "--- carving with binwalk -e ---"
  OUT="/tmp/binwalk-$(basename "$F")"
  binwalk --run-as=root -e -C "$OUT" "$F" >/dev/null 2>&1 || binwalk -e -C "$OUT" "$F" >/dev/null 2>&1
  if [ -d "$OUT" ]; then
    echo "carved into $OUT ; scanning carved files for flags:"
    grep -raoiE "$FLAG_RE" "$OUT" 2>/dev/null | sort -u | sed 's/^/>>> FLAG CANDIDATE: /'
  fi
fi

echo; echo "=== steghide (JPG/BMP/WAV/AU only) ==="
if command -v steghide >/dev/null; then
  for pw in "" password secret stego hidden flag ctf "$(basename "$F")"; do
    if steghide extract -sf "$F" -p "$pw" -xf /tmp/steg.out -f >/dev/null 2>&1; then
      echo "steghide extracted with passphrase '$pw' -> /tmp/steg.out"
      file /tmp/steg.out
      cat /tmp/steg.out | hit
      strings /tmp/steg.out | grep -aiE "$FLAG_RE" | sort -u
      break
    fi
  done
fi

echo; echo "=== whole-file flag sweep ==="
cat "$F" | hit || echo "(none - try manual xxd / different tool)"
