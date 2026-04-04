package rootaccess

// CardName identifies a Root Access card type.
type CardName string

const (
	CardBackdoor      CardName = "backdoor"
	CardPing          CardName = "ping"
	CardSniffer       CardName = "sniffer"
	CardBufferOverflow CardName = "buffer_overflow"
	CardFirewall      CardName = "firewall"
	CardReboot        CardName = "reboot"
	CardDebugger      CardName = "debugger"
	CardSwap          CardName = "swap"
	CardEncryptedKey  CardName = "encrypted_key"
	CardRoot          CardName = "root"

	// CardPenaltyLose is a synthetic card used by the turn timer to eliminate
	// the active player when their turn times out. It is never in the deck and
	// never valid as a player-initiated move.
	CardPenaltyLose CardName = "penalty_lose"
)

// cardValues maps each card to its numeric value.
var cardValues = map[CardName]int{
	CardBackdoor:       0,
	CardPing:           1,
	CardSniffer:        2,
	CardBufferOverflow: 3,
	CardFirewall:       4,
	CardReboot:         5,
	CardDebugger:       6,
	CardSwap:           7,
	CardEncryptedKey:   8,
	CardRoot:           9,
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

// deckComposition lists all 21 cards in the deck.
var deckComposition = []CardName{
	CardBackdoor, CardBackdoor,
	CardPing, CardPing, CardPing, CardPing, CardPing, CardPing,
	CardSniffer, CardSniffer,
	CardBufferOverflow, CardBufferOverflow,
	CardFirewall, CardFirewall,
	CardReboot, CardReboot,
	CardDebugger, CardDebugger,
	CardSwap,
	CardEncryptedKey,
	CardRoot,
}

// buildDeck returns a fresh unshuffled copy of the full 21-card deck.
func buildDeck() []CardName {
	deck := make([]CardName, len(deckComposition))
	copy(deck, deckComposition)
	return deck
}

// allNonPingCards lists every card name except Ping, used for Ping guess
// validation. Ping cannot be named as a guess target.
var allNonPingCards = []CardName{
	CardBackdoor,
	CardSniffer,
	CardBufferOverflow,
	CardFirewall,
	CardReboot,
	CardDebugger,
	CardSwap,
	CardEncryptedKey,
	CardRoot,
}

// isValidGuess returns true if c is a legal Ping guess target.
func isValidGuess(c CardName) bool {
	for _, v := range allNonPingCards {
		if v == c {
			return true
		}
	}
	return false
}

// tokensToWin returns the number of Access Tokens needed to win the game
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
