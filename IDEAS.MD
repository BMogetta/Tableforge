# Recess — Session Notes

---

## 4. TODO — Grafana subdomain migration

- [ ] Move Grafana from `localhost/grafana` (path) to `grafana.localhost` (subdomain)
- [ ] Remove `GF_SERVER_ROOT_URL` env var — no longer needed with subdomain
- [ ] Update Traefik label: `Host(\`grafana.localhost\`)` instead of `PathPrefix(\`/grafana\`)`
- [ ] Move all other services to subdomains: `recess.localhost`, `traefik.localhost`, etc.
- [ ] Verify `*.localhost` resolves on all team machines (Windows may need `/etc/hosts` entry)

Subdomain routing advantage: each service thinks it's at root `/`, no base path config needed in any app.

---

## 6. Turn notifications when tab is hidden

Combination of **Web Notifications API** + pre-unlocked `AudioContext`.

### How it works

- Browser blocks audio started without user interaction
- Solution: unlock `AudioContext` on first user action during a game
- If tab is hidden when it's the player's turn: show system notification + attempt audio
- System notification always works. Audio works if context was pre-unlocked

### APIs used

- **Page Visibility API** — `document.hidden` / `visibilitychange` event
- **Web Notifications API** — `Notification.requestPermission()` + `new Notification()`
- **AudioContext** — pre-unlock on first game interaction
- **WakeLock API** — prevent screen sleep during active turn

### src/lib/turn-notifier.ts

```typescript
export class TurnNotifier {
  private audioContext: AudioContext | null = null

  // Call on every player interaction during a game (move, click, etc.)
  unlock() {
    if (!this.audioContext) this.audioContext = new AudioContext()
    if (this.audioContext.state === 'suspended') this.audioContext.resume()
  }

  async notifyTurn() {
    // System notification — always works if permission granted
    if (Notification.permission === 'granted' && document.hidden) {
      new Notification("It's your turn!", {
        body: 'Make your move in Recess',
        icon: '/icon-192.png',
        tag:  'turn-notification', // replaces previous if still showing
      })
    }

    // Audio — only works if context was pre-unlocked
    if (this.audioContext?.state === 'running') {
      playSound('your-turn')
    }
  }
}
```

### Request notification permission

Ask on game start, not on app load:

```typescript
if (Notification.permission === 'default') {
  await Notification.requestPermission()
}
```

### WakeLock

```typescript
// On game start
const wakeLock = await navigator.wakeLock.request('screen')

// On game end
await wakeLock.release()
```

### TODOs

- [ ] Create `src/lib/turn-notifier.ts`
- [ ] Call `unlock()` on every player interaction during a game
- [ ] Request `Notification` permission when a game session starts
- [ ] Integrate with WebSocket handler that receives opponent move events
- [ ] Add WakeLock on game start, release on game end

---

## 7. Missing static files — public/ (MOSTLY DONE)

Done: favicon.svg, manifest.json, robots.txt, security.txt, index.html meta tags, self-hosted fonts.

### Still pending

- [ ] Export `icon-192.png` and `icon-512.png` from the favicon SVG
- [ ] Create `og-image.png` 1200×630px
- [ ] Optional: `favicon.ico` fallback for older browsers
- [ ] Verify `.well-known/` is included in Vite build: `ls dist/.well-known/`