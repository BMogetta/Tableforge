package featureflags

// FlagDevtoolsForAdmins is the Unleash flag that gates the admin devtools
// panel in production builds. The flag alone doesn't grant access — the
// caller must also be an owner.
const FlagDevtoolsForAdmins = "devtools-for-admins"

// roleOwner mirrors middleware.RoleOwner. Kept local to avoid an import
// cycle (middleware depends on featureflags.Checker). The middleware
// package is still the source of truth for the role enum; this string
// duplicates the value only.
const roleOwner = "owner"

// Capabilities is the server-calculated view the frontend reads via
// /auth/me/capabilities. It pairs the role (from the JWT) with live flag
// state and returns booleans the frontend can hand to lazy-loaders without
// duplicating any policy client-side.
type Capabilities struct {
	CanSeeDevtools bool `json:"canSeeDevtools"`
}

// Compute builds a Capabilities snapshot for a given role. Pass a nil
// Checker to get a zero Capabilities struct — useful in tests or if the
// SDK failed to init.
func Compute(flags Checker, role string) Capabilities {
	return Capabilities{
		CanSeeDevtools: canSeeDevtools(flags, role),
	}
}

func canSeeDevtools(flags Checker, role string) bool {
	if role != roleOwner {
		return false
	}
	if flags == nil {
		return false
	}
	return flags.IsEnabled(FlagDevtoolsForAdmins, false)
}
