package main

import "testing"

func TestCalculateRiskScore(t *testing.T) {
	tests := []struct {
		name   string
		age    int
		income int
		want   int
	}{
		{"minor", 17, 0, 100},
		{"low income", 25, 5000, 80},
		{"senior", 65, 50000, 40},
		{"standard", 30, 50000, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateRiskScore(tt.age, tt.income); got != tt.want {
				t.Errorf("CalculateRiskScore(%d, %d) = %d; want %d", tt.age, tt.income, got, tt.want)
			}
		})
	}
}
