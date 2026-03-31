package mcts

import (
	"math"

	"github.com/tableforge/game-server/internal/bot"
	"github.com/tableforge/game-server/internal/domain/engine"
)

// node is a single node in the MCTS tree.
// wins stores cumulative score from playerID's perspective — the same playerID
// that is passed to Search and threaded through backpropagate. Each node
// accumulates wins for the player who made node.move when that player is
// playerID, and 1-score otherwise.
type node struct {
	move   bot.BotMove     // move that led to this node; zero value for root
	player engine.PlayerID // player who made node.move

	parent   *node
	children []*node

	visits int
	wins   float64 // cumulative score from playerID's perspective

	untriedMoves []bot.BotMove
}

// newNode allocates a node. untriedMoves is consumed by expand() over time.
func newNode(parent *node, move bot.BotMove, player engine.PlayerID, untriedMoves []bot.BotMove) *node {
	// Copy the slice so the caller's slice is never mutated.
	tried := make([]bot.BotMove, len(untriedMoves))
	copy(tried, untriedMoves)
	return &node{
		move:         move,
		player:       player,
		parent:       parent,
		untriedMoves: tried,
	}
}

// UCB1 returns the Upper Confidence Bound score for this node.
// Unvisited nodes return +Inf so they are always selected first.
// explorationC is the exploration constant (√2 ≈ 1.41 is canonical).
func (n *node) UCB1(explorationC float64) float64 {
	if n.visits == 0 {
		return math.Inf(1)
	}
	exploitation := n.wins / float64(n.visits)
	exploration := explorationC * math.Sqrt(math.Log(float64(n.parent.visits))/float64(n.visits))
	return exploitation + exploration
}

// bestChild returns the child with the highest UCB1 score.
// Panics if called on a node with no children — callers must guard with
// isFullyExpanded() and len(children) > 0.
func (n *node) bestChild(explorationC float64) *node {
	best := n.children[0]
	bestScore := best.UCB1(explorationC)
	for _, c := range n.children[1:] {
		if s := c.UCB1(explorationC); s > bestScore {
			bestScore = s
			best = c
		}
	}
	return best
}

// mostVisited returns the child with the highest visit count.
// Used for final move selection after all iterations are complete.
// Panics if called on a leaf — callers must ensure len(children) > 0.
func (n *node) mostVisited() *node {
	best := n.children[0]
	for _, c := range n.children[1:] {
		if c.visits > best.visits {
			best = c
		}
	}
	return best
}

// isFullyExpanded reports whether all legal moves from this node have been
// tried at least once (i.e. untriedMoves is empty).
func (n *node) isFullyExpanded() bool {
	return len(n.untriedMoves) == 0
}

// expand pops the last untried move, creates a child node for it, attaches
// it to n, and returns the child.
//
// childPlayer is the player who acted to reach the child
// (adapter.CurrentPlayer of the state BEFORE the move was applied).
func (n *node) expand(move bot.BotMove, childPlayer engine.PlayerID, untriedInChild []bot.BotMove) *node {
	// Pop move from untried list (order does not matter).
	last := len(n.untriedMoves) - 1
	n.untriedMoves = n.untriedMoves[:last]

	child := newNode(n, move, childPlayer, untriedInChild)
	n.children = append(n.children, child)
	return child
}

// backpropagate walks from this node to the root, incrementing visit counts
// and accumulating the score from playerID's perspective.
//
// score is the result for playerID (1.0 win, 0.0 loss, 0.5 draw).
// Each node accumulates wins when its player matches playerID, and 1-score
// otherwise — so UCB1 at every level reflects the correct player's interest.
func (n *node) backpropagate(score float64, playerID engine.PlayerID) {
	for current := n; current != nil; current = current.parent {
		current.visits++
		if current.player == playerID {
			current.wins += score
		} else {
			current.wins += 1.0 - score
		}
	}
}
