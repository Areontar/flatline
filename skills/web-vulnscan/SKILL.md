---
description: Fingerprint a web app and scan it for known vulnerabilities and misconfigs (whatweb, nikto), including WordPress.
---
# web-vulnscan - fingerprint and vuln-scan a web target

## When to use
On any **web** challenge, right after `web-enum`, to learn the tech stack (server,
framework, CMS, versions) and find known issues, default files, and dangerous
misconfigurations. Especially when the site looks like a real app or a CMS.

## Fingerprint first (fast, cheap)
```sh
whatweb -a 3 "http://$HAL_TARGET_IP:$HAL_TARGET_PORT"
```
Look at the output for: `WordPress`, `Drupal`, `Joomla`, server banners, PHP/CMS
versions, admin panels, and interesting headers.

## Vulnerability scan
```sh
nikto -host "http://$HAL_TARGET_IP:$HAL_TARGET_PORT" -maxtime 120s
```
Nikto flags default files, backup files, dangerous methods, outdated components,
and exposed admin pages - follow up on anything it reports with `curl`.

## If it is WordPress
`wpscan` is **not bundled** (it needs a Ruby stack and an internet-updated vuln
feed we do not have). Enumerate WordPress by hand instead:
```sh
curl -s "http://$HAL_TARGET_IP:$HAL_TARGET_PORT/wp-json/wp/v2/users"   # user list -> usernames
curl -s "http://$HAL_TARGET_IP:$HAL_TARGET_PORT/wp-login.php" -I        # confirm login page
curl -s "http://$HAL_TARGET_IP:$HAL_TARGET_PORT/?author=1" -I           # user enum via redirect
# then brute the login with the password skill:
hydra -L users.txt -P /skills/wordlists/passwords-top.txt "$HAL_TARGET_IP" \
  http-post-form "/wp-login.php:log=^USER^&pwd=^PASS^:F=incorrect"
```

## Next
Turn findings into action: injectable param → `web-sqli`; login form →
`password-crack`; readable config/backup file → fetch it with `curl` and grep for
credentials and the flag.
