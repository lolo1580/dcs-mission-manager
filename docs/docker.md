# Running the DCS Manager web application (Docker)

The Windows desktop remains the only component that talks to DCS. The server is
an optional multi-user web application: it receives finished missions from one
or more desktops, keeps accounts and personal tokens, and serves the shared
flights, statistics, flight plans and checklists.

The desktop keeps working when the server is absent.

## 1. First start on the LAN

```bash
cp .env.example .env
# Edit .env: at least change DCSMANAGER_ADMIN_PASSWORD.
docker compose pull
docker compose up -d
```

Compose pulls the published image
`ghcr.io/lolo1580/dcs-mission-manager:<version>` by default. Use
`docker compose up -d --build` to build it locally from this checkout instead.

If the package is private, authenticate first:

```bash
echo "<github-token>" | docker login ghcr.io -u <github-user> --password-stdin
```

The GitHub release workflow publishes `linux/amd64` and `linux/arm64` images on
every version tag. The version tag and `latest` (stable releases only) are
pushed to GHCR. CI also publishes `main` and `sha-<commit>` images (amd64) on
every push to `main`, for testing the latest code without a local build.

Open `http://<server-ip>:8080`. The first account is created from
`DCSMANAGER_ADMIN_USER` and `DCSMANAGER_ADMIN_PASSWORD` **only when the users
table is empty**. Later starts keep the accounts stored in the database, and the
environment password is ignored.

The SQLite database, sessions and tokens live in the **named volume**
`dcsmanager_data` (mounted at `/data`). It is deliberately separate from the
desktop's `./data` folder, so running compose from the repository cannot touch
the desktop database. That volume is the only thing to back up.

For a NAS, build both architectures with Buildx:

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=2.1.0 -t ghcr.io/lolo1580/dcs-mission-manager:2.1.0 --push .
```

The image runs as UID 10001 and the named volume is created with the right
ownership automatically. Do not point the volume at a host folder unless you
manage its permissions yourself.

## 2. Create a desktop token

1. Sign in as the administrator.
2. Open **My profile → Desktop tokens** and create a token for the PC.
3. Copy the token immediately: it is shown once.

On the desktop:

1. Open **Settings → Server sync**.
2. Enter `http://<server-ip>:8080` and paste the token.
3. Save, then **Test connection** and **Sync now**.

The desktop identifier is generated automatically. Re-uploading the same
mission updates it instead of creating a duplicate.

## 3. Accounts

Administrators create accounts from **Administration**:

- `member`: shared flights, statistics, flight plans and checklists;
- `admin`: also accounts, tokens and the audit log.

There is no self-registration. Email recovery is optional and disabled until
SMTP is configured (see below). An administrator resets a password from the same
page; changing a password signs the account out everywhere and issues a new
session to the current browser.

### Password recovery

Password recovery is optional. Set the SMTP variables in `.env`, plus the public
URL used in the reset link:

```env
DCSMANAGER_SMTP_HOST=smtp.example.com
DCSMANAGER_SMTP_PORT=587
DCSMANAGER_SMTP_USER=dcs@example.com
DCSMANAGER_SMTP_PASSWORD=...
DCSMANAGER_SMTP_FROM=DCS Manager <dcs@example.com>
DCSMANAGER_PUBLIC_URL=https://dcs.example.com
```

Port 587 with STARTTLS is the default. The login page then shows **Forgot
password?**; the link is valid for 30 minutes and can only be used once. Without
SMTP, the button reports that email recovery is not configured; an administrator
can still reset a password from **Administration**.

## 4. HTTPS and Internet exposure
The container speaks plain HTTP. For anything beyond a trusted LAN, put it
behind a reverse proxy that terminates TLS and set:

```env
DCSMANAGER_SECURE_COOKIES=true
```

Minimal Caddy example:

```caddyfile
dcs.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Then run compose with `DCSMANAGER_PORT=127.0.0.1:8080` so the container port is
not reachable directly. Sessions use `HttpOnly` cookies with `SameSite=Lax`, and
every mutation is checked against the request `Origin`.

## 5. Backup and upgrade

An administrator can download a zip snapshot from **Administration → Server
backup**. It contains a consistent `dcsmanager.db` and a short README.

For a volume-level copy:

```bash
# Backup (the container may stay stopped during the copy)
docker compose stop
docker run --rm -v "$PWD":/backup \
  --volumes-from "$(docker compose ps -aq dcsmanager)" alpine \
  tar czf /backup/dcsmanager-data.tgz -C /data .
docker compose start

# Restore a volume copy
docker compose stop
docker run --rm -v "$PWD":/backup \
  --volumes-from "$(docker compose ps -aq dcsmanager)" alpine \
  sh -c 'cd /data && tar xzf /backup/dcsmanager-data.tgz'
docker compose start

# Upgrade to the published image
git pull
docker compose pull
docker compose up -d
```

The database schema is migrated automatically at startup. The health endpoint
checks that SQLite answers, so an unhealthy database marks the container
unhealthy. Test a restore on a copy before relying on a backup.

Upgrading from the old bind-mounted `./data` layout: copy that folder into the
named volume once before starting 2.1.0:

```bash
docker compose down
docker run --rm -v "$PWD/data":/from -v dcsmanager_data:/to alpine \
  sh -c 'cp -a /from/. /to/'
docker compose pull
docker compose up -d
```

`docker compose down -v` also removes the named volume. Do not use it unless you
mean to delete the server database.

## 6. Limits

- SQLite in WAL mode suits a squadron-sized server. PostgreSQL is not supported
  yet.
- One Docker instance serves one squadron. Multi-tenant hosting is out of scope.
- The server never reads `Saved Games` or a DCS installation; only the desktop
  does.
- Flight plans and checklists are owned by the account that created them and can
  be private or shared with the squadron. The desktop and the server exchange
  them in both directions; concurrent edits use **last write wins** and the
  losing copy is kept under **Administration → Sync conflicts**, where it can be
  restored or merged field by field.
- Deleting a document is synchronised as a tombstone, so it does not reappear
  from another desktop.
- A shared document can be edited by its owner and by administrators; other
  members see it read-only (the server refuses the change).
