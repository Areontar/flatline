---
description: Decode encodings (base64/hex/rot/XOR) and crack simple crypto/RSA with python (pycryptodome, sympy, gmpy2).
---
# crypto-triage - decode encodings and crack simple crypto

## When to use
The challenge is **crypto** or **misc**: you have a blob of text that looks
encoded/encrypted (base64, hex, a shifted alphabet, XOR, an RSA `n`/`e`/`c`, a
JWT, morse), and the flag is inside once decoded.

## Fastest path for encodings (copy-paste)
Pass the suspicious string - the helper tries base64/base32/hex/rot13/rot-N/
reverse/URL/gzip and prints anything flag-shaped:
```sh
/skills/crypto-triage/decode.sh 'ZmxhZ3tleGFtcGxlfQ=='
# or from a file:
/skills/crypto-triage/decode.sh "$(cat /tmp/cipher.txt)"
```

## By hand
```sh
echo 'STRING' | base64 -d
echo 'STRING' | xxd -r -p              # hex -> bytes
echo 'STRING' | tr 'A-Za-z' 'N-ZA-Mn-za-m'   # rot13
```

## RSA / number-theory (use python3 + pycryptodome + sympy + gmpy2)
```python
from Crypto.Util.number import long_to_bytes, inverse
import sympy
n = ...; e = ...; c = ...
p, q = list(sympy.factorint(n))      # works when n is small / factorable
phi = (p-1)*(q-1); d = inverse(e, phi)
print(long_to_bytes(pow(c, d, n)))
```
For small `e` (e.g. 3) with no padding, try the integer cube root of `c`
(`sympy.integer_nthroot(c, e)`).

## JWT
```sh
echo 'HEADER.PAYLOAD' | cut -d. -f2 | base64 -d 2>/dev/null   # read claims
```
Try `alg:none` and brute the HMAC secret against the bundled password list if the
token must be forged.

## Hashes
Identify then crack - see the `password` skill:
```sh
hashid 'THEHASH'
```
