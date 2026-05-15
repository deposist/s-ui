# S-UI v1.5.0 - security foundation and realtime platform

## Upgrade notes

Before upgrading production, create a protected backup of the database files:

```sh
sudo systemctl stop s-ui
sudo install -d -m 0700 /root/s-ui-backups
sudo cp -a /usr/local/s-ui/s-ui.db /root/s-ui-backups/s-ui.db.$(date +%Y%m%d%H%M%S)
sudo cp -a /usr/local/s-ui/s-ui.db-wal /root/s-ui-backups/ 2>/dev/null || true
sudo cp -a /usr/local/s-ui/s-ui.db-shm /root/s-ui-backups/ 2>/dev/null || true
sudo systemctl start s-ui
```

Do not publish database backups, subscription URLs, private keys,
certificates, admin credentials, or API tokens in pull requests, issues, CI
logs, or support chats.

## Highlights

- Admins can invalidate all active web sessions from the Admins panel. This
  rotates the web session generation and clears the initiator cookie. API
  tokens are not revoked by this action.
- Grouped API routes were added as the compatibility layer for upcoming
  security, notification, observability, and bulk outbound-check features.
  Existing `/api/<action>` URLs remain supported.
- The installer and `s-ui` management menu now include Chinese as language
  option `3`. Non-interactive installs can use `SUI_LANG=zh`.
- The embedded `sing-box` runtime remains `v1.13.11` from the `v1.4.3`
  runtime update.

## Security defaults

- Telegram notifications are opt-in. This release does not add external
  analytics or telemetry.
- Production deployments that enable encrypted secret storage should set a
  stable `SUI_SECRETBOX_KEY` value and keep it outside the repository and CI
  logs.
- Legacy `Token` header sunset details will be announced with the scoped API
  token migration in this release branch.

## Rollback

The current database schema remains compatible with the previous `v1.4.3`
binary. If rollback is required, deploy the full previous archive or image,
not only the executable, so runtime sidecar libraries such as `libcronet`
remain in sync with the binary.
