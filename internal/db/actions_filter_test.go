package db

import (
	"testing"
)

// TestActionsFilterStruct verifies that the ActionsFilter struct contains the Supernode field
func TestActionsFilterStruct(t *testing.T) {
	// Test that the Supernode field exists and can be set
	filter := ActionsFilter{}
	
	// Test nil case (no filter)
	if filter.Supernode != nil {
		t.Error("Expected Supernode to be nil by default")
	}
	
	// Test setting the supernode filter
	testSupernode := "lumera1abc123xyz"
	filter.Supernode = &testSupernode
	
	if filter.Supernode == nil {
		t.Error("Expected Supernode to be set")
	}
	
	if *filter.Supernode != testSupernode {
		t.Errorf("Expected Supernode to be %q, got %q", testSupernode, *filter.Supernode)
	}
}

// TestActionsFilterWithAllFields verifies all filter fields can be set together
func TestActionsFilterWithAllFields(t *testing.T) {
	filterType := "ACTION_TYPE_CASCADE"
	creator := "lumera1creator..."
	state := "ACTION_STATE_DONE"
	supernode := "lumera1supernode..."
	var fromHeight int64 = 1000
	var toHeight int64 = 2000
	
	filter := ActionsFilter{
		Type:       &filterType,
		Creator:    &creator,
		State:      &state,
		Supernode:  &supernode,
		FromHeight: &fromHeight,
		ToHeight:   &toHeight,
		Limit:      50,
	}
	
	// Verify all fields are set correctly
	if *filter.Type != filterType {
		t.Errorf("Expected Type to be %q, got %q", filterType, *filter.Type)
	}
	if *filter.Creator != creator {
		t.Errorf("Expected Creator to be %q, got %q", creator, *filter.Creator)
	}
	if *filter.State != state {
		t.Errorf("Expected State to be %q, got %q", state, *filter.State)
	}
	if *filter.Supernode != supernode {
		t.Errorf("Expected Supernode to be %q, got %q", supernode, *filter.Supernode)
	}
	if *filter.FromHeight != fromHeight {
		t.Errorf("Expected FromHeight to be %d, got %d", fromHeight, *filter.FromHeight)
	}
	if *filter.ToHeight != toHeight {
		t.Errorf("Expected ToHeight to be %d, got %d", toHeight, *filter.ToHeight)
	}
	if filter.Limit != 50 {
		t.Errorf("Expected Limit to be 50, got %d", filter.Limit)
	}
}
