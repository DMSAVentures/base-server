package tiers

import "strings"

// TierName represents the display tier name
type TierName string

const (
	TierFree TierName = "free"
	TierPro  TierName = "pro"
	TierTeam TierName = "team"
)

// PriceDescriptionToTier maps price descriptions to tier names
var PriceDescriptionToTier = map[string]TierName{
	"free":            TierFree,
	"lc_pro_monthly":  TierPro,
	"lc_pro_annual":   TierPro,
	"lc_team_monthly": TierTeam,
	"lc_team_annual":  TierTeam,
}

// TierDisplayNames maps tier names to display strings
var TierDisplayNames = map[TierName]string{
	TierFree: "Free",
	TierPro:  "Pro",
	TierTeam: "Team",
}

// GetTierForPriceDescription returns the tier name for a price description
func GetTierForPriceDescription(priceDescription string) TierName {
	if tier, ok := PriceDescriptionToTier[priceDescription]; ok {
		return tier
	}
	if tier, ok := PriceDescriptionToTier[strings.ToLower(priceDescription)]; ok {
		return tier
	}
	return TierFree
}

// GetTierDisplayName returns the display name for a tier
func GetTierDisplayName(tier TierName) string {
	if name, ok := TierDisplayNames[tier]; ok {
		return name
	}
	return "Free"
}
