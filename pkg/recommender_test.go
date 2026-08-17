package main

import (
	"testing"
)

func TestCalculateRecommendedCPU(t *testing.T) {
	tests := []struct {
		name     string
		usage    float64
		expected float64
	}{
		{
			name:     "zero usage",
			usage:    0,
			expected: 0.1,
		},
		{
			name:     "low usage",
			usage:    0.05,
			expected: 0.1,
		},
		{
			name:     "medium usage",
			usage:    0.5,
			expected: 0.75,
		},
		{
			name:     "high usage",
			usage:    0.8,
			expected: 1.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRecommendedCPU(tt.usage)
			if result != tt.expected {
				t.Errorf("CalculateRecommendedCPU(%f) = %f, want %f", tt.usage, result, tt.expected)
			}
		})
	}
}

func TestCalculateRecommendedMemory(t *testing.T) {
	tests := []struct {
		name     string
		usage    float64
		expected float64
	}{
		{
			name:     "zero usage",
			usage:    0,
			expected: 128,
		},
		{
			name:     "low usage",
			usage:    0.1,
			expected: 128,
		},
		{
			name:     "medium usage",
			usage:    0.5,
			expected: 512,
		},
		{
			name:     "high usage",
			usage:    0.8,
			expected: 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRecommendedMemory(tt.usage)
			if result != tt.expected {
				t.Errorf("CalculateRecommendedMemory(%f) = %f, want %f", tt.usage, result, tt.expected)
			}
		})
	}
}

func TestValidateLimits(t *testing.T) {
	tests := []struct {
		name      string
		cpu       float64
		memory    float64
		wantValid bool
	}{
		{
			name:      "valid limits",
			cpu:       1.0,
			memory:    512,
			wantValid: true,
		},
		{
			name:      "zero CPU",
			cpu:       0,
			memory:    512,
			wantValid: false,
		},
		{
			name:      "zero memory",
			cpu:       1.0,
			memory:    0,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateLimits(tt.cpu, tt.memory)
			if result != tt.wantValid {
				t.Errorf("ValidateLimits(%f, %f) = %v, want %v", tt.cpu, tt.memory, result, tt.wantValid)
			}
		})
	}
}

func CalculateRecommendedCPU(usage float64) float64 {
	if usage < 0.1 {
		return 0.1
	}
	return usage * 1.5
}

func CalculateRecommendedMemory(usage float64) float64 {
	if usage < 0.1 {
		return 128
	}
	return usage * 1024
}

func ValidateLimits(cpu, memory float64) bool {
	return cpu > 0 && memory > 0
}
