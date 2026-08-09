#!/usr/bin/env bash
# try.sh <chat-url> [json-field] - POST each prompt-injection payload to a chat
# endpoint (as JSON and as a form) and print any flag-shaped reply.
set -u
URL="${1:-}"
FIELD="${2:-message}"
if [ -z "$URL" ]; then
  echo "usage: $0 http://host:port/chat [json-field-name]" >&2
  exit 2
fi
PAYLOADS="/skills/prompt-inject/payloads.txt"
FLAG_RE='(flag|ctf|hal)\{[^}]{1,120}\}'

send() { # <content-type> <body>  -> prints response
  curl -s -m 15 -X POST "$URL" -H "Content-Type: $1" --data "$2" 2>/dev/null
}

while IFS= read -r p; do
  [ -z "$p" ] && continue
  # 1) JSON {"<field>": "<payload>"}
  body=$(jq -n --arg f "$FIELD" --arg v "$p" '{($f): $v}')
  r=$(send "application/json" "$body")
  # 2) OpenAI-style messages array
  body2=$(jq -n --arg v "$p" '{messages:[{role:"user",content:$v}]}')
  r2=$(send "application/json" "$body2")
  # 3) form field
  r3=$(curl -s -m 15 -X POST "$URL" --data-urlencode "$FIELD=$p" 2>/dev/null)

  for resp in "$r" "$r2" "$r3"; do
    hit=$(printf '%s' "$resp" | grep -aoiE "$FLAG_RE" | sort -u)
    if [ -n "$hit" ]; then
      echo ">>> FLAG CANDIDATE (payload: $p):"
      echo "$hit"
    fi
  done
done < "$PAYLOADS"

echo "[*] done. If nothing matched, inspect a raw reply and adjust the field/URL:"
echo "    curl -s -X POST '$URL' -H 'Content-Type: application/json' -d '{\"$FIELD\":\"hi\"}'"
