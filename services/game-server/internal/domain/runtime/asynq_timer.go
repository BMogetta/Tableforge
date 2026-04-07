package runtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/recess/game-server/internal/platform/store"
)

const (
	TypeTurnTimeout  = "timer:turn"
	TypeReadyTimeout = "timer:ready"

	// Queue name must match the Queues map in the asynq.Server config.
	defaultQueue = "game"
)

func turnTaskID(sessionID uuid.UUID) string {
	return "timer:session:" + sessionID.String()
}

func readyTaskID(sessionID uuid.UUID) string {
	return "timer:ready:" + sessionID.String()
}

type timerPayload struct {
	SessionID string `json:"session_id"`
}

// AsynqTimer implements Timer using Asynq delayed tasks.
type AsynqTimer struct {
	client    *asynq.Client
	inspector *asynq.Inspector
}

func NewAsynqTimer(client *asynq.Client, inspector *asynq.Inspector) *AsynqTimer {
	return &AsynqTimer{client: client, inspector: inspector}
}

func (t *AsynqTimer) Schedule(session store.GameSession) {
	if session.TurnTimeoutSecs == nil || *session.TurnTimeoutSecs <= 0 {
		return
	}
	dur := time.Duration(*session.TurnTimeoutSecs) * time.Second
	t.enqueue(TypeTurnTimeout, turnTaskID(session.ID), session.ID, dur)
}

func (t *AsynqTimer) ScheduleIn(sessionID uuid.UUID, delay time.Duration) {
	t.enqueue(TypeTurnTimeout, turnTaskID(sessionID), sessionID, delay)
}

func (t *AsynqTimer) Cancel(sessionID uuid.UUID) {
	t.deleteTask(turnTaskID(sessionID))
}

func (t *AsynqTimer) ScheduleReady(sessionID uuid.UUID, timeout time.Duration) {
	t.enqueue(TypeReadyTimeout, readyTaskID(sessionID), sessionID, timeout)
}

func (t *AsynqTimer) CancelReady(sessionID uuid.UUID) {
	t.deleteTask(readyTaskID(sessionID))
}

func (t *AsynqTimer) enqueue(taskType, taskID string, sessionID uuid.UUID, delay time.Duration) {
	payload, err := json.Marshal(timerPayload{SessionID: sessionID.String()})
	if err != nil {
		slog.Error("asynq timer: marshal payload failed", "task_type", taskType, "error", err)
		return
	}

	// Delete existing task first to allow rescheduling with a new delay.
	t.deleteTask(taskID)

	task := asynq.NewTask(taskType, payload)
	_, err = t.client.Enqueue(task,
		asynq.TaskID(taskID),
		asynq.ProcessIn(delay),
		asynq.MaxRetry(3),
		asynq.Queue(defaultQueue),
	)
	if err != nil {
		slog.Error("asynq timer: enqueue failed", "task_type", taskType, "task_id", taskID, "error", err)
	}
}

func (t *AsynqTimer) deleteTask(taskID string) {
	if err := t.inspector.DeleteTask(defaultQueue, taskID); err != nil && err != asynq.ErrTaskNotFound {
		slog.Error("asynq timer: delete task failed", "task_id", taskID, "error", err)
	}
}

// ParseRedisOpt extracts addr and password from a redis:// URL for asynq.RedisClientOpt.
func ParseRedisOpt(rawURL string) (addr, password string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parse redis url: %w", err)
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "6379"
	}
	if u.User != nil {
		password, _ = u.User.Password()
	}
	return host + ":" + port, password, nil
}
