/**
 * SFX playback engine with persistent Cache API storage.
 *
 * ## How it works
 * 1. `sfx.play('chat.receive')` is called from any component.
 * 2. The engine checks the browser Cache API (`tf-sfx-v1`) for the audio.
 *    - Cache hit  → decode + play instantly (no network).
 *    - Cache miss → fetch from CDN, store in cache, decode + play.
 * 3. Volume is resolved from the Zustand store settings:
 *    `volume_master * category_volume` (e.g. volume_sfx for game sounds).
 *    If `mute_all` is true or `notify_sound` is false, playback is skipped.
 *
 * ## Cache lifecycle
 * - Stored in Cache API under the name `tf-sfx-v1`.
 * - Persists until the user clears site data.
 * - Bump the version suffix to bust old caches on breaking changes.
 * - `sfx.preload()` warms the cache in the background without blocking.
 *
 * ## AudioContext
 * Created lazily on first user interaction (Chrome autoplay policy).
 * A single AudioContext is shared for all SFX playback.
 */

import { useAppStore } from '@/stores/store'
import { CATALOG, type SfxId } from './catalog'

const CACHE_NAME = 'tf-sfx-v1'

// -- Volume category mapping -------------------------------------------------

type VolumeCategory = 'volume_sfx' | 'volume_ui' | 'volume_notifications'

const CATEGORY_MAP: Record<string, VolumeCategory> = {
  chat: 'volume_ui',
  notification: 'volume_notifications',
  game: 'volume_sfx',
  ui: 'volume_ui',
  queue: 'volume_notifications',
}

function getCategoryVolume(id: SfxId): VolumeCategory {
  const category = (id as string).split('.')[0]
  return CATEGORY_MAP[category] ?? 'volume_sfx'
}

// -- Lazy AudioContext -------------------------------------------------------

let ctx: AudioContext | null = null

function getContext(): AudioContext {
  if (!ctx) ctx = new AudioContext()
  // Resume if suspended (Chrome autoplay policy).
  if (ctx.state === 'suspended') ctx.resume()
  return ctx
}

// -- Decoded buffer cache (in-session) ---------------------------------------
// Avoids re-decoding the same file within a session. The raw bytes live in
// Cache API (persistent); decoded AudioBuffers live here (session-only).

const bufferCache = new Map<string, AudioBuffer>()

// -- Core --------------------------------------------------------------------

async function fetchAndCache(url: string): Promise<Response> {
  const cache = await caches.open(CACHE_NAME)
  const cached = await cache.match(url)
  if (cached) return cached

  const res = await fetch(url, { mode: 'cors' })
  if (res.ok) {
    // Clone before consuming — one copy for cache, one for caller.
    await cache.put(url, res.clone())
  }
  return res
}

async function getBuffer(id: SfxId): Promise<AudioBuffer | null> {
  const url = CATALOG[id]
  if (!url) return null

  const existing = bufferCache.get(url)
  if (existing) return existing

  const res = await fetchAndCache(url)
  if (!res.ok) return null

  const audioCtx = getContext()
  const arrayBuf = await res.arrayBuffer()
  const decoded = await audioCtx.decodeAudioData(arrayBuf)
  bufferCache.set(url, decoded)
  return decoded
}

// -- Public API --------------------------------------------------------------

/**
 * Play a sound by catalog key. Non-blocking, never throws.
 * Respects mute_all, notify_sound, and per-category volume settings.
 */
function play(id: SfxId): void {
  const settings = useAppStore.getState().settings

  if (settings.mute_all || !settings.notify_sound) return

  const masterVol = settings.volume_master
  const categoryVol = settings[getCategoryVolume(id)]
  const finalVol = masterVol * categoryVol
  if (finalVol <= 0) return

  // Fire-and-forget — playback errors are silently ignored.
  getBuffer(id)
    .then(buffer => {
      if (!buffer) return
      const audioCtx = getContext()
      const source = audioCtx.createBufferSource()
      const gain = audioCtx.createGain()
      gain.gain.value = finalVol
      source.buffer = buffer
      source.connect(gain).connect(audioCtx.destination)
      source.start()
    })
    .catch(() => {})
}

/**
 * Preload one or more sounds into Cache API + decode buffer.
 * Call during idle time (e.g. after login) to warm the cache.
 * Non-blocking, never throws.
 */
function preload(...ids: SfxId[]): void {
  for (const id of ids) {
    getBuffer(id).catch(() => {})
  }
}

/**
 * Preload every sound in the catalog. Useful on first login
 * to warm the cache in the background.
 */
function preloadAll(): void {
  const ids = (Object.keys(CATALOG) as SfxId[]).filter(k => CATALOG[k].length > 0)
  preload(...ids)
}

/**
 * Delete the entire SFX cache. Useful for cache-busting on version change
 * or exposing a "clear cache" option in settings.
 */
async function clearCache(): Promise<void> {
  await caches.delete(CACHE_NAME)
  bufferCache.clear()
}

export const sfx = { play, preload, preloadAll, clearCache } as const
