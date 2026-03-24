import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { GameHeader } from '../components/GameHeader'
import { GameStatus } from '../components/GameStatus'
import { PauseVoteOverlay } from '../components/PauseVoteOverlay'
import { SuspendedScreen } from '../components/SuspendedScreen'
import { GameOverActions } from '../components/GameOverActions'

// ---------------------------------------------------------------------------
// GameHeader
// ---------------------------------------------------------------------------

describe('GameHeader', () => {
  const defaults = {
    gameId: 'tictactoe',
    moveCount: 5,
    canPause: false,
    isPausePending: false,
    isOver: false,
    isSpectator: false,
    onLobby: vi.fn(),
    onPause: vi.fn(),
  }

  it('renders game id and move count', () => {
    render(<GameHeader {...defaults} />)
    expect(screen.getByText('tictactoe')).toBeInTheDocument()
    expect(screen.getByText('Move 5')).toBeInTheDocument()
  })

  it('calls onLobby when ← Lobby is clicked', () => {
    const onLobby = vi.fn()
    render(<GameHeader {...defaults} onLobby={onLobby} />)
    fireEvent.click(screen.getByText('← Lobby'))
    expect(onLobby).toHaveBeenCalledTimes(1)
  })

  it('does not show pause button when canPause is false', () => {
    render(<GameHeader {...defaults} canPause={false} />)
    expect(screen.queryByTestId('pause-btn')).not.toBeInTheDocument()
  })

  it('shows pause button when canPause is true', () => {
    render(<GameHeader {...defaults} canPause={true} />)
    expect(screen.getByTestId('pause-btn')).toBeInTheDocument()
  })

  it('calls onPause when pause button is clicked', () => {
    const onPause = vi.fn()
    render(<GameHeader {...defaults} canPause={true} onPause={onPause} />)
    fireEvent.click(screen.getByTestId('pause-btn'))
    expect(onPause).toHaveBeenCalledTimes(1)
  })

  it('disables pause button when isPausePending is true', () => {
    render(<GameHeader {...defaults} canPause={true} isPausePending={true} />)
    expect(screen.getByTestId('pause-btn')).toBeDisabled()
  })
})

// ---------------------------------------------------------------------------
// GameStatus
// ---------------------------------------------------------------------------

describe('GameStatus', () => {
  const defaults = {
    statusText: 'Your turn',
    isMyTurn: true,
    isOver: false,
    isSuspended: false,
    isSpectator: false,
    winnerId: null,
    playerId: 'player-1',
    opponentId: 'player-2',
    opponentOnline: true,
  }

  it('renders status text', () => {
    render(<GameStatus {...defaults} />)
    expect(screen.getByTestId('game-status')).toHaveTextContent('Your turn')
  })

  it('shows opponent presence when active', () => {
    render(<GameStatus {...defaults} />)
    expect(screen.getByTestId('opponent-presence')).toBeInTheDocument()
    expect(screen.getByTestId('opponent-presence-text')).toHaveTextContent('Opponent online')
  })

  it('shows opponent offline text when opponentOnline is false', () => {
    render(<GameStatus {...defaults} opponentOnline={false} />)
    expect(screen.getByTestId('opponent-presence-text')).toHaveTextContent('Opponent offline')
  })

  it('hides presence indicator when isSpectator', () => {
    render(<GameStatus {...defaults} isSpectator={true} />)
    expect(screen.queryByTestId('opponent-presence')).not.toBeInTheDocument()
  })

  it('hides presence indicator when isSuspended', () => {
    render(<GameStatus {...defaults} isSuspended={true} />)
    expect(screen.queryByTestId('opponent-presence')).not.toBeInTheDocument()
  })

  it('hides presence indicator when opponentId is null', () => {
    render(<GameStatus {...defaults} opponentId={null} />)
    expect(screen.queryByTestId('opponent-presence')).not.toBeInTheDocument()
  })

  it('sets data-online correctly on presence dot', () => {
    render(<GameStatus {...defaults} opponentOnline={false} />)
    expect(screen.getByTestId('opponent-presence-dot')).toHaveAttribute('data-online', 'false')
  })
})

// ---------------------------------------------------------------------------
// PauseVoteOverlay
// ---------------------------------------------------------------------------

describe('PauseVoteOverlay', () => {
  const defaults = {
    votes: ['player-1'],
    required: 2,
    votedPause: false,
    isPending: false,
    isSpectator: false,
    onVote: vi.fn(),
  }

  it('renders vote count', () => {
    render(<PauseVoteOverlay {...defaults} />)
    expect(screen.getByText('1 / 2 voted')).toBeInTheDocument()
  })

  it('shows vote button when participant has not voted', () => {
    render(<PauseVoteOverlay {...defaults} />)
    expect(screen.getByTestId('vote-pause-btn')).toBeInTheDocument()
  })

  it('calls onVote when vote button is clicked', () => {
    const onVote = vi.fn()
    render(<PauseVoteOverlay {...defaults} onVote={onVote} />)
    fireEvent.click(screen.getByTestId('vote-pause-btn'))
    expect(onVote).toHaveBeenCalledTimes(1)
  })

  it('disables vote button when isPending', () => {
    render(<PauseVoteOverlay {...defaults} isPending={true} />)
    expect(screen.getByTestId('vote-pause-btn')).toBeDisabled()
  })

  it('hides vote button and shows waiting text when votedPause', () => {
    render(<PauseVoteOverlay {...defaults} votedPause={true} />)
    expect(screen.queryByTestId('vote-pause-btn')).not.toBeInTheDocument()
    expect(screen.getByText(/waiting for opponent/i)).toBeInTheDocument()
  })

  it('hides vote button for spectators', () => {
    render(<PauseVoteOverlay {...defaults} isSpectator={true} />)
    expect(screen.queryByTestId('vote-pause-btn')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// SuspendedScreen
// ---------------------------------------------------------------------------

describe('SuspendedScreen', () => {
  const defaults = {
    resumeVotes: [],
    resumeRequired: 2,
    votedResume: false,
    isPending: false,
    canResume: true,
    onResume: vi.fn(),
    onBackToLobby: vi.fn(),
  }

  it('renders suspended screen', () => {
    render(<SuspendedScreen {...defaults} />)
    expect(screen.getByTestId('suspended-screen')).toBeInTheDocument()
    expect(screen.getByText('Game Paused')).toBeInTheDocument()
  })

  it('does not show resume vote count when no votes', () => {
    render(<SuspendedScreen {...defaults} resumeVotes={[]} />)
    expect(screen.queryByTestId('resume-vote-count')).not.toBeInTheDocument()
  })

  it('shows resume vote count when votes exist', () => {
    render(<SuspendedScreen {...defaults} resumeVotes={['player-1']} />)
    expect(screen.getByTestId('resume-vote-count')).toHaveTextContent('1 / 2 voted to resume')
  })

  it('shows vote to resume button when canResume', () => {
    render(<SuspendedScreen {...defaults} canResume={true} />)
    expect(screen.getByTestId('vote-resume-btn')).toBeInTheDocument()
  })

  it('hides vote to resume button when canResume is false', () => {
    render(<SuspendedScreen {...defaults} canResume={false} />)
    expect(screen.queryByTestId('vote-resume-btn')).not.toBeInTheDocument()
  })

  it('calls onResume when vote button is clicked', () => {
    const onResume = vi.fn()
    render(<SuspendedScreen {...defaults} onResume={onResume} />)
    fireEvent.click(screen.getByTestId('vote-resume-btn'))
    expect(onResume).toHaveBeenCalledTimes(1)
  })

  it('disables vote button when isPending', () => {
    render(<SuspendedScreen {...defaults} isPending={true} />)
    expect(screen.getByTestId('vote-resume-btn')).toBeDisabled()
  })

  it('shows waiting text when votedResume', () => {
    render(<SuspendedScreen {...defaults} votedResume={true} />)
    expect(screen.getByText(/waiting for opponent/i)).toBeInTheDocument()
  })

  it('calls onBackToLobby when back button is clicked', () => {
    const onBackToLobby = vi.fn()
    render(<SuspendedScreen {...defaults} onBackToLobby={onBackToLobby} />)
    fireEvent.click(screen.getByText('← Back to Lobby'))
    expect(onBackToLobby).toHaveBeenCalledTimes(1)
  })
})

// ---------------------------------------------------------------------------
// GameOverActions
// ---------------------------------------------------------------------------

describe('GameOverActions', () => {
  const defaults = {
    isSpectator: false,
    votedRematch: false,
    rematchVotes: 0,
    totalPlayers: 2,
    isRematchPending: false,
    onBackToLobby: vi.fn(),
    onViewReplay: vi.fn(),
    onRematch: vi.fn(),
  }

  it('renders back to lobby and view replay buttons', () => {
    render(<GameOverActions {...defaults} />)
    expect(screen.getByText('Back to Lobby')).toBeInTheDocument()
    expect(screen.getByTestId('view-replay-btn')).toBeInTheDocument()
  })

  it('calls onBackToLobby when clicked', () => {
    const onBackToLobby = vi.fn()
    render(<GameOverActions {...defaults} onBackToLobby={onBackToLobby} />)
    fireEvent.click(screen.getByText('Back to Lobby'))
    expect(onBackToLobby).toHaveBeenCalledTimes(1)
  })

  it('calls onViewReplay when clicked', () => {
    const onViewReplay = vi.fn()
    render(<GameOverActions {...defaults} onViewReplay={onViewReplay} />)
    fireEvent.click(screen.getByTestId('view-replay-btn'))
    expect(onViewReplay).toHaveBeenCalledTimes(1)
  })

  it('shows rematch button for participants', () => {
    render(<GameOverActions {...defaults} />)
    expect(screen.getByTestId('rematch-btn')).toBeInTheDocument()
  })

  it('hides rematch button for spectators', () => {
    render(<GameOverActions {...defaults} isSpectator={true} />)
    expect(screen.queryByTestId('rematch-btn')).not.toBeInTheDocument()
  })

  it('calls onRematch when rematch button is clicked', () => {
    const onRematch = vi.fn()
    render(<GameOverActions {...defaults} onRematch={onRematch} />)
    fireEvent.click(screen.getByTestId('rematch-btn'))
    expect(onRematch).toHaveBeenCalledTimes(1)
  })

  it('disables rematch button when votedRematch is true', () => {
    render(<GameOverActions {...defaults} votedRematch={true} />)
    expect(screen.getByTestId('rematch-btn')).toBeDisabled()
  })

  it('shows waiting text with vote count when votedRematch', () => {
    render(<GameOverActions {...defaults} votedRematch={true} rematchVotes={1} totalPlayers={2} />)
    expect(screen.getByTestId('rematch-btn')).toHaveTextContent('Waiting for opponent… (1/2)')
  })

  it('shows vote count on rematch button when others have voted', () => {
    render(<GameOverActions {...defaults} rematchVotes={1} totalPlayers={2} />)
    expect(screen.getByTestId('rematch-btn')).toHaveTextContent('Rematch (1/2 voted)')
  })

  it('shows plain Rematch text when no votes', () => {
    render(<GameOverActions {...defaults} rematchVotes={0} />)
    expect(screen.getByTestId('rematch-btn')).toHaveTextContent('Rematch')
  })
})
