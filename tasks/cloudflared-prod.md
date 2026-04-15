# Add Cloudflared for Production

## Why
`tasks/tls-setup.md` recommends Cloudflare Tunnel for TLS on Raspberry Pi, and
`.env.example` already defines `CLOUDFLARE_TUNNEL_TOKEN` — but no service in
the compose stack consumes it. Operators currently have to run `cloudflared`
out of band. This task wires it into the `production` profile so `make up-prod`
is a single command that brings the app online behind a Cloudflare-managed TLS
endpoint, with no port-forward and no public IP exposure.

## What to create

### 1. `cloudflared` service in `docker-compose.services.yml`
Add under the `production` profile. Run in `tunnel run` mode with the token
from the environment. Place it on `app_network` so it can reach `traefik:80`
by container DNS.

```yaml
cloudflared:
  image: cloudflare/cloudflared:latest
  container_name: cloudflared
  restart: unless-stopped
  profiles: ["production"]
  command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN}
  networks:
    - app_network
  depends_on:
    - traefik
  deploy:
    resources:
      limits:
        memory: 128M
        cpus: "0.25"
```

No ports are published — the tunnel is egress-only (outbound 443 to Cloudflare).

### 2. Fail-fast env validation
`CLOUDFLARE_TUNNEL_TOKEN` must be non-empty whenever the `production` profile
is active. Options:
- Add a preflight check to `make up-prod` that errors if the var is empty.
- Or rely on `cloudflared` crash-looping — functional but noisy.

Prefer the Makefile guard:
```makefile
up-prod: networks
	@test -n "$$CLOUDFLARE_TUNNEL_TOKEN" || { echo "CLOUDFLARE_TUNNEL_TOKEN is required for up-prod"; exit 1; }
	docker compose ... --profile production up -d --build
```

### 3. Update `.env.example` comments
The current comment says the tunnel should point to `http://caddy:80`. That's
stale — there is no Caddy in this stack. Change to `http://traefik:80`.

```
# ── Cloudflare Tunnel (prod only) ───────────────────────────────────────────
# Get this token from: dash.cloudflare.com → Zero Trust → Networks → Tunnels
# In the tunnel's Public Hostname config, point the hostname to
# http://traefik:80 (resolvable because cloudflared shares app_network).
CLOUDFLARE_TUNNEL_TOKEN=
```

### 4. Docs
Add a short section to `docs/infrastructure/docker.md` (under "Production
Profile") describing the tunnel wiring, and link from
`docs/getting-started.md` under a new "Production Deploy" heading.

## Cloudflare dashboard configuration

One-time setup before the operator can set `CLOUDFLARE_TUNNEL_TOKEN`.

### DNS
1. Add the domain (e.g. `bmogetta.com`) to Cloudflare as a zone if it isn't
   already. Update the registrar's nameservers to the pair Cloudflare assigns.
2. No manual A/AAAA record is needed for the app subdomain — the tunnel
   creates a proxied CNAME automatically when you map a Public Hostname.

### Zero Trust → Networks → Tunnels
1. **Create a tunnel**
   - Zero Trust dashboard → Networks → Tunnels → **Create a tunnel**
   - Connector type: **Cloudflared**
   - Name: `recess-prod` (or per-environment if running multiple)
   - Save → copy the **token** shown on the "Install connector" screen. This
     is the value of `CLOUDFLARE_TUNNEL_TOKEN`. Store it in the host's `.env`
     and nowhere else; treat it like a password.

2. **Public Hostname**
   - In the tunnel's **Public Hostname** tab, add a route:
     - Subdomain: `recess` (or whatever subdomain is desired)
     - Domain: pick the zone (e.g. `bmogetta.com`)
     - Path: leave empty
     - Service — Type: `HTTP`, URL: `traefik:80`
   - Save. Cloudflare creates the proxied DNS record automatically.

3. **(Optional) Additional hostnames**
   - If `UNLEASH_HOST` or `grafana.<domain>` are exposed publicly, add each
     as its own Public Hostname entry pointing at `traefik:80`. Traefik
     already routes by `Host()` label for those services. Do **not** expose
     internal observability (`loki`, `prometheus`, `tempo`) publicly.

4. **WebSocket support**
   - Nothing to toggle — Cloudflare proxies WebSockets by default on the
     orange-cloud proxy. Verify after deploy by hitting `/ws`.

5. **Access policies (optional hardening)**
   - Zero Trust → Access → Applications → add an application for
     `grafana.<domain>` and `unleash.<domain>` and restrict to the owner
     email. Do **not** gate the main app hostname — the game needs public
     access.

### SSL/TLS mode
- SSL/TLS → Overview → set encryption mode to **Full** (not Flexible, not
  Full (strict)). The tunnel terminates TLS at Cloudflare's edge and speaks
  HTTP to `traefik:80` over the encrypted tunnel, so Full is correct and
  Full (strict) would require a cert on the origin.

### GitHub OAuth callback
Update the OAuth app at github.com/settings/developers:
- Homepage URL: `https://recess.<domain>`
- Callback URL: `https://recess.<domain>/auth/github/callback`

## Operator runbook

Once wired, the full prod bring-up on a fresh Raspberry Pi is:

```bash
git clone <repo> /opt/tableforge && cd /opt/tableforge
make setup
cp .env.example .env
# fill in: ENV=production, DOMAIN, ALLOWED_ORIGINS=https://<domain>,
# GITHUB_*, JWT_SECRET, OWNER_EMAIL, all DB + REDIS passwords,
# DB_HOST=pgbouncer, GRAFANA_ADMIN_PASSWORD, CLOUDFLARE_TUNNEL_TOKEN
make up-prod
```

Verify:
- `docker compose ps cloudflared` → running, healthy
- `docker compose logs cloudflared` → `Registered tunnel connection` for 4
  connections (Cloudflare establishes 4 edge connections by default)
- `curl -sI https://recess.<domain>/api/v1/health` → 200 via Cloudflare
- Browser flow: login with GitHub, start a game, confirm WebSocket upgrades

## Files to create / modify
- `docker-compose.services.yml` — add `cloudflared` service under `production` profile
- `Makefile` — guard `up-prod` on `CLOUDFLARE_TUNNEL_TOKEN`
- `.env.example` — fix stale `caddy:80` comment → `traefik:80`
- `docs/infrastructure/docker.md` — document the tunnel in the production section
- `docs/getting-started.md` — add a "Production Deploy" pointer

## Dependencies
- Cloudflare account with the target zone already added.
- Owner of the zone must create the tunnel and generate the token.
- Pi (or any host) must have outbound 443 to `*.cloudflare.com`.

## Testing
- `docker compose -f docker-compose.yml -f docker-compose.services.yml -f docker-compose.monitoring.yml --profile production --profile app --profile monitoring config --quiet`
- Bring up with a dummy token → confirm `cloudflared` crash-loops with a clear
  auth error (validates the service is wired, not that the token works).
- With a real token, confirm tunnel registers 4 edge connections and the
  public hostname serves the frontend.

## Out of scope
- Rotating the tunnel token (operator concern; replace value in `.env` and
  `docker compose up -d cloudflared`).
- Multi-region HA (running `cloudflared` on two hosts with the same token is
  supported by Cloudflare but not wired here).
- Migrating off Cloudflare Tunnel to ACME/Let's Encrypt — tracked separately
  in `tasks/tls-setup.md`.
