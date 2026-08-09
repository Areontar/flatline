#!/usr/bin/env bash
# decode.sh <string> - try common CTF encodings and print anything flag-shaped.
set -u
S="${1:-}"
if [ -z "$S" ]; then
  echo "usage: $0 '<encoded-string>'" >&2
  exit 2
fi
FLAG_RE='(flag|ctf|hal)\{[^}]{1,120}\}'
show() { # <label> ; reads decoded bytes on stdin
  local label="$1"; local out; out=$(cat)
  [ -z "$out" ] && return
  if echo "$out" | grep -aoiE "$FLAG_RE" >/dev/null 2>&1; then
    echo ">>> FLAG CANDIDATE via $label:"
    echo "$out" | grep -aoiE "$FLAG_RE" | sort -u
  elif echo "$out" | grep -aqE '[[:print:]]{4}'; then
    echo "[$label] $(echo "$out" | head -c 200)"
  fi
}

printf '%s' "$S" | base64 -d 2>/dev/null | show "base64"
printf '%s' "$S" | base32 -d 2>/dev/null | show "base32"
printf '%s' "$S" | tr -d ' \n' | xxd -r -p 2>/dev/null | show "hex"
printf '%s' "$S" | tr 'A-Za-z' 'N-ZA-Mn-za-m' | show "rot13"
printf '%s' "$S" | rev | show "reversed"
printf '%s' "$S" | python3 -c 'import sys,urllib.parse;print(urllib.parse.unquote(sys.stdin.read()),end="")' 2>/dev/null | show "url-decode"

# rot-N brute (Caesar) over the whole alphabet
for n in $(seq 1 25); do
  printf '%s' "$S" | python3 -c "
import sys
n=$n; s=sys.stdin.read()
print(''.join(chr((ord(c)-b+n)%26+b) if c.isalpha() else c for c in s for b in [65 if c.isupper() else 97]),end='')
" 2>/dev/null | grep -aoiE "$FLAG_RE" && echo "   ^ (rot-$n)"
done

# base64 then decode again (double-encoded is common)
printf '%s' "$S" | base64 -d 2>/dev/null | base64 -d 2>/dev/null | show "base64x2"

echo "[*] If nothing matched, the flag may be XOR/RSA/vigenere - see SKILL.md."
