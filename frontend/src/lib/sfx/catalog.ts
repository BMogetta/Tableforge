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
  'notification.dm': '',
  'notification.invite': '',

  // -- Game ------------------------------------------------------------------
  'game.card_play': ['', ''] as readonly string[],
  'game.card_draw': ['', ''] as readonly string[],
  'game.my_turn': '',
  'game.round_end': '',
  'game.win': '',
  'game.lose': '',
  'game.draw': '',
  'game.start': '',
  'game.elimination': '',

  // -- UI --------------------------------------------------------------------
  'ui.click': '',

  // -- Queue -----------------------------------------------------------------
  'queue.match_found': '',
} as const satisfies Record<string, string | readonly string[]>

export type SfxId = keyof typeof CATALOG

/** The union of possible catalog entry types (single URL or array of URLs). */
export type CatalogEntry = string | readonly string[]

export { CATALOG }
