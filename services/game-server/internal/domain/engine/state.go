package engine

import "encoding/json"

// DeepCopyData returns a JSON round-tripped copy of data so the caller can
// mutate the result without affecting the original. Plugins call this at the
// entry of ApplyMove to keep the reducer pure: same state + same move must
// produce the same new state without touching the caller's map.
//
// Falls back to returning the original map on marshal error. That path is
// unreachable in practice because GameState.Data is constrained to JSON-safe
// values by the engine contract, and the fallback only exists so deep-copy
// failure doesn't silently corrupt the reducer signature.
func DeepCopyData(data map[string]any) map[string]any {
	raw, err := json.Marshal(data)
	if err != nil {
		return data
	}
	var cp map[string]any
	if err := json.Unmarshal(raw, &cp); err != nil {
		return data
	}
	return cp
}
