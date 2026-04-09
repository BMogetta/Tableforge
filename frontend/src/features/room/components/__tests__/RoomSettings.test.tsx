import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { RoomSettings } from '../RoomSettings'
import type { LobbySetting } from '@/lib/schema-generated.zod'

const mockUpdateSetting = vi.fn()

vi.mock('@/features/room/api', () => ({
  rooms: {
    updateSetting: (...args: unknown[]) => mockUpdateSetting(...args),
  },
}))

const selectDescriptor: LobbySetting = {
  key: 'difficulty',
  label: 'Difficulty',
  description: 'Game difficulty level',
  type: 'select',
  default: 'normal',
  options: [
    { value: 'easy', label: 'Easy' },
    { value: 'normal', label: 'Normal' },
    { value: 'hard', label: 'Hard' },
  ],
}

const intDescriptor: LobbySetting = {
  key: 'turn_timeout',
  label: 'Turn Timeout',
  type: 'int',
  default: '30',
  min: 10,
  max: 300,
}

let onSettingChange: ReturnType<typeof vi.fn>

beforeEach(() => {
  vi.clearAllMocks()
  onSettingChange = vi.fn()
  mockUpdateSetting.mockResolvedValue(undefined)
})

describe('RoomSettings', () => {
  it('returns null when no descriptors', () => {
    const { container } = render(
      <RoomSettings
        roomId='r1'
        isOwner={true}
        descriptors={[]}
        values={{}}
        onSettingChange={onSettingChange}
      />,
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders setting labels and descriptions', () => {
    render(
      <RoomSettings
        roomId='r1'
        isOwner={false}
        descriptors={[selectDescriptor]}
        values={{ difficulty: 'normal' }}
        onSettingChange={onSettingChange}
      />,
    )
    expect(screen.getByText('Difficulty')).toBeInTheDocument()
    expect(screen.getByText('Game difficulty level')).toBeInTheDocument()
  })

  it('shows readonly value for non-owner (select)', () => {
    render(
      <RoomSettings
        roomId='r1'
        isOwner={false}
        descriptors={[selectDescriptor]}
        values={{ difficulty: 'hard' }}
        onSettingChange={onSettingChange}
      />,
    )
    // Shows human-readable label, not raw value
    expect(screen.getByText('Hard')).toBeInTheDocument()
    expect(screen.queryByRole('combobox')).toBeNull()
  })

  it('shows readonly value for non-owner (int)', () => {
    render(
      <RoomSettings
        roomId='r1'
        isOwner={false}
        descriptors={[intDescriptor]}
        values={{ turn_timeout: '60' }}
        onSettingChange={onSettingChange}
      />,
    )
    expect(screen.getByText('60')).toBeInTheDocument()
    expect(screen.queryByRole('spinbutton')).toBeNull()
  })

  it('shows select control for owner', () => {
    render(
      <RoomSettings
        roomId='r1'
        isOwner={true}
        descriptors={[selectDescriptor]}
        values={{ difficulty: 'normal' }}
        onSettingChange={onSettingChange}
      />,
    )
    const select = screen.getByRole('combobox')
    expect(select).toBeInTheDocument()
    expect(select).toHaveValue('normal')
  })

  it('shows number input for owner (int type)', () => {
    render(
      <RoomSettings
        roomId='r1'
        isOwner={true}
        descriptors={[intDescriptor]}
        values={{ turn_timeout: '30' }}
        onSettingChange={onSettingChange}
      />,
    )
    const input = screen.getByRole('spinbutton')
    expect(input).toBeInTheDocument()
    expect(input).toHaveValue(30)
  })

  it('calls API and onSettingChange when owner changes select', async () => {
    const user = userEvent.setup()
    render(
      <RoomSettings
        roomId='r1'
        isOwner={true}
        descriptors={[selectDescriptor]}
        values={{ difficulty: 'normal' }}
        onSettingChange={onSettingChange}
      />,
    )
    await user.selectOptions(screen.getByRole('combobox'), 'hard')

    expect(mockUpdateSetting).toHaveBeenCalledWith('r1', 'difficulty', 'hard')
    expect(onSettingChange).toHaveBeenCalledWith('difficulty', 'hard')
  })

  it('shows error on API failure', async () => {
    mockUpdateSetting.mockRejectedValue(new Error('server error'))
    const user = userEvent.setup()
    render(
      <RoomSettings
        roomId='r1'
        isOwner={true}
        descriptors={[selectDescriptor]}
        values={{ difficulty: 'normal' }}
        onSettingChange={onSettingChange}
      />,
    )
    await user.selectOptions(screen.getByRole('combobox'), 'hard')

    expect(await screen.findByText(/error/i)).toBeInTheDocument()
    expect(onSettingChange).not.toHaveBeenCalled()
  })

  it('uses default value when no value provided', () => {
    render(
      <RoomSettings
        roomId='r1'
        isOwner={true}
        descriptors={[selectDescriptor]}
        values={{}}
        onSettingChange={onSettingChange}
      />,
    )
    expect(screen.getByRole('combobox')).toHaveValue('normal')
  })
})
