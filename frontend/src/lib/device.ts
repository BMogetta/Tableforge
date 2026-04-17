// ---------------------------------------------------------------------------
// Device context — zero-dependency capture of browser/device/environment info.
//
// Two shapes:
//   getDeviceContext()      → nested object  (for JSONB storage)
//   getDeviceContextAttrs() → flat Record    (for OTel log attributes)
//
// Data is cached at module load. The async userAgentData API resolves into
// the cache; until then the synchronous UA string fallback is used.
// ---------------------------------------------------------------------------

// --- Experimental browser API types ------------------------------------------

interface UABrand {
  brand: string
  version: string
}

interface UAHighEntropyValues {
  fullVersionList?: UABrand[]
  platform?: string
  platformVersion?: string
}

interface NavigatorUA {
  mobile: boolean
  platform: string
  getHighEntropyValues?: (hints: string[]) => Promise<UAHighEntropyValues>
}

interface NetworkInformation {
  effectiveType?: string
}

interface NavigatorExt extends Navigator {
  userAgentData?: NavigatorUA
  connection?: NetworkInformation
  deviceMemory?: number
}

// --- Types -------------------------------------------------------------------

export interface DeviceContext {
  browser: { name: string; version: string }
  os: { name: string; version: string }
  platform: string
  mobile: boolean
  screen: { width: number; height: number; dpr: number; orientation: string }
  viewport: { width: number; height: number }
  hardware: { cores: number | null; memory: number | null }
  network: { online: boolean; type: string | null }
  locale: { languages: string[]; timezone: string }
  preferences: { colorScheme: string }
}

// --- UA parsing (sync fallback) ---------------------------------------------

const UA = typeof navigator !== 'undefined' ? navigator.userAgent : ''

function parseBrowser(): { name: string; version: string } {
  // Order matters — Chrome/Edge/Opera contain "Safari" and "Chrome" in their UA.
  const patterns: [string, RegExp][] = [
    ['Edge', /Edg(?:e|A|iOS)?\/(\S+)/],
    ['Opera', /(?:OPR|Opera)\/(\S+)/],
    ['Firefox', /Firefox\/(\S+)/],
    ['Chrome', /Chrome\/(\S+)/],
    ['Safari', /Version\/(\S+).*Safari/],
  ]
  for (const [name, re] of patterns) {
    const m = UA.match(re)
    if (m) return { name, version: m[1] }
  }
  return { name: 'Unknown', version: '' }
}

function parseOS(): { name: string; version: string } {
  const patterns: [string, RegExp][] = [
    ['iOS', /(?:iPhone|iPad|iPod).*OS (\d+[._]\d+)/],
    ['Android', /Android (\d+(?:\.\d+)?)/],
    ['Windows', /Windows NT (\d+\.\d+)/],
    ['macOS', /Mac OS X (\d+[._]\d+[._]?\d*)/],
    ['Linux', /Linux/],
    ['ChromeOS', /CrOS/],
  ]
  for (const [name, re] of patterns) {
    const m = UA.match(re)
    if (m) return { name, version: (m[1] ?? '').replace(/_/g, '.') }
  }
  return { name: 'Unknown', version: '' }
}

// --- Cache -------------------------------------------------------------------

let cachedBrowser = parseBrowser()
let cachedOS = parseOS()
let cachedPlatform = ''
let cachedMobile = /Mobi|Android/i.test(UA)

// Attempt high-entropy UA data (Chrome/Edge 90+). Resolves into cache.
if (typeof navigator !== 'undefined' && 'userAgentData' in navigator) {
  const uad = (navigator as NavigatorExt).userAgentData!
  cachedMobile = uad.mobile ?? cachedMobile
  cachedPlatform = uad.platform ?? ''
  uad
    .getHighEntropyValues?.(['fullVersionList', 'platformVersion'])
    .then(v => {
      if (v.fullVersionList?.length) {
        // Pick the most specific brand (skip "Chromium", "Not…" brands).
        const brand =
          v.fullVersionList.find(b => !/Chromium|Not/i.test(b.brand)) ?? v.fullVersionList[0]
        cachedBrowser = { name: brand.brand, version: brand.version }
      }
      if (v.platform) cachedOS = { name: v.platform, version: v.platformVersion ?? '' }
      if (v.platform) cachedPlatform = v.platform
    })
    .catch(() => {})
}

// --- Public API --------------------------------------------------------------

export function getDeviceContext(): DeviceContext {
  const w = typeof window !== 'undefined' ? window : undefined
  const nav: NavigatorExt | undefined = typeof navigator !== 'undefined' ? navigator : undefined
  const conn = nav?.connection

  return {
    browser: { ...cachedBrowser },
    os: { ...cachedOS },
    platform: cachedPlatform,
    mobile: cachedMobile,
    screen: {
      width: w?.screen?.width ?? 0,
      height: w?.screen?.height ?? 0,
      dpr: w?.devicePixelRatio ?? 1,
      orientation: w?.screen?.orientation?.type ?? '',
    },
    viewport: {
      width: w?.innerWidth ?? 0,
      height: w?.innerHeight ?? 0,
    },
    hardware: {
      cores: nav?.hardwareConcurrency ?? null,
      memory: nav?.deviceMemory ?? null,
    },
    network: {
      online: nav?.onLine ?? true,
      type: conn?.effectiveType ?? null,
    },
    locale: {
      languages: nav?.languages ? [...nav.languages] : [],
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    },
    preferences: {
      colorScheme: w?.matchMedia?.('(prefers-color-scheme: dark)')?.matches ? 'dark' : 'light',
    },
  }
}

export function getDeviceContextAttrs(): Record<string, string> {
  const ctx = getDeviceContext()
  return {
    'device.browser.name': ctx.browser.name,
    'device.browser.version': ctx.browser.version,
    'device.os.name': ctx.os.name,
    'device.os.version': ctx.os.version,
    'device.platform': ctx.platform,
    'device.mobile': String(ctx.mobile),
    'device.screen.width': String(ctx.screen.width),
    'device.screen.height': String(ctx.screen.height),
    'device.screen.dpr': String(ctx.screen.dpr),
    'device.screen.orientation': ctx.screen.orientation,
    'device.viewport.width': String(ctx.viewport.width),
    'device.viewport.height': String(ctx.viewport.height),
    'device.hardware.cores': String(ctx.hardware.cores ?? ''),
    'device.hardware.memory': String(ctx.hardware.memory ?? ''),
    'device.network.online': String(ctx.network.online),
    'device.network.type': ctx.network.type ?? '',
    'device.locale.languages': ctx.locale.languages.join(','),
    'device.locale.timezone': ctx.locale.timezone,
    'device.preferences.colorScheme': ctx.preferences.colorScheme,
  }
}
