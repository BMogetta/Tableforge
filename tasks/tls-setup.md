# TLS/SSL Setup

## Why
No TLS configured. Cookies travel in plaintext, WebSocket unencrypted, MITM trivial.

## Options
1. **Cloudflare Tunnel** — `CLOUDFLARE_TUNNEL_TOKEN` already in `.env.example`.
   Zero-config TLS, no cert management. Best for Raspberry Pi.
2. **ACME / Let's Encrypt** — Traefik native support. Needs port 80/443 exposed.
3. **Manual certs** — self-managed, most operational burden.

## TBD
- Which option to use (Cloudflare Tunnel recommended for Pi)
- Domain name
- DNS configuration
