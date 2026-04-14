import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ProfileHeader } from '../components/ProfileHeader'

describe('ProfileHeader', () => {
  it('renders username and bio', () => {
    render(
      <ProfileHeader playerId='p1' username='alice' bio='I love board games' isLoading={false} />,
    )
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('I love board games')).toBeInTheDocument()
  })

  it('renders avatar image when provided', () => {
    render(
      <ProfileHeader
        playerId='p1'
        username='alice'
        avatarUrl='https://example.com/avatar.png'
        isLoading={false}
      />,
    )
    const img = screen.getByAltText('alice')
    expect(img).toHaveAttribute('src', 'https://example.com/avatar.png')
  })

  it('renders avatar placeholder when no image', () => {
    render(<ProfileHeader playerId='p1' username='alice' isLoading={false} />)
    expect(screen.getByText('A')).toBeInTheDocument()
  })

  it('renders country when provided', () => {
    render(<ProfileHeader playerId='p1' username='alice' country='AR' isLoading={false} />)
    expect(screen.getByText('AR')).toBeInTheDocument()
  })

  it('shows loading state', () => {
    render(<ProfileHeader playerId='p1' isLoading={true} />)
    expect(screen.getByText('Loading profile...')).toBeInTheDocument()
  })

  it('falls back to playerId prefix when no username', () => {
    render(<ProfileHeader playerId='abcd1234-rest' isLoading={false} />)
    expect(screen.getByText('abcd1234')).toBeInTheDocument()
  })

  it('renders BOT badge with profile when isBot is true', () => {
    render(
      <ProfileHeader playerId='bot1' username='bot_easy_1' isBot botProfile='easy' isLoading={false} />,
    )
    expect(screen.getByTestId('profile-bot-badge')).toBeInTheDocument()
    expect(screen.getByTestId('profile-bot-badge')).toHaveTextContent('easy')
  })

  it('does not render BOT badge when isBot is false', () => {
    render(<ProfileHeader playerId='p1' username='alice' isLoading={false} />)
    expect(screen.queryByTestId('profile-bot-badge')).not.toBeInTheDocument()
  })
})
