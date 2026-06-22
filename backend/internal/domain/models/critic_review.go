package models

// CriticReview is the output of an adversarial review of a Recommendation.
// Verdict=="block" means the recommendation must not proceed to execution.
type CriticReview struct {
	Verdict   string   `json:"verdict"`   // "approve" | "block"
	Concerns  []string `json:"concerns"`  // specific risk findings; empty on approve
	RiskLevel string   `json:"risk_level"` // "low" | "medium" | "high"
	Reasoning string   `json:"reasoning"` // one sentence explaining the verdict
}
