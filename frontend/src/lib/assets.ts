export type AssetType = 'image' | 'audio' | 'font'

export interface GameAsset {
  type: AssetType
  url: string
  /** Optional key for caching — defaults to url */
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
 */
export async function loadAssets(
  assets: GameAsset[],
  onProgress?: (p: LoadProgress) => void
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
    assets.map(asset =>
      loadSingleAsset(asset)
        .then(report)
        .catch(report) // skip failed assets silently
    )
  )
}

function loadSingleAsset(asset: GameAsset): Promise<void> {
  switch (asset.type) {
    case 'image':
      return new Promise((resolve, reject) => {
        const img = new Image()
        img.onload = () => resolve()
        img.onerror = reject
        img.src = asset.url
      })

    case 'audio':
      return new Promise((resolve, reject) => {
        const audio = new Audio()
        audio.oncanplaythrough = () => resolve()
        audio.onerror = reject
        audio.src = asset.url
        audio.load()
      })

    case 'font':
      return document.fonts.load(`1em ${asset.url}`).then(() => {})

    default:
      return Promise.resolve()
  }
}