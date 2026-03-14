package loveletter

// CardName identifies a Love Letter card type.
type CardName string

const (
	CardSpy        CardName = "spy"
	CardGuard      CardName = "guard"
	CardPriest     CardName = "priest"
	CardBaron      CardName = "baron"
	CardHandmaid   CardName = "handmaid"
	CardPrince     CardName = "prince"
	CardChancellor CardName = "chancellor"
	CardKing       CardName = "king"
	CardCountess   CardName = "countess"
	CardPrincess   CardName = "princess"

	// CardPenaltyLose is a synthetic card used by the turn timer to eliminate
	// the active player when their turn times out. It is never in the deck and
	// never valid as a player-initiated move.
	CardPenaltyLose CardName = "penalty_lose"
)

// cardValues maps each card to its numeric value.
var cardValues = map[CardName]int{
	CardSpy:        0,
	CardGuard:      1,
	CardPriest:     2,
	CardBaron:      3,
	CardHandmaid:   4,
	CardPrince:     5,
	CardChancellor: 6,
	CardKing:       7,
	CardCountess:   8,
	CardPrincess:   9,
}

// cardValue returns the numeric value of a card.
// Returns -1 for unknown cards.
func cardValue(c CardName) int {
	v, ok := cardValues[c]
	if !ok {
		return -1
	}
	return v
}

// deckComposition lists all 21 cards in the updated edition deck.
var deckComposition = []CardName{
	CardSpy, CardSpy,
	CardGuard, CardGuard, CardGuard, CardGuard, CardGuard, CardGuard,
	CardPriest, CardPriest,
	CardBaron, CardBaron,
	CardHandmaid, CardHandmaid,
	CardPrince, CardPrince,
	CardChancellor, CardChancellor,
	CardKing,
	CardCountess,
	CardPrincess,
}

// buildDeck returns a fresh unshuffled copy of the full 20-card deck.
func buildDeck() []CardName {
	deck := make([]CardName, len(deckComposition))
	copy(deck, deckComposition)
	return deck
}

// allNonGuardCards lists every card name except Guard, used for Guard guess
// validation. The Guard cannot be named as a guess target.
var allNonGuardCards = []CardName{
	CardSpy,
	CardPriest,
	CardBaron,
	CardHandmaid,
	CardPrince,
	CardChancellor,
	CardKing,
	CardCountess,
	CardPrincess,
}

// isValidGuess returns true if c is a legal Guard guess target.
func isValidGuess(c CardName) bool {
	for _, v := range allNonGuardCards {
		if v == c {
			return true
		}
	}
	return false
}

// tokensToWin returns the number of affection tokens needed to win the game
// for a given player count.
func tokensToWin(playerCount int) int {
	switch playerCount {
	case 2:
		return 7
	case 3:
		return 5
	case 4:
		return 4
	default: // 5
		return 3
	}
}
