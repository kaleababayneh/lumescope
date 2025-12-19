package db

import (
	"testing"
	"time"
)

// TestActionTransactionStruct verifies that the ActionTransaction struct contains all required fields
func TestActionTransactionStruct(t *testing.T) {
	// Test that all fields exist and can be set
	now := time.Now()
	gasWanted := int64(100000)
	gasUsed := int64(80000)
	actionPrice := "5000000"
	actionPriceDenom := "ulume"
	flowPayer := "lumera1payer..."
	flowPayee := "lumera1payee..."
	txFee := "500"
	txFeeDenom := "ulume"

	tx := ActionTransaction{
		ActionID:         123,
		TxType:           "register",
		TxHash:           "ABCDEF1234567890",
		Height:           1000,
		BlockTime:        now,
		GasWanted:        &gasWanted,
		GasUsed:          &gasUsed,
		ActionPrice:      &actionPrice,
		ActionPriceDenom: &actionPriceDenom,
		FlowPayer:        &flowPayer,
		FlowPayee:        &flowPayee,
		TxFee:            &txFee,
		TxFeeDenom:       &txFeeDenom,
		CreatedAt:        now,
	}

	// Verify all fields are set correctly
	if tx.ActionID != 123 {
		t.Errorf("Expected ActionID to be 123, got %d", tx.ActionID)
	}
	if tx.TxType != "register" {
		t.Errorf("Expected TxType to be 'register', got %q", tx.TxType)
	}
	if tx.TxHash != "ABCDEF1234567890" {
		t.Errorf("Expected TxHash to be 'ABCDEF1234567890', got %q", tx.TxHash)
	}
	if tx.Height != 1000 {
		t.Errorf("Expected Height to be 1000, got %d", tx.Height)
	}
	if tx.BlockTime != now {
		t.Errorf("Expected BlockTime to be %v, got %v", now, tx.BlockTime)
	}
	if *tx.GasWanted != gasWanted {
		t.Errorf("Expected GasWanted to be %d, got %d", gasWanted, *tx.GasWanted)
	}
	if *tx.GasUsed != gasUsed {
		t.Errorf("Expected GasUsed to be %d, got %d", gasUsed, *tx.GasUsed)
	}
	if *tx.ActionPrice != actionPrice {
		t.Errorf("Expected ActionPrice to be %q, got %q", actionPrice, *tx.ActionPrice)
	}
	if *tx.ActionPriceDenom != actionPriceDenom {
		t.Errorf("Expected ActionPriceDenom to be %q, got %q", actionPriceDenom, *tx.ActionPriceDenom)
	}
	if *tx.FlowPayer != flowPayer {
		t.Errorf("Expected FlowPayer to be %q, got %q", flowPayer, *tx.FlowPayer)
	}
	if *tx.FlowPayee != flowPayee {
		t.Errorf("Expected FlowPayee to be %q, got %q", flowPayee, *tx.FlowPayee)
	}
	if *tx.TxFee != txFee {
		t.Errorf("Expected TxFee to be %q, got %q", txFee, *tx.TxFee)
	}
	if *tx.TxFeeDenom != txFeeDenom {
		t.Errorf("Expected TxFeeDenom to be %q, got %q", txFeeDenom, *tx.TxFeeDenom)
	}
}

// TestActionTransactionTxTypes verifies that the expected transaction types are supported
func TestActionTransactionTxTypes(t *testing.T) {
	validTxTypes := []string{"register", "finalize", "approve"}

	for _, txType := range validTxTypes {
		tx := ActionTransaction{
			ActionID:  123,
			TxType:    txType,
			TxHash:    "HASH123",
			Height:    1000,
			BlockTime: time.Now(),
		}

		if tx.TxType != txType {
			t.Errorf("Expected TxType to be %q, got %q", txType, tx.TxType)
		}
	}
}

// TestActionTransactionNilOptionalFields verifies optional fields can be nil
func TestActionTransactionNilOptionalFields(t *testing.T) {
	tx := ActionTransaction{
		ActionID:  123,
		TxType:    "register",
		TxHash:    "HASH123",
		Height:    1000,
		BlockTime: time.Now(),
		// All optional fields left as nil
	}

	// Verify all optional fields are nil by default
	if tx.GasWanted != nil {
		t.Error("Expected GasWanted to be nil by default")
	}
	if tx.GasUsed != nil {
		t.Error("Expected GasUsed to be nil by default")
	}
	if tx.ActionPrice != nil {
		t.Error("Expected ActionPrice to be nil by default")
	}
	if tx.ActionPriceDenom != nil {
		t.Error("Expected ActionPriceDenom to be nil by default")
	}
	if tx.FlowPayer != nil {
		t.Error("Expected FlowPayer to be nil by default")
	}
	if tx.FlowPayee != nil {
		t.Error("Expected FlowPayee to be nil by default")
	}
	if tx.TxFee != nil {
		t.Error("Expected TxFee to be nil by default")
	}
	if tx.TxFeeDenom != nil {
		t.Error("Expected TxFeeDenom to be nil by default")
	}
}

// TestActionTransactionMultipleForSameAction verifies multiple transactions can be associated with the same action
func TestActionTransactionMultipleForSameAction(t *testing.T) {
	var actionID uint64 = 123
	now := time.Now()

	registerTx := ActionTransaction{
		ActionID:  actionID,
		TxType:    "register",
		TxHash:    "HASH_REGISTER",
		Height:    1000,
		BlockTime: now,
	}

	finalizeTx := ActionTransaction{
		ActionID:  actionID,
		TxType:    "finalize",
		TxHash:    "HASH_FINALIZE",
		Height:    1100,
		BlockTime: now.Add(time.Hour),
	}

	approveTx := ActionTransaction{
		ActionID:  actionID,
		TxType:    "approve",
		TxHash:    "HASH_APPROVE",
		Height:    1200,
		BlockTime: now.Add(2 * time.Hour),
	}

	// Verify all transactions share the same actionID but have different types
	transactions := []ActionTransaction{registerTx, finalizeTx, approveTx}

	for _, tx := range transactions {
		if tx.ActionID != actionID {
			t.Errorf("Expected ActionID to be %d, got %d", actionID, tx.ActionID)
		}
	}

	// Verify all txTypes are unique
	txTypes := make(map[string]bool)
	for _, tx := range transactions {
		if txTypes[tx.TxType] {
			t.Errorf("Duplicate TxType found: %q", tx.TxType)
		}
		txTypes[tx.TxType] = true
	}

	// Verify heights are in increasing order (register < finalize < approve)
	if registerTx.Height >= finalizeTx.Height {
		t.Error("Expected register height to be less than finalize height")
	}
	if finalizeTx.Height >= approveTx.Height {
		t.Error("Expected finalize height to be less than approve height")
	}
}
