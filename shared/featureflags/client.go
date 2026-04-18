// Package featureflags is the shared wrapper around github.com/Unleash/unleash-client-go/v4.
// Every service initializes one of these at startup and passes it into handlers
// and middleware. The wrapper adds:
//
//   - A per-call fallback value: callers always pass a default so a missing flag
//     or a cold start returns a known answer instead of false.
//   - Nil-safety: a nil *Client answers with the caller-supplied default.
//   - A slog-backed listener so the SDK doesn't silently drop errors/metrics.
package featureflags

import (
	"log/slog"
	"net/http"

	"github.com/Unleash/unleash-client-go/v4"
	"github.com/recess/shared/config"
)

// Checker is the minimal interface handlers/middleware should depend on.
// Easier to mock than the full *Client in tests.
type Checker interface {
	IsEnabled(name string, defaultValue bool) bool
}

// Client wraps *unleash.Client. Keep this exported so main.go can defer Close.
type Client struct {
	inner *unleash.Client
}

var _ Checker = (*Client)(nil)

// Init creates an Unleash SDK client with shared defaults (slog listener,
// refresh interval provided by the SDK default of 15s). Returns a non-nil
// Client even if the initial fetch hasn't completed — IsEnabled will fall
// back to defaultValue until the first sync lands.
func Init(cfg config.UnleashConfig) (*Client, error) {
	c, err := unleash.NewClient(
		unleash.WithAppName(cfg.AppName),
		unleash.WithUrl(cfg.URL),
		unleash.WithEnvironment(cfg.Environment),
		unleash.WithCustomHeaders(http.Header{
			"Authorization": []string{cfg.Token},
		}),
		unleash.WithListener(&slogListener{}),
	)
	if err != nil {
		return nil, err
	}
	return &Client{inner: c}, nil
}

// IsEnabled answers with defaultValue when the client is nil (init failed) or
// when the flag is unknown server-side.
func (c *Client) IsEnabled(name string, defaultValue bool) bool {
	if c == nil || c.inner == nil {
		return defaultValue
	}
	return c.inner.IsEnabled(name, unleash.WithFallback(defaultValue))
}

// Close releases background goroutines. Safe to call on a nil *Client.
func (c *Client) Close() error {
	if c == nil || c.inner == nil {
		return nil
	}
	return c.inner.Close()
}

// slogListener implements unleash.ErrorListener + WarningListener + ReadyListener
// + MetricListener. Without a listener, the SDK drops these events silently and
// its internal channels block. We just forward them to slog at the right level.
type slogListener struct{}

func (l *slogListener) OnError(err error)     { slog.Warn("unleash: error", "err", err) }
func (l *slogListener) OnWarning(err error)   { slog.Debug("unleash: warning", "err", err) }
func (l *slogListener) OnReady()              { slog.Info("unleash: ready") }
func (l *slogListener) OnCount(string, bool)  {}
func (l *slogListener) OnSent(unleash.MetricsData) {}
func (l *slogListener) OnRegistered(unleash.ClientData) {}
func (l *slogListener) OnImpression(unleash.ImpressionEvent) {}
