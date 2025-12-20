package db

import (
	"testing"
	"time"
)

// TestActionStatsFilterStruct verifies that the ActionStatsFilter struct contains all expected fields
func TestActionStatsFilterStruct(t *testing.T) {
	// Test that the filter can be created with all fields nil
	filter := ActionStatsFilter{}

	if filter.ActionType != nil {
		t.Error("Expected ActionType to be nil by default")
	}
	if filter.From != nil {
		t.Error("Expected From to be nil by default")
	}
	if filter.To != nil {
		t.Error("Expected To to be nil by default")
	}
}

// TestActionStatsFilterWithTimeRange verifies that From and To can be set correctly
func TestActionStatsFilterWithTimeRange(t *testing.T) {
	actionType := "ACTION_TYPE_CASCADE"
	fromTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	toTime := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)

	filter := ActionStatsFilter{
		ActionType: &actionType,
		From:       &fromTime,
		To:         &toTime,
	}

	if filter.ActionType == nil || *filter.ActionType != actionType {
		t.Errorf("Expected ActionType to be %q, got %v", actionType, filter.ActionType)
	}
	if filter.From == nil || !filter.From.Equal(fromTime) {
		t.Errorf("Expected From to be %v, got %v", fromTime, filter.From)
	}
	if filter.To == nil || !filter.To.Equal(toTime) {
		t.Errorf("Expected To to be %v, got %v", toTime, filter.To)
	}
}

// TestActionStatsFilterOnlyFrom verifies that filter can have only From set
func TestActionStatsFilterOnlyFrom(t *testing.T) {
	fromTime := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	filter := ActionStatsFilter{
		From: &fromTime,
	}

	if filter.From == nil || !filter.From.Equal(fromTime) {
		t.Errorf("Expected From to be %v, got %v", fromTime, filter.From)
	}
	if filter.To != nil {
		t.Error("Expected To to be nil")
	}
	if filter.ActionType != nil {
		t.Error("Expected ActionType to be nil")
	}
}

// TestActionStatsFilterOnlyTo verifies that filter can have only To set
func TestActionStatsFilterOnlyTo(t *testing.T) {
	toTime := time.Date(2024, 6, 30, 23, 59, 59, 0, time.UTC)

	filter := ActionStatsFilter{
		To: &toTime,
	}

	if filter.To == nil || !filter.To.Equal(toTime) {
		t.Errorf("Expected To to be %v, got %v", toTime, filter.To)
	}
	if filter.From != nil {
		t.Error("Expected From to be nil")
	}
	if filter.ActionType != nil {
		t.Error("Expected ActionType to be nil")
	}
}

// TestActionStatsFilterOnlyActionType verifies that filter can have only ActionType set (no join needed)
func TestActionStatsFilterOnlyActionType(t *testing.T) {
	actionType := "ACTION_TYPE_SENSE"

	filter := ActionStatsFilter{
		ActionType: &actionType,
	}

	if filter.ActionType == nil || *filter.ActionType != actionType {
		t.Errorf("Expected ActionType to be %q, got %v", actionType, filter.ActionType)
	}
	if filter.From != nil {
		t.Error("Expected From to be nil")
	}
	if filter.To != nil {
		t.Error("Expected To to be nil")
	}
}

// TestActionStatsExtendedStruct verifies the response struct
func TestActionStatsExtendedStruct(t *testing.T) {
	stats := ActionStatsExtended{
		Total: 100,
		StateCounts: []StateCount{
			{State: "ACTION_STATE_PENDING", Count: 20},
			{State: "ACTION_STATE_DONE", Count: 50},
			{State: "ACTION_STATE_APPROVED", Count: 30},
		},
		MimeTypeStats: []MimeTypeStat{
			{MimeType: "image/jpeg", Count: 60, AvgSize: 1024.5},
			{MimeType: "application/pdf", Count: 40, AvgSize: 2048.0},
		},
	}

	if stats.Total != 100 {
		t.Errorf("Expected Total to be 100, got %d", stats.Total)
	}
	if len(stats.StateCounts) != 3 {
		t.Errorf("Expected 3 state counts, got %d", len(stats.StateCounts))
	}
	if len(stats.MimeTypeStats) != 2 {
		t.Errorf("Expected 2 MIME type stats, got %d", len(stats.MimeTypeStats))
	}

	// Verify state counts sum to total
	sum := 0
	for _, sc := range stats.StateCounts {
		sum += sc.Count
	}
	if sum != stats.Total {
		t.Errorf("Expected state counts sum (%d) to equal total (%d)", sum, stats.Total)
	}
}

// TestMimeTypeStatStruct verifies the MIME type stats structure
func TestMimeTypeStatStruct(t *testing.T) {
	stat := MimeTypeStat{
		MimeType: "video/mp4",
		Count:    25,
		AvgSize:  5000000.5,
	}

	if stat.MimeType != "video/mp4" {
		t.Errorf("Expected MimeType to be 'video/mp4', got %q", stat.MimeType)
	}
	if stat.Count != 25 {
		t.Errorf("Expected Count to be 25, got %d", stat.Count)
	}
	if stat.AvgSize != 5000000.5 {
		t.Errorf("Expected AvgSize to be 5000000.5, got %f", stat.AvgSize)
	}
}

// TestActionStatsFilterDateRangeJoinBehavior documents the expected JOIN behavior
// When From or To is set, the query should JOIN action_transactions to filter by blockTime
// When neither From nor To is set, the query should NOT use a JOIN
func TestActionStatsFilterDateRangeJoinBehavior(t *testing.T) {
	// Test case 1: No date filter - should not need JOIN
	noDateFilter := ActionStatsFilter{
		ActionType: func() *string { s := "ACTION_TYPE_CASCADE"; return &s }(),
	}
	needsJoinNoDate := noDateFilter.From != nil || noDateFilter.To != nil
	if needsJoinNoDate {
		t.Error("Expected no JOIN needed when From and To are nil")
	}

	// Test case 2: Only From set - should need JOIN
	fromTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	fromOnlyFilter := ActionStatsFilter{
		From: &fromTime,
	}
	needsJoinFromOnly := fromOnlyFilter.From != nil || fromOnlyFilter.To != nil
	if !needsJoinFromOnly {
		t.Error("Expected JOIN needed when From is set")
	}

	// Test case 3: Only To set - should need JOIN
	toTime := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
	toOnlyFilter := ActionStatsFilter{
		To: &toTime,
	}
	needsJoinToOnly := toOnlyFilter.From != nil || toOnlyFilter.To != nil
	if !needsJoinToOnly {
		t.Error("Expected JOIN needed when To is set")
	}

	// Test case 4: Both From and To set - should need JOIN
	bothFilter := ActionStatsFilter{
		From: &fromTime,
		To:   &toTime,
	}
	needsJoinBoth := bothFilter.From != nil || bothFilter.To != nil
	if !needsJoinBoth {
		t.Error("Expected JOIN needed when both From and To are set")
	}
}
