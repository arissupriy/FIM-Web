package repository

import (
	"testing"
)

func TestFIMEventFilters(t *testing.T) {
	// Test defaults
	filters := NewFIMEventFilters()
	if filters.Page != 1 {
		t.Errorf("Default Page should be 1, got %d", filters.Page)
	}
	if filters.Limit != 50 {
		t.Errorf("Default Limit should be 50, got %d", filters.Limit)
	}
}

func TestFIMEventFiltersValidate(t *testing.T) {
	tests := []struct {
		name          string
		page          int
		limit         int
		expectedPage  int
		expectedLimit int
	}{
		{
			name:          "negative page normalized",
			page:          -1,
			limit:         50,
			expectedPage:  1,
			expectedLimit: 50,
		},
		{
			name:          "limit capped at 100",
			page:          1,
			limit:         200,
			expectedPage:  1,
			expectedLimit: 100,
		},
		{
			name:          "zero limit normalized",
			page:          1,
			limit:         0,
			expectedPage:  1,
			expectedLimit: 50, // default
		},
		{
			name:          "valid values unchanged",
			page:          5,
			limit:         25,
			expectedPage:  5,
			expectedLimit: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FIMEventFilters{Page: tt.page, Limit: tt.limit}
			f.Validate()
			if f.Page != tt.expectedPage {
				t.Errorf("Page = %d, want %d", f.Page, tt.expectedPage)
			}
			if f.Limit != tt.expectedLimit {
				t.Errorf("Limit = %d, want %d", f.Limit, tt.expectedLimit)
			}
		})
	}
}

func TestFIMEventFiltersOffset(t *testing.T) {
	tests := []struct {
		name           string
		page           int
		limit          int
		expectedOffset int
	}{
		{
			name:           "page 1",
			page:           1,
			limit:          50,
			expectedOffset: 0,
		},
		{
			name:           "page 2",
			page:           2,
			limit:          50,
			expectedOffset: 50,
		},
		{
			name:           "page 3 with limit 25",
			page:           3,
			limit:          25,
			expectedOffset: 50,
		},
		{
			name:           "page 10",
			page:           10,
			limit:          50,
			expectedOffset: 450,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FIMEventFilters{Page: tt.page, Limit: tt.limit}
			offset := f.Offset()
			if offset != tt.expectedOffset {
				t.Errorf("Offset() = %d, want %d", offset, tt.expectedOffset)
			}
		})
	}
}
