/**
 * SFX asset catalog — single source of truth for all UI sound URLs.
 *
 * ## Adding a new sound
 * 1. Upload the file to UploadThing (or any CDN with stable URLs).
 * 2. Add the URL to the appropriate category below.
 * 3. Use the key in your component: `sfx.play('chat.send')`
 *
 * ## Naming convention
 * Keys follow `{category}.{action}` dot notation:
 *   - chat.*           — messaging sounds (room chat, DMs)
 *   - notification.*   — alerts (friend request, invite, generic)
 *   - game.*           — in-game sounds (move, turn, win, lose, draw)
 *   - ui.*             — generic UI interactions (click, toggle, error)
 *   - queue.*          — matchmaking events
 *
 * ## URL requirements
 * - Must be a direct link to an audio file (mp3, ogg, wav).
 * - Must support CORS (UploadThing, S3, Cloudflare R2 all do).
 * - Prefer short files (<100KB) to keep cache small.
 */

const CATALOG = {
  // -- Chat ------------------------------------------------------------------
  // 'chat.send':     '',
  'chat.receive': 'https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJlzzTPoHAqPzRYN6uLM3mEfoOhTtnbW59BreZ',

  // -- Notifications ---------------------------------------------------------
  // 'notification.dm':             '',
  // 'notification.friend_request': '',
  // 'notification.invite':         '',
  // 'notification.generic':        '',

  // -- Game ------------------------------------------------------------------
  // 'game.move':       '',
  // 'game.my_turn':    '',
  // 'game.win':        '',
  // 'game.lose':       '',
  // 'game.draw':       '',
  // 'game.countdown':  '',

  // -- UI --------------------------------------------------------------------
  // 'ui.click':   '',
  // 'ui.error':   '',
  // 'ui.toggle':  '',

  // -- Queue -----------------------------------------------------------------
  // 'queue.match_found': '',
  // 'queue.ready':       '',
  // -- Multi-variant examples (array format) ----------------------------------
  // 'game.card_play': ['https://cdn.example.com/card-play-1.mp3', 'https://cdn.example.com/card-play-2.mp3'],
  // 'game.move':      ['https://cdn.example.com/move-a.mp3', 'https://cdn.example.com/move-b.mp3', 'https://cdn.example.com/move-c.mp3'],
} as const satisfies Record<string, string | readonly string[]>

export type SfxId = keyof typeof CATALOG

/** The union of possible catalog entry types (single URL or array of URLs). */
export type CatalogEntry = string | readonly string[]

export { CATALOG }
