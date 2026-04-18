package config

// UnleashConfig is the set of env-derived values every service needs to bring
// up a Unleash SDK client. Keep this alongside the other tiny helpers in
// shared/config so services don't each reinvent the loader.
type UnleashConfig struct {
	URL         string
	Token       string
	AppName     string
	Environment string
}

// LoadUnleash reads UNLEASH_URL, UNLEASH_API_TOKEN and UNLEASH_ENV from the
// environment (falling back to compose-friendly defaults) and pairs them with
// the caller-supplied appName, which is usually the service name (e.g.
// "game-server"). AppName shows up in the Unleash UI as the client identity.
func LoadUnleash(appName string) UnleashConfig {
	return UnleashConfig{
		URL:         Env("UNLEASH_URL", "http://unleash:4242/api"),
		Token:       Env("UNLEASH_API_TOKEN", "*:*.unleash-insecure-api-token"),
		AppName:     appName,
		Environment: Env("UNLEASH_ENV", "development"),
	}
}
