// Package mcts implements IS-MCTS (Information Set Monte Carlo Tree Search).
// It is game-agnostic — callers provide a bot.BotAdapter for their specific game.
//
// Usage:
//
//	move, err := mcts.Search(ctx, state, playerID, adapter, cfg)
//
// Search honours ctx cancellation and cfg.MaxThinkTime. Wrap ctx with a
// deadline before calling if you want a hard wall-clock limit:
//
//	ctx, cancel := context.WithTimeout(ctx, cfg.MaxThinkTime)
//	defer cancel()
//	move, err := mcts.Search(ctx, state, playerID, adapter, cfg)
package mcts

import (
	"context"
	"math/rand"

	"github.com/tableforge/server/internal/bot"
	"github.com/tableforge/server/internal/domain/engine"
)

// ErrNoMoves is re-exported from bot for callers that import only this package.
// The canonical sentinel lives in bot.ErrNoMoves to avoid an import cycle.
var ErrNoMoves = bot.ErrNoMoves

// Search runs IS-MCTS from root for playerID and returns the best move found.
//
// Algorithm per iteration:
//  1. Determinize — sample a world state consistent with playerID's knowledge.
//  2. Select      — walk the tree via UCB1 until a non-fully-expanded node.
//  3. Expand      — add one new child for an untried move.
//  4. Rollout     — simulate to terminal using adapter.RolloutPolicy.
//  5. Backpropagate — propagate result up the tree from playerID's perspective.
//
// Final move: child of root with the most visits (robust child criterion).
//
// ctx cancellation is checked at the top of every iteration. Pass a context
// with a deadline matching cfg.MaxThinkTime for wall-clock enforcement.
func Search(
	ctx context.Context,
	root bot.BotGameState,
	playerID engine.PlayerID,
	adapter bot.BotAdapter,
	cfg bot.BotConfig,
) (bot.BotMove, error) {
	rootMoves := adapter.ValidMoves(root)
	if len(rootMoves) == 0 {
		return "", bot.ErrNoMoves
	}

	rootNode := newNode(nil, "", "", rootMoves)

	for i := 0; i < cfg.Iterations; i++ {
		if ctx.Err() != nil {
			break
		}

		// 1. Determinize — produce a fully-observable world consistent with
		//    what playerID can legally know. Each iteration gets an independent
		//    sample so the search averages over the information set.
		det := adapter.Determinize(root.Clone(), playerID)

		// 2 & 3. Select + Expand — walk to a node to evaluate.
		n, state := selectAndExpand(rootNode, det, adapter, cfg.ExplorationC)

		// 4. Rollout — random/heuristic playout to a terminal state.
		score := rollout(state, adapter, playerID)

		// 5. Backpropagate — update visit counts and wins from playerID's perspective.
		n.backpropagate(score, playerID)
	}

	if len(rootNode.children) == 0 {
		// All iterations were cancelled before a single expansion.
		// Fall back to the first legal move rather than returning an error,
		// so the bot never deadlocks a live game.
		return rootMoves[0], nil
	}

	return rootNode.mostVisited().move, nil
}

// selectAndExpand walks the tree from n using UCB1 until it finds a node that
// is not fully expanded, then expands it by one child. Returns the new (or
// terminal) node and the game state at that node.
func selectAndExpand(
	n *node,
	state bot.BotGameState,
	adapter bot.BotAdapter,
	explorationC float64,
) (*node, bot.BotGameState) {
	for {
		if adapter.IsTerminal(state) {
			return n, state
		}

		// Guard: if a non-terminal state has no legal moves, treat it as
		// terminal to avoid expanding a dead-end node that would panic bestChild.
		moves := adapter.ValidMoves(state)
		if len(moves) == 0 {
			return n, state
		}

		if !n.isFullyExpanded() {
			return expand(n, state, adapter)
		}

		if len(n.children) == 0 {
			return n, state
		}

		best := n.bestChild(explorationC)
		state = adapter.ApplyMove(state, best.move)
		n = best
	}
}

// expand picks the last untried move from n, applies it to state, creates a
// child node, and returns the child with the resulting state.
func expand(
	n *node,
	state bot.BotGameState,
	adapter bot.BotAdapter,
) (*node, bot.BotGameState) {
	// The player acting is the current player before the move is applied.
	actingPlayer := adapter.CurrentPlayer(state)

	// Pop the last untried move (order is irrelevant).
	move := n.untriedMoves[len(n.untriedMoves)-1]

	nextState := adapter.ApplyMove(state, move)
	childMoves := adapter.ValidMoves(nextState)

	child := n.expand(move, actingPlayer, childMoves)
	return child, nextState
}

// rollout simulates from state to a terminal state using adapter.RolloutPolicy
// and returns the result score for playerID (1.0 win, 0.0 loss, 0.5 draw).
func rollout(
	state bot.BotGameState,
	adapter bot.BotAdapter,
	playerID engine.PlayerID,
) float64 {
	for !adapter.IsTerminal(state) {
		moves := adapter.ValidMoves(state)
		if len(moves) == 0 {
			break
		}
		move := adapter.RolloutPolicy(state, moves)
		state = adapter.ApplyMove(state, move)
	}
	return adapter.Result(state, playerID)
}

// RandomRolloutPolicy is a convenience helper that adapters can delegate to
// when they have no heuristic to apply. It selects a uniformly random move.
func RandomRolloutPolicy(_ bot.BotGameState, moves []bot.BotMove) bot.BotMove {
	return moves[rand.Intn(len(moves))]
}
