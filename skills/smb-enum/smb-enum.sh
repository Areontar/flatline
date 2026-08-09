#!/usr/bin/env bash
# smb-enum.sh <host> [user] [pass] - enumerate SMB shares via a null/guest or
# credentialed session and grep readable files for flags.
set -u
HOST="${1:-}"
USER_="${2:-}"
PASS_="${3:-}"
if [ -z "$HOST" ]; then
  echo "usage: $0 <host> [user] [pass]" >&2
  exit 2
fi
FLAG_RE='(flag|ctf|hal)\{[^}]{1,120}\}'

echo "=== smbclient -L (list shares) ==="
if [ -z "$USER_" ]; then
  smbclient -N -L "//$HOST/" 2>/dev/null
else
  smbclient -L "//$HOST/" -U "$USER_%$PASS_" 2>/dev/null
fi

echo; echo "=== smbmap (permissions) ==="
if command -v smbmap >/dev/null; then
  smbmap -H "$HOST" -u "${USER_:-null}" -p "${PASS_:-}" 2>/dev/null
fi

echo; echo "=== enum4linux (users/shares/groups/os) ==="
if command -v enum4linux-ng >/dev/null; then
  enum4linux-ng -A "$HOST" 2>/dev/null
elif command -v enum4linux >/dev/null; then
  enum4linux -a "$HOST" 2>/dev/null
fi

echo; echo "[*] To pull files from a readable share <SHARE>:"
if [ -z "$USER_" ]; then
  echo "    smbclient -N //$HOST/<SHARE> -c 'recurse ON; prompt OFF; mget *'"
else
  echo "    smbclient //$HOST/<SHARE> -U '$USER_%$PASS_' -c 'recurse ON; prompt OFF; mget *'"
fi
echo "    then: grep -raoiE '$FLAG_RE' ."
