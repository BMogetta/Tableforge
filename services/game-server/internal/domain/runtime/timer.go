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
	Cancel(sessionID uuid.UUID)
	ScheduleReady(sessionID uuid.UUID, timeout time.Duration)
	CancelReady(sessionID uuid.UUID)
}
