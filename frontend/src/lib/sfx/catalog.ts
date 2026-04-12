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
 *   - game.*           — abstract game state (turn, win, lose, round end, start)
 *   - card.*           — physical card interactions (play, draw, shuffle)
 *   - chip.*           — chip / token interactions (score, accumulate)
 *   - dice.* / die.*   — dice interactions (shake, throw)
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
  'chat.send': '',
  'chat.receive': 'https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJlzzTPoHAqPzRYN6uLM3mEfoOhTtnbW59BreZ',

  // -- Notifications ---------------------------------------------------------
  'notification.dm': '',
  'notification.invite': '',

  // -- Game (abstract state) -------------------------------------------------
  'game.card_play': [''] as readonly string[],
  'game.card_draw': [''] as readonly string[],
  'game.my_turn': '',
  'game.round_end': '',
  'game.win': '',
  'game.lose': '',
  'game.draw': '',
  'game.start': '',
  'game.elimination': '',

  // -- Card (physical card interactions) -------------------------------------
  // card.play      → card-place-1..4
  // card.draw      → card-slide-1..8
  // card.deal      → card-fan-1,2         (fan-out / initial deal)
  // card.discard   → card-shove-1..4      (shoved discard)
  // card.shuffle   → card-shuffle         (one-shot)
  // card.pack_open → cards-pack-open-1,2  (ceremonial round start)
  // card.pack_pull → cards-pack-take-out-1,2
  'card.play': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJgAw2OFQYpchrAMPN8FHU0qGLwWsmy4fRQlod','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJzx6a4SSrZAr9XyTpu6lRYVCaFNUj7skdEPBQ','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ54M7vpf4VO8sDmnuZowCJA1Ii2dWFE7hk0xc','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJwcGQdHYy6OkpXxSlaIoWrRjP1zKtC3wgNQsM'] as readonly string[],
  'card.draw': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJKz3NfQETjhX4ny3sKPOiL0Rwk8F9fGm6S2Nz','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJGi6vEHBaMs1WRwnO20GZkpFAxBogv3IT4PSf','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ1iaiVgLAJ6SxHvw5zMjT4Cku0FPfald1gZKs','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJr6p9m7Q3hZmK7Jo1fBMHVytdO5Dq94NjUiGS','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJuwD8jaK8Nkf156M2nDezK3pEqshiSTmv4JGr','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJMbaBPwKoCDkJVbf9Zu6BKdEv4Re25OX7czal','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJCiLjbmGc1l9JOqIitjfokQGmKFuhdAbEw26a','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJW3JdKLPNLr8MyGZV3ascuj4gJKYz2CSwmAR6'] as readonly string[],
  'card.deal': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJruKNsf3hZmK7Jo1fBMHVytdO5Dq94NjUiGS6','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJcMXGA6gPxfoWns3qmeASKkLHp2Fu4b8EaJdt'] as readonly string[],
  'card.discard': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJwCLUlQYy6OkpXxSlaIoWrRjP1zKtC3wgNQsM','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ1OTKCnLAJ6SxHvw5zMjT4Cku0FPfald1gZKs','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJDQorxlCxuGba6I0PywrK5N3pR8dFA4lYqQeX','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJYIrEcjzAChspVL41dX3oMBOZ76Slf0tkK8Pa'] as readonly string[],
  'card.shuffle': 'https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ9D6Ke4ccCnbi8uDf6Gz0lR7yOreqPQ2IwkKT',
  'card.pack_open': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJvBILUh5OnHC8l9VGJ40RfgFQTy3pDjzqkIKS','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJn3MpzJe1ZgTPSr4d8XDaeMC2zA0vhKkjmoEQ'] as readonly string[],
  'card.pack_pull': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ6JpvM1T8Y7L3BHRCSou0GDh2ndVFxkEcfMaO','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJbqwRfjvpYGDsc8rhzgnwx6Jvu7Ue2B9lIZi5'] as readonly string[],

  // -- Chip (tokens / scoring / accumulation) --------------------------------
  // chip.place   → chip-lay-1..3          (single token placement)
  // chip.collide → chips-collide-1..4     (impact — pot / result)
  // chip.handle  → chips-handle-1..6      (score updates)
  // chip.stack   → chips-stack-1..6       (accumulation — counter rising)
  'chip.place': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ5ZLvXW7f4VO8sDmnuZowCJA1Ii2dWFE7hk0x','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJr6QdkTW3hZmK7Jo1fBMHVytdO5Dq94NjUiGS','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJJR9xJDuU2B3AizkubHGm6CDn4L7WxocewPls'] as readonly string[],
  'chip.collide': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJgYIkzdQYpchrAMPN8FHU0qGLwWsmy4fRQlod','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ4DVbe5UpLNdqz8i17KFTbo5nEZykwxYu92e6','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ5gXU9Uf4VO8sDmnuZowCJA1Ii2dWFE7hk0xc','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJzgXCRbrZAr9XyTpu6lRYVCaFNUj7skdEPBQt'] as readonly string[],
  'chip.handle': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJwr9CfTSYy6OkpXxSlaIoWrRjP1zKtC3wgNQs','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJxgnX3i6CRbsBUTM1ewYVHL5pN4irDSJFPhK0','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJGRyWslaMs1WRwnO20GZkpFAxBogv3IT4PSfa','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJS5OqEOW4kOBFvcXaZYfNejRLECmiHgUpADQ1','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJYkyMn1zAChspVL41dX3oMBOZ76Slf0tkK8Pa','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ2omnMSt5L7gtIqvwTD8WjxRKZH9r0pkzy6um'] as readonly string[],
  'chip.stack': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJr6eVvAp3hZmK7Jo1fBMHVytdO5Dq94NjUiGS','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ3TkDj1sxUN5JnzEPicxDaqLW09GdeH6wh8l7','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJovDBSaBVGsFiUqphkVDAN7Lzwr62evEcYWm0','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJDUmBzoCxuGba6I0PywrK5N3pR8dFA4lYqQeX','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJNjpdfARmrcyA94sZj6QPvlOXCYpBo7JgqdkE','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ9QgCYBaccCnbi8uDf6Gz0lR7yOreqPQ2IwkK'] as readonly string[],

  // -- Dice (randomness / spinners) ------------------------------------------
  // dice.grab  → dice-grab-1,2            (pickup)
  // dice.shake → dice-shake-1..3          ("thinking" / loader)
  // dice.throw → dice-throw-1..3          (multi-die roll)
  // die.throw  → die-throw-1..4           (single die roll)
  'dice.grab': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJpRygyRCIp8Vtj4eDxyXQwWa0ARd9HKCGbFi5','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJhg2ZGTJj3ADCI5xnrFQigRyEUV7wHZpmNOfG'] as readonly string[],
  'dice.shake': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJF96b9T1fXlKhQOwuvrDYjeciAU4z7VJ6SWpk','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJ31zUHbxUN5JnzEPicxDaqLW09GdeH6wh8l7m','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJY4OTWuzAChspVL41dX3oMBOZ76Slf0tkK8Pa'] as readonly string[],
  'dice.throw': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJgINxVYKQYpchrAMPN8FHU0qGLwWsmy4fRQlo','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJRXIS8CknFiPhpteSsM4jk9D86bzEHxBOafCU','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJJEYQUfwuU2B3AizkubHGm6CDn4L7WxocewPl'] as readonly string[],
  'die.throw': ['https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJlAquOGHAqPzRYN6uLM3mEfoOhTtnbW59BreZ','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJRfwt8jknFiPhpteSsM4jk9D86bzEHxBOafCU','https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJpazwLOIp8Vtj4eDxyXQwWa0ARd9HKCGbFi5B', 'https://l4kdz58q3w.ufs.sh/f/emSxdi95EKfJhK8nVOZJj3ADCI5xnrFQigRyEUV7wHZpmNOf'] as readonly string[],

  // -- UI --------------------------------------------------------------------
  'ui.click': '',

  // -- Queue -----------------------------------------------------------------
  'queue.match_found': '',
} as const satisfies Record<string, string | readonly string[]>

export type SfxId = keyof typeof CATALOG

/** The union of possible catalog entry types (single URL or array of URLs). */
export type CatalogEntry = string | readonly string[]

export { CATALOG }
