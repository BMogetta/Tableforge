import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

vi.mock('../sfx', () => ({
  sfx: { play: vi.fn() },
}))

import { sfx } from '../sfx'

let mockPermission: NotificationPermission = 'default'
let mockRequestPermission: ReturnType<typeof vi.fn>
let mockNotificationInstance: { body: string; tag: string } | null = null

function setupNotificationMock() {
  mockPermission = 'default'
  mockRequestPermission = vi.fn(async () => {
    mockPermission = 'granted'
    return 'granted' as NotificationPermission
  })
  mockNotificationInstance = null

  vi.stubGlobal(
    'Notification',
    class MockNotification {
      body: string
      tag: string
      constructor(_title: string, opts: { body: string; tag: string }) {
        this.body = opts.body
        this.tag = opts.tag
        mockNotificationInstance = this
      }
      static get permission() {
        return mockPermission
      }
      static requestPermission = mockRequestPermission
    },
  )
}

beforeEach(() => {
  setupNotificationMock()
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  vi.clearAllMocks()
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

// -- requestPermission --------------------------------------------------------

describe('requestPermission', () => {
  it('calls Notification.requestPermission when permission is default', async () => {
    mockPermission = 'default'
    const { requestPermission } = await import('../turn-notifier')
    const result = await requestPermission()
    expect(mockRequestPermission).toHaveBeenCalledOnce()
    expect(result).toBe('granted')
  })

  it('does not prompt when permission is already granted', async () => {
    mockPermission = 'granted'
    const { requestPermission } = await import('../turn-notifier')
    const result = await requestPermission()
    expect(mockRequestPermission).not.toHaveBeenCalled()
    expect(result).toBe('granted')
  })

  it('does not prompt when permission is denied', async () => {
    mockPermission = 'denied'
    const { requestPermission } = await import('../turn-notifier')
    const result = await requestPermission()
    expect(mockRequestPermission).not.toHaveBeenCalled()
    expect(result).toBe('denied')
  })

  it('returns denied when Notification API is not available', async () => {
    // @ts-expect-error — deliberately removing Notification from window
    delete window.Notification
    const { requestPermission } = await import('../turn-notifier')
    const result = await requestPermission()
    expect(result).toBe('denied')
  })
})

// -- notifyTurn ---------------------------------------------------------------

describe('notifyTurn', () => {
  it('does nothing when tab is visible', async () => {
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    mockPermission = 'granted'
    const { notifyTurn } = await import('../turn-notifier')
    notifyTurn()
    expect(mockNotificationInstance).toBeNull()
    expect(sfx.play).not.toHaveBeenCalled()
  })

  it('shows notification and plays sound when tab is hidden', async () => {
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    mockPermission = 'granted'
    const { notifyTurn } = await import('../turn-notifier')
    notifyTurn()
    expect(mockNotificationInstance).not.toBeNull()
    expect(mockNotificationInstance!.tag).toBe('turn-notification')
    expect(sfx.play).toHaveBeenCalledWith('game.my_turn')
  })

  it('plays sound but skips notification when permission is denied', async () => {
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    mockPermission = 'denied'
    const { notifyTurn } = await import('../turn-notifier')
    notifyTurn()
    expect(mockNotificationInstance).toBeNull()
    expect(sfx.play).toHaveBeenCalledWith('game.my_turn')
  })

  it('uses tag to replace previous notification', async () => {
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    mockPermission = 'granted'
    const { notifyTurn } = await import('../turn-notifier')
    notifyTurn()
    const first = mockNotificationInstance
    notifyTurn()
    expect(first!.tag).toBe('turn-notification')
    expect(mockNotificationInstance!.tag).toBe('turn-notification')
  })
})

// -- WakeLock -----------------------------------------------------------------

describe('acquireWakeLock / releaseWakeLock', () => {
  let mockRelease: ReturnType<typeof vi.fn>
  let releaseHandler: (() => void) | null

  beforeEach(() => {
    releaseHandler = null
    mockRelease = vi.fn(async () => {
      releaseHandler?.()
    })

    vi.stubGlobal('navigator', {
      ...navigator,
      wakeLock: {
        request: vi.fn(async () => ({
          release: mockRelease,
          addEventListener: (_event: string, handler: () => void) => {
            releaseHandler = handler
          },
        })),
      },
    })
  })

  it('acquires wake lock', async () => {
    vi.resetModules()
    const { acquireWakeLock } = await import('../turn-notifier')
    await acquireWakeLock()
    expect(navigator.wakeLock.request).toHaveBeenCalledWith('screen')
  })

  it('releases wake lock', async () => {
    vi.resetModules()
    const { acquireWakeLock, releaseWakeLock } = await import('../turn-notifier')
    await acquireWakeLock()
    await releaseWakeLock()
    expect(mockRelease).toHaveBeenCalledOnce()
  })

  it('release is no-op when not held', async () => {
    vi.resetModules()
    const { releaseWakeLock } = await import('../turn-notifier')
    await releaseWakeLock()
    // No error thrown
  })
})
