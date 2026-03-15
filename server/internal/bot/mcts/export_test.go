// export_test.go exposes unexported node internals for white-box unit tests.
// This file is compiled only during `go test`.
package mcts

import (
	"math"

	"github.com/tableforge/server/internal/bot"
	"github.com/tableforge/server/internal/domain/engine"
)

// TestNode wraps the internal node type so node_test.go can construct and
// inspect nodes without exporting node itself.
type TestNode struct {
	n *node
}

// NewTestNode creates a root TestNode with the given untried moves.
func NewTestNode(moves []bot.BotMove) *TestNode {
	return &TestNode{n: newNode(nil, "", "", moves)}
}

// NewTestChild creates a child TestNode attached to parent.
func (tn *TestNode) NewTestChild(move bot.BotMove, player engine.PlayerID, moves []bot.BotMove) *TestNode {
	child := tn.n.expand(move, player, moves)
	return &TestNode{n: child}
}

// Backpropagate calls the internal backpropagate method.
func (tn *TestNode) Backpropagate(score float64, playerID engine.PlayerID) {
	tn.n.backpropagate(score, playerID)
}

// Visits returns the node's visit count.
func (tn *TestNode) Visits() int { return tn.n.visits }

// Wins returns the node's cumulative win score.
func (tn *TestNode) Wins() float64 { return tn.n.wins }

// UCB1 exposes the UCB1 calculation.
func (tn *TestNode) UCB1(explorationC float64) float64 { return tn.n.UCB1(explorationC) }

// SetVisits and SetWins allow tests to set up specific scenarios.
func (tn *TestNode) SetVisits(v int)   { tn.n.visits = v }
func (tn *TestNode) SetWins(w float64) { tn.n.wins = w }

// PosInf returns positive infinity, used in UCB1 assertions.
func PosInf() float64 { return math.Inf(1) }
