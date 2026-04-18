package featureflags

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/recess/shared/config"
)

// fakeUnleashServer serves the three endpoints the SDK hits: register,
// features (with the caller-supplied payload), and metrics. Tests set the
// feature list per case.
func fakeUnleashServer(t *testing.T, features map[string]bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/client/register", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/client/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/client/features", func(w http.ResponseWriter, _ *http.Request) {
		featuresArr := make([]map[string]any, 0, len(features))
		for name, enabled := range features {
			featuresArr = append(featuresArr, map[string]any{
				"name":     name,
				"enabled":  enabled,
				"strategies": []map[string]any{{"name": "default"}},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":  1,
			"features": featuresArr,
		})
	})
	return httptest.NewServer(mux)
}

func testConfig(url string) config.UnleashConfig {
	return config.UnleashConfig{
		URL:         url,
		Token:       "test-token",
		AppName:     "featureflags-test",
		Environment: "development",
	}
}

// waitForFlag polls the client until it reports the expected value or until
// the deadline passes. SDK refreshes happen async, hence the poll instead of
// an assertEventually helper.
func waitForFlag(c *Client, name string, want bool) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.IsEnabled(name, !want) == want {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func TestInit_FlagOn_ReturnsTrue(t *testing.T) {
	srv := fakeUnleashServer(t, map[string]bool{"lit": true})
	defer srv.Close()

	c, err := Init(testConfig(srv.URL))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer c.Close()

	if !waitForFlag(c, "lit", true) {
		t.Fatal("flag 'lit' should have become true")
	}
}

func TestInit_FlagOff_ReturnsFalse(t *testing.T) {
	srv := fakeUnleashServer(t, map[string]bool{"dark": false})
	defer srv.Close()

	c, err := Init(testConfig(srv.URL))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer c.Close()

	if !waitForFlag(c, "dark", false) {
		t.Fatal("flag 'dark' should have become false")
	}
}

func TestIsEnabled_UnknownFlag_UsesDefault(t *testing.T) {
	srv := fakeUnleashServer(t, map[string]bool{"lit": true})
	defer srv.Close()

	c, err := Init(testConfig(srv.URL))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer c.Close()

	// Wait a beat so the initial fetch completes, then ask for a flag we
	// never registered server-side — the wrapper should fall through to the
	// supplied default.
	time.Sleep(200 * time.Millisecond)

	if got := c.IsEnabled("never-registered", true); !got {
		t.Error("default=true should survive an unknown flag lookup")
	}
	if got := c.IsEnabled("never-registered", false); got {
		t.Error("default=false should survive an unknown flag lookup")
	}
}

func TestIsEnabled_NilClient_UsesDefault(t *testing.T) {
	var c *Client // init failed, nil instance

	if got := c.IsEnabled("anything", true); !got {
		t.Error("nil client: default=true should be returned")
	}
	if got := c.IsEnabled("anything", false); got {
		t.Error("nil client: default=false should be returned")
	}
}

func TestClose_NilClient_NoPanic(t *testing.T) {
	var c *Client
	if err := c.Close(); err != nil {
		t.Errorf("nil close should be a no-op, got %v", err)
	}
}

// Assert Client satisfies the Checker interface at compile time. Doesn't
// ship runtime behavior; here for quick regression detection if someone
// changes Checker's shape.
var _ Checker = (*Client)(nil)
