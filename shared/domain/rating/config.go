package rating

// ---------------------------------------------------------------------------
// Default constants
// ---------------------------------------------------------------------------

const (
	// DefaultMMR is the starting MMR and DisplayRating for new players.
	DefaultMMR = 1500.0

	// EloScale is the logistic curve denominator. The classic Elo system
	// uses 400, meaning a 400-point gap ≈ 10:1 expected win ratio.
	EloScale = 400.0

	// ---------------------------------------------------------------------------
	// Dynamic K-factor boundaries
	// ---------------------------------------------------------------------------

	// KMax is the K-factor for brand-new players (≤ CalibrationGames).
	// High values let the system converge quickly during early placement.
	KMax = 48.0

	// KMin is the floor K-factor for very experienced players.
	// This prevents ratings from becoming completely static.
	KMin = 10.0

	// KDefault is the K-factor once calibration is over but before the
	// player is considered a veteran.
	KDefault = 24.0

	// CalibrationGames is the number of games during which the elevated
	// KMax is used. Afterwards K decays toward KMin.
	CalibrationGames = 30

	// VeteranGames is the game count at which K reaches KMin.
	VeteranGames = 200

	// ---------------------------------------------------------------------------
	// Streak modifiers
	// ---------------------------------------------------------------------------

	// StreakThreshold is the minimum consecutive wins/losses before the
	// streak multiplier kicks in.
	StreakThreshold = 3

	// StreakMultiplierMax caps the additional K bonus from streaks.
	StreakMultiplierMax = 1.5

	// ---------------------------------------------------------------------------
	// Display-rating convergence
	// ---------------------------------------------------------------------------

	// DisplayConvergenceRate controls how fast the public rating chases
	// the hidden MMR each game. Value ∈ (0,1]; 1 means instant sync.
	//   DisplayRating += ConvergenceRate * (MMR - DisplayRating)
	DisplayConvergenceRate = 0.15
)

// Config bundles every tunable so callers can override defaults without
// touching the package-level constants.
type Config struct {
	DefaultMMR             float64
	EloScale               float64
	KMax                   float64
	KMin                   float64
	KDefault               float64
	CalibrationGames       int
	VeteranGames           int
	StreakThreshold        int
	StreakMultiplierMax    float64
	DisplayConvergenceRate float64
}

// DefaultConfig returns a Config populated with the package-level constants.
func DefaultConfig() Config {
	return Config{
		DefaultMMR:             DefaultMMR,
		EloScale:               EloScale,
		KMax:                   KMax,
		KMin:                   KMin,
		KDefault:               KDefault,
		CalibrationGames:       CalibrationGames,
		VeteranGames:           VeteranGames,
		StreakThreshold:        StreakThreshold,
		StreakMultiplierMax:    StreakMultiplierMax,
		DisplayConvergenceRate: DisplayConvergenceRate,
	}
}
