# GoDaddy DDNS

Keeps one GoDaddy DNS `A` record pointing at the public IPv4 address of the
machine it runs on. It checks the public IP on a schedule and updates the
record only when the IP changes.

## How to use

1. Create a personal access token on the
   [GoDaddy Developer Portal](https://developer.godaddy.com/docs/api-users/auth/how-to)
   with the `domains.dns:update` scope.
2. Run the container:

   ```shell
   docker run -d --name godaddy-ddns \
     -e GODADDY_PAT=<personal-access-token> \
     -e DOMAIN=example.com \
     -e NAME=home \
     ghcr.io/chenwei791129/godaddy-ddns:1
   ```

   This keeps `home.example.com` pointing at your public IP. Use `NAME=@` (the
   default) for the domain itself.

Images are published to GitHub Container Registry for `linux/amd64` and
`linux/arm64` on every release. Tags: `X.Y.Z`, `X.Y`, `X` and `latest`. The
`alpine` tag is a deprecated alias of `latest` (see
[Upgrading](#upgrading-from-the-shell-script-image)). See
[CHANGELOG.md](CHANGELOG.md) for release notes.

### Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `GODADDY_PAT` | yes¹ | | GoDaddy personal access token with the `domains.dns:update` scope |
| `DOMAIN` | yes | | Domain whose record is managed, e.g. `example.com` |
| `NAME` | | `@` | Record name; `@` is the domain itself |
| `TYPE` | | `A` | Record type; only `A` (IPv4) is supported |
| `TTL` | | `600` | Record TTL in seconds; values below 600 are raised to 600 |
| `CHECK_URL` | | `https://whatismyip.akamai.com/` | URL that returns the public IPv4 address as plain text; it is always queried over IPv4 |
| `CHECK_INTERVAL` | | `5m` | Time between checks, e.g. `30s`, `5m`, `1h`; the first check runs at startup |
| `SUCCESS_EXEC` | | | Shell command run after the record was updated to a new IP |
| `FAILED_EXEC` | | | Shell command run when the public IP cannot be determined or the update fails |
| `GODADDY_KEY`, `GODADDY_SECRET` | | | **Deprecated** classic developer key, used only when `GODADDY_PAT` is not set |
| `CRON_TIME` | | | **Deprecated** cron expression, used only when `CHECK_INTERVAL` is not set |

¹ Or the deprecated `GODADDY_KEY` and `GODADDY_SECRET` together. When
`GODADDY_PAT` is set, those two are ignored.

`TZ` (for example `TZ=Asia/Taipei`) sets the time zone of `CRON_TIME`
schedules; the default is UTC.

### Hooks

`SUCCESS_EXEC` and `FAILED_EXEC` run with `/bin/sh -c`. A success hook does not
run when the record already had the right IP. Hooks see the container's
environment plus:

| Variable | Description |
|---|---|
| `PUBLIC_IP` | The detected public IP (empty if it could not be determined) |
| `PREVIOUS_IP` | The IP the record had before, when it could be read |
| `FAILURE_REASON` | Why the check failed (failure hook only) |

`curl` and `wget` are available in the image. A hook that runs longer than 5
minutes is stopped together with any commands it started. A failing hook is
logged and does not affect the next checks. For example:

```shell
-e SUCCESS_EXEC='curl -fsS "https://example.com/notify?ip=$PUBLIC_IP"'
```

## Upgrading from the shell script image

Version 1.0.0 replaces the original shell script with a Go program. Existing
configurations keep working. The deprecated settings below will be removed in
2.0.0; all of them except the image tag log a warning at startup.

| Deprecated | Replace with |
|---|---|
| `GODADDY_KEY` and `GODADDY_SECRET` | `GODADDY_PAT`: GoDaddy is retiring classic developer keys for the Domains API |
| `CRON_TIME="*/5 * * * *"` | `CHECK_INTERVAL=5m` |
| `$PublicIP` in hooks | `$PUBLIC_IP` |
| the `alpine` image tag | `latest`, or a version tag such as `1` |

Other changes:

- The default `CHECK_URL` is the same service over HTTPS, and TLS certificates
  are now verified for every request: a `CHECK_URL` with a self-signed
  certificate no longer works. A plain `http://` `CHECK_URL` still works but
  logs a warning.
- The first check runs immediately at startup instead of at the next cron time.
- The container runs the program directly: arguments passed to the container
  (for example a compose `command:`) are ignored, and `/scripts/godaddy-ddns.sh`
  no longer exists. Restart the container to force a check.
- `TZ` must be a time zone name such as `Asia/Taipei`; POSIX strings such as
  `CST-8` are no longer understood and fall back to UTC.
- Images are published per release. The `YYYYMMDD-<sha>` tags are no longer
  updated; use `latest` or a version tag.
- The Docker Hub image `awei/godaddy-ddns` is no longer updated; use
  `ghcr.io/chenwei791129/godaddy-ddns`.

## Development

```shell
go test -race ./...
docker build -t godaddy-ddns .
```

| Path | Contents |
|---|---|
| `cmd/godaddy-ddns` | Entry point: wires the packages together and handles signals |
| `internal/config` | Reads and validates the environment, including Deprecated Settings |
| `internal/ddns` | The update check: Public IP lookup, record update, hooks and scheduling |
| `internal/godaddy` | GoDaddy Domains API client |

Design decisions are recorded in [docs/adr](docs/adr), and the project
vocabulary in [CONTEXT.md](CONTEXT.md).

## Credits

Originally based on
[GoDaddy.sh by Nazar78](http://teanazar.com/2016/05/godaddy-ddns-updater/).

## License

The repository is open-sourced software licensed under the
[MIT license](https://opensource.org/licenses/MIT).
