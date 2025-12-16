package main

// CalculateRiskScore calculates a risk score based on age and income.
// It returns a score between 0 and 100.
func CalculateRiskScore(age int, income int) int {
	if age < 18 {
		return 100 // High risk for minors
	}
	if income < 10000 {
		return 80 // High risk for low income
	}
	if age > 60 {
		return 40 // Medium risk for seniors
	}
	return 20 // Low risk
}
