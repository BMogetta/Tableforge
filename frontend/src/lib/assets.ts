/**
 * Asset preloading system for Tableforge game sessions.
 *
 * ## Current implementation
 * Assets are loaded via native browser APIs (Image, Audio, document.fonts).
 * Caching is handled at two levels:
 *   1. In-session deduplication: `sessionCache` (this file) — prevents
 *      reloading the same asset twice within a single browser session.
 *   2. HTTP cache: managed by the browser + Caddy headers. Assets served
 *      with `Cache-Control: max-age=31536000, immutable` (fingerprinted files)
 *      survive across sessions and page reloads at no extra cost.
 *
 * ## Future: Service Worker with Workbox
 * When the asset library grows (card art, sounds, animations), consider adding
 * a Service Worker using Workbox (https://developer.chrome.com/docs/workbox).
 *
 * Things to keep in mind when implementing:
 *
 * ### Strategy per asset type
 * - Static game assets (images, audio): CacheFirst — serve from cache,
 *   update in background. Works well with fingerprinted URLs.
 * - Fonts: CacheFirst with long expiry — fonts rarely change.
 * - API responses: NetworkFirst or StaleWhileRevalidate depending on how
 *   stale the data can be.
 *
 * ### Cache versioning
 * Use Workbox's built-in cache versioning or name caches per game version
 * (e.g. "tf-loveletter-v2") so old assets are cleaned up on deploy.
 *
 * ### Registration
 * Register the SW in src/main.tsx after app mount:
 *   import { registerSW } from 'virtual:pwa-register' // via vite-plugin-pwa
 *
 * ### Integration with loadAssets()
 * The loadAssets() signature is intentionally cache-agnostic. When adding
 * a Service Worker, only loadSingleAsset() needs to change — everything
 * that calls loadAssets() (GameLoading.tsx, etc.) stays the same.
 *
 * ### Offline support
 * If offline play is a goal, precache the game shell (index.html, JS bundles)
 * via Workbox's generateSW or injectManifest mode in vite.config.ts.
 *
 * ### Recommended packages
 * - vite-plugin-pwa — integrates Workbox with Vite, handles manifest + SW
 * - workbox-strategies — CacheFirst, NetworkFirst, StaleWhileRevalidate
 * - workbox-expiration — TTL and max-entries per cache
 * - workbox-cacheable-response — only cache 200 responses
 */

export type AssetType = 'image' | 'audio' | 'font'

export interface GameAsset {
  type: AssetType
  url: string
  /** Optional key for deduplication — defaults to url. */
  key?: string
}

export interface LoadProgress {
  loaded: number
  total: number
  /** 0–1 */
  progress: number
}

/**
 * Preloads all assets in the manifest concurrently.
 * Calls onProgress after each asset resolves or rejects.
 * Never throws — failed assets are silently skipped.
 *
 * The signature is intentionally cache-agnostic: swap the implementation
 * of loadSingleAsset() to change the caching strategy without touching callers.
 */
export async function loadAssets(
  assets: GameAsset[],
  onProgress?: (p: LoadProgress) => void,
): Promise<void> {
  if (assets.length === 0) {
    onProgress?.({ loaded: 0, total: 0, progress: 1 })
    return
  }

  let loaded = 0
  const total = assets.length

  const report = () => {
    loaded++
    onProgress?.({ loaded, total, progress: loaded / total })
  }

  await Promise.all(
    assets.map(
      asset => loadSingleAsset(asset).then(report).catch(report), // skip failed assets silently
    ),
  )
}

// ---------------------------------------------------------------------------
// In-session deduplication cache
//
// Prevents reloading assets already fetched during the current browser session.
// Cleared on page reload — this is intentional since the HTTP cache handles
// persistence across sessions when Caddy sends proper Cache-Control headers.
//
// Future: replace with a Cache API / Service Worker lookup when implementing
// the Workbox strategy described in the module docs above.
// ---------------------------------------------------------------------------

const sessionCache = new Set<string>()

async function loadSingleAsset(asset: GameAsset): Promise<void> {
  const key = asset.key ?? asset.url
  if (sessionCache.has(key)) return Promise.resolve()

  switch (asset.type) {
    case 'image':
      return new Promise((resolve, reject) => {
        const img = new Image()
        img.onload = () => {
          sessionCache.add(key)
          resolve()
        }
        img.onerror = reject
        img.src = asset.url
      })

    case 'audio':
      return new Promise((resolve, reject) => {
        const audio = new Audio()
        audio.oncanplaythrough = () => {
          sessionCache.add(key)
          resolve()
        }
        audio.onerror = reject
        audio.src = asset.url
        audio.load()
      })

    case 'font':
      return document.fonts.load(`1em ${asset.url}`).then(() => {
        sessionCache.add(key)
      })

    default:
      return Promise.resolve()
  }
}
