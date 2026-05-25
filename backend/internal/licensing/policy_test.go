package licensing

import "testing"

func TestEffectiveEntitlementCommunityFallback(t *testing.T) {
	state := State{
		Status: StatusInvalid,
		Tier:   TierPro,
		Limits: []byte(`{"workspaces":10,"connected_telegram_accounts":10}`),
	}

	ent := EffectiveEntitlement(state)
	if ent.Edition != TierCommunity {
		t.Fatalf("edition = %q, want community", ent.Edition)
	}
	if ent.MaxConnectedTelegramAccounts != 1 {
		t.Fatalf("accounts = %d, want 1", ent.MaxConnectedTelegramAccounts)
	}
	if ent.MaxWorkspaces != 1 {
		t.Fatalf("workspaces = %d, want 1", ent.MaxWorkspaces)
	}
}

func TestEffectiveEntitlementProLimits(t *testing.T) {
	state := State{
		Status: StatusValid,
		Tier:   TierPro,
		Limits: []byte(`{"workspaces":4,"connected_telegram_accounts":5}`),
	}

	ent := EffectiveEntitlement(state)
	if ent.Edition != TierPro {
		t.Fatalf("edition = %q, want pro", ent.Edition)
	}
	if ent.MaxConnectedTelegramAccounts != 5 {
		t.Fatalf("accounts = %d, want 5", ent.MaxConnectedTelegramAccounts)
	}
	if ent.MaxWorkspaces != 4 {
		t.Fatalf("workspaces = %d, want 4", ent.MaxWorkspaces)
	}
}

func TestEffectiveEntitlementGraceStatusAllowsPaid(t *testing.T) {
	state := State{
		Status: StatusGrace,
		Tier:   TierTeam,
		Limits: []byte(`{"workspaces":3,"connected_telegram_accounts":7}`),
	}

	ent := EffectiveEntitlement(state)
	if ent.Edition != TierTeam {
		t.Fatalf("edition = %q, want team", ent.Edition)
	}
	if ent.MaxConnectedTelegramAccounts != 7 {
		t.Fatalf("accounts = %d, want 7", ent.MaxConnectedTelegramAccounts)
	}
}

func TestEffectiveEntitlementInvalidLimitsFallback(t *testing.T) {
	state := State{
		Status: StatusValid,
		Tier:   TierPro,
		Limits: []byte(`{invalid`),
	}

	ent := EffectiveEntitlement(state)
	if ent.Edition != TierCommunity {
		t.Fatalf("edition = %q, want community on invalid limits json", ent.Edition)
	}
}
