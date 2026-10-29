package materiality

import "encoding/json"

// PolicySeed returns version, canonical hash, and JSON for upserting materiality_policy_versions.
// Migration seeds a placeholder hash; runtime should prefer this authoritative row.
func PolicySeed() (version, hash string, policyJSON []byte, err error) {
	p := CurrentPolicy()
	b, err := json.Marshal(p)
	if err != nil {
		return "", "", nil, err
	}
	return p.Version, p.Hash(), b, nil
}

// BudgetSeed returns version, canonical hash, and JSON for upserting materiality_budget_profiles.
func BudgetSeed() (version, hash string, budgetJSON []byte, err error) {
	b := DiscoveryStandardBudget()
	raw, err := json.Marshal(b)
	if err != nil {
		return "", "", nil, err
	}
	return b.Version, b.Hash(), raw, nil
}
