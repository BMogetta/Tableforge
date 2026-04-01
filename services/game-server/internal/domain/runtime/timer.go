package runtime

import (
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/game-server/internal/platform/store"
)

// Timer abstracts turn and ready timer scheduling.
// Implementations must be safe for concurrent use.
type Timer interface {
	Schedule(session store.GameSession)
	ScheduleIn(sessionID uuid.UUID, delay time.Duration)
	Cancel(sessionID uuid.UUID)
	ScheduleReady(sessionID uuid.UUID, timeout time.Duration)
	CancelReady(sessionID uuid.UUID)
}

// ResumePenalty is the fraction of turn_timeout_secs given after a pause.
const ResumePenalty = 0.4
