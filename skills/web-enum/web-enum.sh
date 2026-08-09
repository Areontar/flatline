#!/usr/bin/env bash
# web-enum.sh <base-url> - brute-force common paths with ffuf, print only hits.
# Falls back to gobuster if ffuf is missing. Bundled wordlist, no internet needed.
set -u
URL="${1:-}"
if [ -z "$URL" ]; then
  echo "usage: $0 http://host:port[/subpath]" >&2
  exit 2
fi
WL="/skills/wordlists/web-common.txt"
echo "[*] enumerating $URL"

if command -v ffuf >/dev/null 2>&1; then
  ffuf -u "${URL%/}/FUZZ" -w "$WL" \
       -mc 200,204,301,302,307,401,403 \
       -t 40 -timeout 8 -s
elif command -v gobuster >/dev/null 2>&1; then
  gobuster dir -u "$URL" -w "$WL" -q -k -t 40 \
       -s 200,204,301,302,307,401,403 -b ""
else
  echo "[!] neither ffuf nor gobuster present; looping with curl" >&2
  while IFS= read -r p; do
    [ -z "$p" ] && continue
    code=$(curl -s -o /dev/null -w '%{http_code}' -m 8 "${URL%/}/$p")
    case "$code" in
      200|204|301|302|307|401|403) echo "$code  ${URL%/}/$p" ;;
    esac
  done < "$WL"
fi

echo "[*] done. Fetch any 200/30x hit with: curl -sL ${URL%/}/<hit>"
