package deep

const qualityRubric = `A. TEMPORAL contradiction: sales decline before price increase — must not claim price caused decline.
B. Counterevidence: ads stable + stock zero vs ads deterioration thread — reframe or reject.
C. Search lag PARTIAL 2d — no overconfident explanation.
D. Positive growth + sufficient stock — positive Finding valid; no forced Action.
E. Insufficient/contradictory evidence — INCONCLUSIVE valid.
F. Multi-SKU contribution not in Thread — EVIDENCE_EMERGENT valid.
G. Strong demand + low stock + no inbound — ACTIONABLE_NOW replenishment may be proposed if menu allows.
H. Search insufficient — MONITOR; no Actions for that recommendation.`

func QualityRubric() string { return qualityRubric }
