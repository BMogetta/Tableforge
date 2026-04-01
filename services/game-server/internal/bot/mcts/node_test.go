package mcts_test

import (
	"math"
	"testing"

	"github.com/recess/game-server/internal/bot"
	"github.com/recess/game-server/internal/bot/mcts"
	"github.com/recess/game-server/internal/domain/engine"
)

const testPlayerID engine.PlayerID = "player-a"
const otherPlayerID engine.PlayerID = "player-b"

// TestNode_UCB1_Unvisited asserts that an unvisited node returns +Inf.
func TestNode_UCB1_Unvisited(t *testing.T) {
	root := mcts.NewTestNode([]bot.BotMove{`{"take":1}`})
	root.SetVisits(10)

	child := root.NewTestChild(`{"take":1}`, testPlayerID, nil)

	got := child.UCB1(1.41)
	if !math.IsInf(got, 1) {
		t.Errorf("expected +Inf for unvisited node, got %v", got)
	}
}

// TestNode_UCB1_PureExploitation asserts exploitation term with explorationC=0.
func TestNode_UCB1_PureExploitation(t *testing.T) {
	root := mcts.NewTestNode([]bot.BotMove{`{"take":1}`})
	root.SetVisits(10)

	child := root.NewTestChild(`{"take":1}`, testPlayerID, nil)
	child.SetVisits(5)
	child.SetWins(4)

	got := child.UCB1(0)
	want := 0.8 // 4/5
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("UCB1 = %.6f, want %.6f", got, want)
	}
}

// TestNode_UCB1_Standard asserts the full UCB1 formula with a non-zero C.
func TestNode_UCB1_Standard(t *testing.T) {
	root := mcts.NewTestNode([]bot.BotMove{`{"take":1}`})
	root.SetVisits(10)

	child := root.NewTestChild(`{"take":1}`, testPlayerID, nil)
	child.SetVisits(2)
	child.SetWins(1)

	// exploitation = 1/2 = 0.5
	// exploration  = 1.41 * sqrt(ln(10)/2) = 1.41 * sqrt(1.1513) = 1.41 * 1.0730 ≈ 1.5130
	// total ≈ 2.013
	got := child.UCB1(1.41)
	want := 0.5 + 1.41*math.Sqrt(math.Log(10)/2)
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("UCB1 = %.6f, want %.6f", got, want)
	}
}

// TestNode_Backpropagate_WinForPlayerID asserts that a win for playerID
// accumulates correctly at each node based on which player owns the node.
//
// Tree: root ("") → child (playerA) → grandchild (playerB)
// Backpropagate score=1.0 (playerA wins) with playerID=playerA.
//
// Expected:
//
//	grandchild (playerB): wins += 1-1.0 = 0.0
//	child      (playerA): wins += 1.0
//	root       ("")     : wins += 1-1.0 = 0.0  (root player is "" ≠ playerA)
func TestNode_Backpropagate_WinForPlayerID(t *testing.T) {
	root := mcts.NewTestNode([]bot.BotMove{`{"take":1}`, `{"take":2}`})
	child := root.NewTestChild(`{"take":1}`, testPlayerID, []bot.BotMove{`{"take":1}`})
	grandchild := child.NewTestChild(`{"take":1}`, otherPlayerID, nil)

	grandchild.Backpropagate(1.0, testPlayerID)

	assertNode(t, "grandchild", grandchild, 1, 0.0)
	assertNode(t, "child", child, 1, 1.0)
	assertNode(t, "root", root, 1, 0.0)
}

// TestNode_Backpropagate_LossForPlayerID asserts a loss propagates correctly.
//
// Tree: root ("") → child (playerA) → grandchild (playerB)
// Backpropagate score=0.0 (playerA loses) with playerID=playerA.
//
// Expected:
//
//	grandchild (playerB): wins += 1-0.0 = 1.0
//	child      (playerA): wins += 0.0
//	root       ("")     : wins += 1-0.0 = 1.0
func TestNode_Backpropagate_LossForPlayerID(t *testing.T) {
	root := mcts.NewTestNode([]bot.BotMove{`{"take":1}`, `{"take":2}`})
	child := root.NewTestChild(`{"take":1}`, testPlayerID, []bot.BotMove{`{"take":1}`})
	grandchild := child.NewTestChild(`{"take":1}`, otherPlayerID, nil)

	grandchild.Backpropagate(0.0, testPlayerID)

	assertNode(t, "grandchild", grandchild, 1, 1.0)
	assertNode(t, "child", child, 1, 0.0)
	assertNode(t, "root", root, 1, 1.0)
}

// TestNode_Backpropagate_MultipleUpdates asserts accumulation over multiple
// backpropagations.
func TestNode_Backpropagate_MultipleUpdates(t *testing.T) {
	root := mcts.NewTestNode([]bot.BotMove{`{"take":1}`})
	child := root.NewTestChild(`{"take":1}`, testPlayerID, nil)

	child.Backpropagate(1.0, testPlayerID) // child wins+=1.0, root wins+=0.0
	child.Backpropagate(0.0, testPlayerID) // child wins+=0.0, root wins+=1.0
	child.Backpropagate(0.5, testPlayerID) // child wins+=0.5, root wins+=0.5

	assertNode(t, "child", child, 3, 1.5)
	assertNode(t, "root", root, 3, 1.5)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertNode(t *testing.T, name string, n *mcts.TestNode, wantVisits int, wantWins float64) {
	t.Helper()
	if n.Visits() != wantVisits {
		t.Errorf("%s: visits = %d, want %d", name, n.Visits(), wantVisits)
	}
	if math.Abs(n.Wins()-wantWins) > 1e-9 {
		t.Errorf("%s: wins = %.6f, want %.6f", name, n.Wins(), wantWins)
	}
}
