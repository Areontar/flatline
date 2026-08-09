---
description: Triage an ELF binary for pwn/reversing (file, checksec, strings, objdump, gdb, pwntools).
---
# pwn-triage - understand a binary before exploiting or reversing it

## When to use
The challenge gives you an executable (ELF, or a service you can download the
binary for) - any **pwn** or **reversing** challenge. Run this first to learn what
the binary is, its protections, and where the interesting logic and strings are.

## First: get the binary
If it is served over the network, download it, then triage:
```sh
curl -s -o /tmp/bin "http://$HAL_TARGET_IP:$HAL_TARGET_PORT/<path>" ; chmod +x /tmp/bin
/skills/pwn-triage/pwn-triage.sh /tmp/bin
```

## Fastest path (copy-paste)
```sh
/skills/pwn-triage/pwn-triage.sh <binary>
```
Prints `file`, `checksec` (protections), flag-shaped strings, interesting strings
(system/sh/gets/flag/format), symbols, and the `main` disassembly.

## Reversing next steps
```sh
strings -n 5 bin | grep -aiE 'flag|ctf|pass|key'   # flag may just be a string
objdump -d -M intel bin | less                      # full disassembly
gdb -q bin                                           # dynamic analysis
#   (in gdb)  break main ; run ; info functions ; x/20i $pc
```

## Pwn next steps (use pwntools in a python3 script)
```python
from pwn import *
e = ELF("bin")                       # symbols, got, plt
# local:  p = process("./bin")
p = remote(os.environ["HAL_TARGET_IP"], int(os.environ["HAL_TARGET_PORT"]))
p.sendline(b"A"*cyclic_find_offset)  # cyclic() to find the offset
p.interactive()
```
Look at `checksec`: no canary + no PIE → classic ret2win/buffer overflow; NX on →
you need ret2libc or a ROP chain (`ROPgadget --binary bin`).
