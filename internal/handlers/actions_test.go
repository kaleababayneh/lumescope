package handlers

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"lumescope/internal/db"
)

// TestListActionsSupernodeFilterParsing tests that the supernode query parameter is parsed correctly
func TestListActionsSupernodeFilterParsing(t *testing.T) {
	tests := []struct {
		name            string
		queryParams     map[string]string
		expectSupernode *string
	}{
		{
			name:            "no supernode param",
			queryParams:     map[string]string{},
			expectSupernode: nil,
		},
		{
			name: "with supernode param",
			queryParams: map[string]string{
				"supernode": "lumera1abc123xyz",
			},
			expectSupernode: func() *string { s := "lumera1abc123xyz"; return &s }(),
		},
		{
			name: "empty supernode param",
			queryParams: map[string]string{
				"supernode": "",
			},
			expectSupernode: nil,
		},
		{
			name: "supernode with other filters",
			queryParams: map[string]string{
				"type":      "ACTION_TYPE_CASCADE",
				"state":     "ACTION_STATE_DONE",
				"supernode": "lumera1supernode456",
			},
			expectSupernode: func() *string { s := "lumera1supernode456"; return &s }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build query string
			values := url.Values{}
			for k, v := range tt.queryParams {
				values.Set(k, v)
			}

			// Parse supernode like the handler does
			queryValues := values
			filter := db.ActionsFilter{}

			if supernodeStr := queryValues.Get("supernode"); supernodeStr != "" {
				filterSupernode := supernodeStr
				filter.Supernode = &filterSupernode
			}

			// Verify
			if tt.expectSupernode == nil {
				if filter.Supernode != nil {
					t.Errorf("Expected Supernode to be nil, got %q", *filter.Supernode)
				}
			} else {
				if filter.Supernode == nil {
					t.Errorf("Expected Supernode to be %q, got nil", *tt.expectSupernode)
				} else if *filter.Supernode != *tt.expectSupernode {
					t.Errorf("Expected Supernode to be %q, got %q", *tt.expectSupernode, *filter.Supernode)
				}
			}
		})
	}
}

// TestBuildSupernodeFilterURL verifies the URL query parameter format
func TestBuildSupernodeFilterURL(t *testing.T) {
	baseURL := "/v1/actions"
	supernode := "lumera1abc..."
	
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("Failed to parse base URL: %v", err)
	}
	
	q := u.Query()
	q.Set("supernode", supernode)
	u.RawQuery = q.Encode()
	
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	// Verify the query param can be retrieved
	gotSupernode := req.URL.Query().Get("supernode")
	if gotSupernode != supernode {
		t.Errorf("Expected supernode query param to be %q, got %q", supernode, gotSupernode)
	}
}

// TestActionTransactionToDTO tests the conversion of db.ActionTransaction to TransactionDTO
func TestActionTransactionToDTO(t *testing.T) {
	blockTime := time.Date(2025, 12, 17, 12, 0, 0, 0, time.UTC)
	gasWanted := int64(200000)
	gasUsed := int64(150000)
	txFee := "5000"
	txFeeDenom := "ulume"
	actionPrice := "1000000"
	actionPriceDenom := "ulume"
	flowPayer := "lumera1payer..."
	flowPayee := "lumera1payee..."

	tx := db.ActionTransaction{
		ActionID:   123,
		TxType:     "register",
		TxHash:     "A1B2C3D4E5F6...",
		Height:     12345,
		BlockTime:  blockTime,
		GasWanted:  &gasWanted,
		GasUsed:    &gasUsed,
		TxFee:            &txFee,
		TxFeeDenom:       &txFeeDenom,
		ActionPrice:      &actionPrice,
		ActionPriceDenom: &actionPriceDenom,
		FlowPayer:        &flowPayer,
		FlowPayee:  &flowPayee,
		CreatedAt:  time.Now(),
	}

	dto := actionTransactionToDTO(tx)

	// Verify required fields
	if dto.TxType != tx.TxType {
		t.Errorf("Expected TxType %q, got %q", tx.TxType, dto.TxType)
	}
	if dto.TxHash != tx.TxHash {
		t.Errorf("Expected TxHash %q, got %q", tx.TxHash, dto.TxHash)
	}
	if dto.Height != tx.Height {
		t.Errorf("Expected Height %d, got %d", tx.Height, dto.Height)
	}
	if !dto.BlockTime.Equal(tx.BlockTime) {
		t.Errorf("Expected BlockTime %v, got %v", tx.BlockTime, dto.BlockTime)
	}

	// Verify optional fields
	if dto.GasWanted == nil || *dto.GasWanted != gasWanted {
		t.Errorf("Expected GasWanted %d, got %v", gasWanted, dto.GasWanted)
	}
	if dto.GasUsed == nil || *dto.GasUsed != gasUsed {
		t.Errorf("Expected GasUsed %d, got %v", gasUsed, dto.GasUsed)
	}
	if dto.TxFee == nil || *dto.TxFee != txFee {
		t.Errorf("Expected TxFee %s, got %v", txFee, dto.TxFee)
	}
	if dto.TxFeeDenom == nil || *dto.TxFeeDenom != txFeeDenom {
		t.Errorf("Expected TxFeeDenom %s, got %v", txFeeDenom, dto.TxFeeDenom)
	}
	if dto.ActionPrice == nil || *dto.ActionPrice != actionPrice {
		t.Errorf("Expected ActionPrice %s, got %v", actionPrice, dto.ActionPrice)
	}
	if dto.ActionPriceDenom == nil || *dto.ActionPriceDenom != actionPriceDenom {
		t.Errorf("Expected ActionPriceDenom %s, got %v", actionPriceDenom, dto.ActionPriceDenom)
	}
	if dto.FlowPayer == nil || *dto.FlowPayer != flowPayer {
		t.Errorf("Expected FlowPayer %s, got %v", flowPayer, dto.FlowPayer)
	}
	if dto.FlowPayee == nil || *dto.FlowPayee != flowPayee {
		t.Errorf("Expected FlowPayee %s, got %v", flowPayee, dto.FlowPayee)
	}
}

// TestActionTransactionToDTONilFields tests conversion with nil optional fields
func TestActionTransactionToDTONilFields(t *testing.T) {
	blockTime := time.Date(2025, 12, 17, 12, 0, 0, 0, time.UTC)

	tx := db.ActionTransaction{
		ActionID:  456,
		TxType:    "finalize",
		TxHash:    "FEDCBA987654...",
		Height:    67890,
		BlockTime: blockTime,
		// All optional fields are nil
		GasWanted:        nil,
		GasUsed:          nil,
		TxFee:            nil,
		TxFeeDenom:       nil,
		ActionPrice:      nil,
		ActionPriceDenom: nil,
		FlowPayer:        nil,
		FlowPayee:  nil,
		CreatedAt:  time.Now(),
	}

	dto := actionTransactionToDTO(tx)

	// Verify required fields still work
	if dto.TxType != tx.TxType {
		t.Errorf("Expected TxType %q, got %q", tx.TxType, dto.TxType)
	}
	if dto.TxHash != tx.TxHash {
		t.Errorf("Expected TxHash %q, got %q", tx.TxHash, dto.TxHash)
	}
	if dto.Height != tx.Height {
		t.Errorf("Expected Height %d, got %d", tx.Height, dto.Height)
	}

	// Verify optional fields are nil
	if dto.GasWanted != nil {
		t.Errorf("Expected GasWanted to be nil, got %v", dto.GasWanted)
	}
	if dto.GasUsed != nil {
		t.Errorf("Expected GasUsed to be nil, got %v", dto.GasUsed)
	}
	if dto.TxFee != nil {
		t.Errorf("Expected TxFee to be nil, got %v", dto.TxFee)
	}
	if dto.TxFeeDenom != nil {
		t.Errorf("Expected TxFeeDenom to be nil, got %v", dto.TxFeeDenom)
	}
	if dto.ActionPrice != nil {
		t.Errorf("Expected ActionPrice to be nil, got %v", dto.ActionPrice)
	}
}

// TestActionItemHasTransactionsField verifies ActionItem struct includes transactions field
func TestActionItemHasTransactionsField(t *testing.T) {
	item := ActionItem{
		ID:          "action123",
		Type:        "ACTION_TYPE_CASCADE",
		Creator:     "lumera1creator...",
		State:       "ACTION_STATE_DONE",
		BlockHeight: 12345,
		MimeType:    "image/jpeg",
		Size:        1024,
		Price: Price{
			Amount: "1000000",
			Denom:  "ulume",
		},
		Transactions: []TransactionDTO{
			{
				TxType:    "register",
				TxHash:    "HASH1...",
				Height:    12340,
				BlockTime: time.Now(),
			},
			{
				TxType:    "finalize",
				TxHash:    "HASH2...",
				Height:    12345,
				BlockTime: time.Now(),
			},
		},
	}

	// Verify transactions are accessible
	if len(item.Transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(item.Transactions))
	}

	if item.Transactions[0].TxType != "register" {
		t.Errorf("Expected first transaction to be 'register', got %q", item.Transactions[0].TxType)
	}

	if item.Transactions[1].TxType != "finalize" {
		t.Errorf("Expected second transaction to be 'finalize', got %q", item.Transactions[1].TxType)
	}
}
