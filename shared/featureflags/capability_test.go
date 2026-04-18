package featureflags

import "testing"

// Anti-drift note: capability.go duplicates the "owner" string constant
// instead of importing middleware.RoleOwner (circular import). The drift
// check would need its own package to avoid the same cycle, which is
// overkill for a 5-character string. If someone renames the role enum,
// all the Capability* handlers will start returning false for owners and
// the integration tests in Phase 3 catch it.

type stub struct {
	flag bool
}

func (s stub) IsEnabled(_ string, _ bool) bool { return s.flag }

func TestCompute_OwnerWithFlagOn_SeesDevtools(t *testing.T) {
	got := Compute(stub{flag: true}, "owner")
	if !got.CanSeeDevtools {
		t.Error("owner + flag ON should see devtools")
	}
}

func TestCompute_OwnerWithFlagOff_NoDevtools(t *testing.T) {
	got := Compute(stub{flag: false}, "owner")
	if got.CanSeeDevtools {
		t.Error("owner + flag OFF should not see devtools")
	}
}

func TestCompute_PlayerWithFlagOn_NoDevtools(t *testing.T) {
	got := Compute(stub{flag: true}, "player")
	if got.CanSeeDevtools {
		t.Error("player should never see devtools regardless of flag")
	}
}

func TestCompute_ManagerWithFlagOn_NoDevtools(t *testing.T) {
	got := Compute(stub{flag: true}, "manager")
	if got.CanSeeDevtools {
		t.Error("manager should never see devtools regardless of flag")
	}
}

func TestCompute_NilCheckerWithOwner_NoDevtools(t *testing.T) {
	got := Compute(nil, "owner")
	if got.CanSeeDevtools {
		t.Error("nil checker should default CanSeeDevtools to false even for owner")
	}
}

func TestCompute_EmptyRole_NoDevtools(t *testing.T) {
	got := Compute(stub{flag: true}, "")
	if got.CanSeeDevtools {
		t.Error("empty role should not see devtools")
	}
}
