---
description: Find open ports and services on a host - fast nmap full-port sweep, then deep version/script scan.
---
# recon-scan - find open ports and services on a target host

## When to use
You have a target host/IP but do not yet know what is running on it - any
**recon** challenge, or the first step on any challenge where the category is not
obviously web. Run this to learn which ports are open and what services answer.

## Fastest path (copy-paste)
```sh
/skills/recon-scan/recon-scan.sh "$HAL_TARGET_IP"
```
This does a fast all-ports sweep, then a focused version/script scan on only the
ports that are open, and prints a clean summary.

## Manual commands (if you need to tune)
```sh
# 1) fast: which ports are open at all
nmap -Pn -T4 -p- --min-rate 2000 "$HAL_TARGET_IP"
# 2) deep: versions + default scripts on the open ports (replace 22,80,...)
nmap -Pn -sV -sC -p 22,80,443 "$HAL_TARGET_IP"
# 3) one UDP sweep of the usual suspects (slow - only if TCP is a dead end)
nmap -Pn -sU --top-ports 20 "$HAL_TARGET_IP"
```

## Next steps by service
- **80/443/8080/8000** → web: run `/skills/web-enum` and `curl` the site.
- **21 ftp** → try anonymous: `curl ftp://$HAL_TARGET_IP/ --user anonymous:anonymous`
- **22 ssh** → note the version; may need creds from another step.
- **139/445 smb** → run `/skills/smb-enum`.
- **389/636 ldap** → `ldapsearch -x -H ldap://$HAL_TARGET_IP -s base`
- **3306/5432/6379/27017** → database: connect and query for the flag.
