---
description: Brute-force hidden directories, files, and backups on a web target (ffuf/gobuster + bundled wordlist).
---
# web-enum - find hidden directories and files on a web target

## When to use
The challenge is a **web** app (an HTTP/HTTPS target) and you need to discover
hidden pages, directories, backup files, admin panels, or a `flag` file. Use this
early on any web challenge, right after a first `curl`.

## Fastest path (copy-paste)
The target URL is built from the injected env vars. Run the helper - it picks
`ffuf`, brute-forces with the bundled wordlist, and prints only interesting hits:

```sh
/skills/web-enum/web-enum.sh "http://$HAL_TARGET_IP:$HAL_TARGET_PORT"
```

Give it a full base URL if you already know a subpath, e.g.
`/skills/web-enum/web-enum.sh "http://$HAL_TARGET_IP:$HAL_TARGET_PORT/app"`.

## What good hits look like
- `200` - page exists, fetch it: `curl -s URL/<hit>`
- `301/302` - redirect, follow it: `curl -sL URL/<hit>`
- `403` - it exists but is forbidden; there is often a real file inside that dir,
  so brute the dir again: `/skills/web-enum/web-enum.sh "URL/<hit>"`

## Manual commands (if you need to tune)
```sh
# ffuf, bundled common wordlist, hide 404s:
ffuf -u "http://$HAL_TARGET_IP:$HAL_TARGET_PORT/FUZZ" -w /skills/wordlists/web-common.txt -mc 200,204,301,302,307,401,403 -s
# gobuster equivalent:
gobuster dir -u "http://$HAL_TARGET_IP:$HAL_TARGET_PORT" -w /skills/wordlists/web-common.txt -q -k
# fingerprint the stack first:
whatweb "http://$HAL_TARGET_IP:$HAL_TARGET_PORT"
```

## Always check these by hand too
`curl -s URL/robots.txt`, `curl -s URL/.git/HEAD`, `curl -s URL/ | grep -i flag`,
and read HTML source comments - flags hide in comments and robots.txt often.
