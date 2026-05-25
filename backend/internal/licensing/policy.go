package licensing

import "encoding/json"

type Entitlement struct {
	Edition                      Tier
	SourceStatus                 Status
	MaxConnectedTelegramAccounts int
	MaxWorkspaces                int
}

func EffectiveEntitlement(state State) Entitlement {
	if !isPaidStatus(state.Status) || (state.Tier != TierPro && state.Tier != TierTeam) {
		return communityEntitlement(state.Status)
	}

	limits := LicenseLimits{}
	if len(state.Limits) > 0 {
		if err := json.Unmarshal(state.Limits, &limits); err != nil {
			return communityEntitlement(state.Status)
		}
	}

	return Entitlement{
		Edition:                      state.Tier,
		SourceStatus:                 state.Status,
		MaxConnectedTelegramAccounts: clampPositive(limits.ConnectedTelegramAccounts, 1),
		MaxWorkspaces:                clampPositive(limits.Workspaces, 1),
	}
}

func isPaidStatus(status Status) bool {
	return status == StatusValid || status == StatusGrace
}

func communityEntitlement(status Status) Entitlement {
	return Entitlement{
		Edition:                      TierCommunity,
		SourceStatus:                 status,
		MaxConnectedTelegramAccounts: 1,
		MaxWorkspaces:                1,
	}
}

func clampPositive(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
