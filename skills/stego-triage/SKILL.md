---
description: Pull hidden data and flags out of a file or image (file, strings, exiftool, binwalk, steghide).
---
# stego-triage - pull hidden data and flags out of a file

## When to use
The challenge gives you a **file** - an image (jpg/png/bmp/gif), audio, PDF,
document, disk image, or any unknown blob - and the flag is hidden inside it.
Use this for any **forensics** or **stego** challenge, on every file you get.

## First: get the file
If the flag file sits on a web/FTP target, download it first, then triage:
```sh
curl -s -o /tmp/artifact "http://$HAL_TARGET_IP:$HAL_TARGET_PORT/<path>"
/skills/stego-triage/stego-triage.sh /tmp/artifact
```

## Fastest path (copy-paste)
```sh
/skills/stego-triage/stego-triage.sh <file>
```
The helper runs `file`, `strings`, `exiftool`, `binwalk`, and (for images)
`steghide`, then highlights anything flag-shaped. **Read its output - any line it
marks `>>> FLAG CANDIDATE` is likely the answer.**

## Manual commands (if the helper finds nothing)
```sh
file suspect.jpg                       # what is it really?
strings -n 6 suspect.jpg | grep -aiE 'flag|ctf|hal'
exiftool suspect.jpg                   # metadata, comments, GPS, author
binwalk -e suspect.jpg                 # carve out embedded/appended files
steghide extract -sf suspect.jpg -p "" # try empty passphrase (then common ones)
zsteg suspect.png                      # (not installed) LSB - use: xxd | head instead
xxd suspect.jpg | head -40             # look at the header/footer by hand
```

## Passwords for steghide
Try empty first, then common words and the challenge name. steghide only works on
JPG/BMP/WAV/AU. The helper already tries a small password list for you.
