package achievements

// Positional i18n key builders. Callers use these to construct Definition
// and Tier key fields so the shape stays consistent across packages. See
// types.go for the key scheme documentation.

// NameKey returns the i18n key for an achievement's display name.
func NameKey(id string) string { return "achievements." + id + ".name" }

// DescriptionKey returns the i18n key for a flat achievement's description.
// Tiered achievements use per-tier descriptions instead — see TierDescriptionKey.
func DescriptionKey(id string) string { return "achievements." + id + ".description" }

// TierNameKey returns the i18n key for a specific tier's display name.
// Tier is 1-based.
func TierNameKey(id string, tier int) string {
	return "achievements." + id + ".tiers." + itoa(tier) + ".name"
}

// TierDescriptionKey returns the i18n key for a specific tier's description.
// Descriptions may use the {{threshold}} placeholder, which i18next
// interpolates with the tier's Threshold at render time.
func TierDescriptionKey(id string, tier int) string {
	return "achievements." + id + ".tiers." + itoa(tier) + ".description"
}

// itoa is an allocation-cheap integer-to-string for 0-99. Achievement tiers
// will never exceed this range in practice, and pulling in strconv for the
// init path would be overkill.
func itoa(n int) string {
	if n >= 0 && n <= 9 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
