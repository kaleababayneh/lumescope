package lumera

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lumescope/internal/db"
)

// TestParseTransferEvent tests the parseTransferEvent function
func TestParseTransferEvent(t *testing.T) {
	tests := []struct {
		name     string
		attrs    []Attribute
		wantNil  bool
		wantFlow *TransferFlow
	}{
		{
			name: "valid transfer with all fields",
			attrs: []Attribute{
				{Key: "sender", Value: "lumera1sender123"},
				{Key: "recipient", Value: "lumera1recipient456"},
				{Key: "amount", Value: "10090ulume"},
			},
			wantNil: false,
			wantFlow: &TransferFlow{
				Payer:  strPtr("lumera1sender123"),
				Payee:  strPtr("lumera1recipient456"),
				Amount: strPtr("10090"),
				Denom:  strPtr("ulume"),
			},
		},
		{
			name: "transfer with only sender",
			attrs: []Attribute{
				{Key: "sender", Value: "lumera1sender123"},
			},
			wantNil: false,
			wantFlow: &TransferFlow{
				Payer: strPtr("lumera1sender123"),
			},
		},
		{
			name: "transfer with only recipient",
			attrs: []Attribute{
				{Key: "recipient", Value: "lumera1recipient456"},
			},
			wantNil: false,
			wantFlow: &TransferFlow{
				Payee: strPtr("lumera1recipient456"),
			},
		},
		{
			name:    "empty attributes",
			attrs:   []Attribute{},
			wantNil: true,
		},
		{
			name: "no sender or recipient",
			attrs: []Attribute{
				{Key: "amount", Value: "1000ulume"},
			},
			wantNil: true,
		},
		{
			name: "large amount",
			attrs: []Attribute{
				{Key: "sender", Value: "lumera1sender"},
				{Key: "recipient", Value: "lumera1recipient"},
				{Key: "amount", Value: "1000000000000ulume"},
			},
			wantNil: false,
			wantFlow: &TransferFlow{
				Payer:  strPtr("lumera1sender"),
				Payee:  strPtr("lumera1recipient"),
				Amount: strPtr("1000000000000"),
				Denom:  strPtr("ulume"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTransferEvent(tt.attrs)
			if tt.wantNil {
				if got != nil {
					t.Errorf("parseTransferEvent() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("parseTransferEvent() = nil, want non-nil")
			}
			if !strPtrEqual(got.Payer, tt.wantFlow.Payer) {
				t.Errorf("Payer = %v, want %v", strPtrVal(got.Payer), strPtrVal(tt.wantFlow.Payer))
			}
			if !strPtrEqual(got.Payee, tt.wantFlow.Payee) {
				t.Errorf("Payee = %v, want %v", strPtrVal(got.Payee), strPtrVal(tt.wantFlow.Payee))
			}
			if !strPtrEqual(got.Amount, tt.wantFlow.Amount) {
				t.Errorf("Amount = %v, want %v", strPtrVal(got.Amount), strPtrVal(tt.wantFlow.Amount))
			}
			if !strPtrEqual(got.Denom, tt.wantFlow.Denom) {
				t.Errorf("Denom = %v, want %v", strPtrVal(got.Denom), strPtrVal(tt.wantFlow.Denom))
			}
		})
	}
}

// TestExtractTransferFlow tests the extractTransferFlow function
func TestExtractTransferFlow(t *testing.T) {
	client := &Client{}
	moduleAddr := "lumera1module"

	tests := []struct {
		name       string
		action     *db.Action
		txType     string
		txResult   TxResult
		moduleAddr string
		txSigner   string
		wantNil    bool
		wantPayer  string
		wantPayee  string
	}{
		{
			name: "register - transfer to module address",
			action: &db.Action{
				ActionID: 123,
				Creator:  "lumera1creator",
			},
			txType: "register",
			txResult: TxResult{
				Events: []Event{
					{
						Type: "transfer",
						Attributes: []Attribute{
							{Key: "sender", Value: "lumera1creator"},
							{Key: "recipient", Value: "lumera1module"},
							{Key: "amount", Value: "10000ulume"},
						},
					},
				},
			},
			moduleAddr: moduleAddr,
			txSigner:   "lumera1creator",
			wantNil:    false,
			wantPayer:  "lumera1creator",
			wantPayee:  "lumera1module",
		},
		{
			name: "register - multiple transfers, pick transfer to module",
			action: &db.Action{
				ActionID: 123,
				Creator:  "lumera1creator",
			},
			txType: "register",
			txResult: TxResult{
				Events: []Event{
					{
						Type: "transfer",
						Attributes: []Attribute{
							{Key: "sender", Value: "lumera1other"},
							{Key: "recipient", Value: "lumera1recipient1"},
							{Key: "amount", Value: "5000ulume"},
						},
					},
					{
						Type: "transfer",
						Attributes: []Attribute{
							{Key: "sender", Value: "lumera1creator"},
							{Key: "recipient", Value: "lumera1module"},
							{Key: "amount", Value: "10000ulume"},
						},
					},
				},
			},
			moduleAddr: moduleAddr,
			txSigner:   "lumera1creator",
			wantNil:    false,
			wantPayer:  "lumera1creator",
			wantPayee:  "lumera1module",
		},
		{
			name: "register - fallback to creator as sender when no module addr",
			action: &db.Action{
				ActionID: 123,
				Creator:  "lumera1creator",
			},
			txType: "register",
			txResult: TxResult{
				Events: []Event{
					{
						Type: "transfer",
						Attributes: []Attribute{
							{Key: "sender", Value: "lumera1creator"},
							{Key: "recipient", Value: "lumera1unknown"},
							{Key: "amount", Value: "10000ulume"},
						},
					},
				},
			},
			moduleAddr: "", // No module address
			txSigner:   "lumera1creator",
			wantNil:    false,
			wantPayer:  "lumera1creator",
			wantPayee:  "lumera1unknown",
		},
		{
			name: "finalize - transfer from module to tx signer",
			action: &db.Action{
				ActionID:         123,
				Creator:          "lumera1creator",
				SupernodeAccount: "lumera1supernode",
			},
			txType: "finalize",
			txResult: TxResult{
				Events: []Event{
					{
						Type: "transfer",
						Attributes: []Attribute{
							{Key: "sender", Value: "lumera1module"},
							{Key: "recipient", Value: "lumera1supernode"},
							{Key: "amount", Value: "8000ulume"},
						},
					},
				},
			},
			moduleAddr: moduleAddr,
			txSigner:   "lumera1supernode", // tx signer is the supernode
			wantNil:    false,
			wantPayer:  "lumera1module",
			wantPayee:  "lumera1supernode",
		},
		{
			name: "finalize - fallback to supernode when tx signer not matching",
			action: &db.Action{
				ActionID:         123,
				Creator:          "lumera1creator",
				SupernodeAccount: "lumera1supernode",
			},
			txType: "finalize",
			txResult: TxResult{
				Events: []Event{
					{
						Type: "transfer",
						Attributes: []Attribute{
							{Key: "sender", Value: "lumera1module"},
							{Key: "recipient", Value: "lumera1supernode"},
							{Key: "amount", Value: "8000ulume"},
						},
					},
				},
			},
			moduleAddr: moduleAddr,
			txSigner:   "lumera1othersigner", // different signer
			wantNil:    false,
			wantPayer:  "lumera1module",
			wantPayee:  "lumera1supernode",
		},
		{
			name: "finalize - fallback to sender from module",
			action: &db.Action{
				ActionID: 123,
				Creator:  "lumera1creator",
			},
			txType: "finalize",
			txResult: TxResult{
				Events: []Event{
					{
						Type: "transfer",
						Attributes: []Attribute{
							{Key: "sender", Value: "lumera1module"},
							{Key: "recipient", Value: "lumera1creator"},
							{Key: "amount", Value: "5000ulume"},
						},
					},
				},
			},
			moduleAddr: moduleAddr,
			txSigner:   "",
			wantNil:    false,
			wantPayer:  "lumera1module",
			wantPayee:  "lumera1creator",
		},
		{
			name: "finalize - no module, pick non-creator sender",
			action: &db.Action{
				ActionID: 123,
				Creator:  "lumera1creator",
			},
			txType: "finalize",
			txResult: TxResult{
				Events: []Event{
					{
						Type: "transfer",
						Attributes: []Attribute{
							{Key: "sender", Value: "lumera1somemodule"},
							{Key: "recipient", Value: "lumera1creator"},
							{Key: "amount", Value: "5000ulume"},
						},
					},
				},
			},
			moduleAddr: "", // No module address known
			txSigner:   "",
			wantNil:    false,
			wantPayer:  "lumera1somemodule",
			wantPayee:  "lumera1creator",
		},
		{
			name: "no transfer events",
			action: &db.Action{
				ActionID: 123,
				Creator:  "lumera1creator",
			},
			txType: "register",
			txResult: TxResult{
				Events: []Event{
					{
						Type: "message",
						Attributes: []Attribute{
							{Key: "action", Value: "register_action"},
						},
					},
				},
			},
			moduleAddr: moduleAddr,
			txSigner:   "lumera1creator",
			wantNil:    true,
		},
		{
			name: "transfer events in logs",
			action: &db.Action{
				ActionID: 123,
				Creator:  "lumera1creator",
			},
			txType: "register",
			txResult: TxResult{
				Events: []Event{},
				Logs: []ABCILog{
					{
						MsgIndex: 0,
						Events: []Event{
							{
								Type: "transfer",
								Attributes: []Attribute{
									{Key: "sender", Value: "lumera1creator"},
									{Key: "recipient", Value: "lumera1module"},
									{Key: "amount", Value: "10000ulume"},
								},
							},
						},
					},
				},
			},
			moduleAddr: moduleAddr,
			txSigner:   "lumera1creator",
			wantNil:    false,
			wantPayer:  "lumera1creator",
			wantPayee:  "lumera1module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.extractTransferFlow(tt.action, tt.txType, tt.txResult, tt.moduleAddr, tt.txSigner)
			if tt.wantNil {
				if got != nil {
					t.Errorf("extractTransferFlow() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("extractTransferFlow() = nil, want non-nil")
			}
			if got.Payer != nil && *got.Payer != tt.wantPayer {
				t.Errorf("Payer = %v, want %v", *got.Payer, tt.wantPayer)
			}
			if got.Payee != nil && *got.Payee != tt.wantPayee {
				t.Errorf("Payee = %v, want %v", *got.Payee, tt.wantPayee)
			}
		})
	}
}

// TestExtractTxSigner tests the extractTxSigner function
func TestExtractTxSigner(t *testing.T) {
	tests := []struct {
		name     string
		tx       *TxResponse
		wantAddr string
	}{
		{
			name:     "nil tx",
			tx:       nil,
			wantAddr: "",
		},
		{
			name: "empty messages",
			tx: &TxResponse{
				Body: TxBody{
					Messages: []json.RawMessage{},
				},
			},
			wantAddr: "",
		},
		{
			name: "message with creator field",
			tx: &TxResponse{
				Body: TxBody{
					Messages: []json.RawMessage{
						json.RawMessage(`{"@type":"/lumera.action.MsgFinalizeAction","creator":"lumera1finalizer"}`),
					},
				},
			},
			wantAddr: "lumera1finalizer",
		},
		{
			name: "message with sender field",
			tx: &TxResponse{
				Body: TxBody{
					Messages: []json.RawMessage{
						json.RawMessage(`{"@type":"/cosmos.bank.v1beta1.MsgSend","sender":"lumera1sender"}`),
					},
				},
			},
			wantAddr: "lumera1sender",
		},
		{
			name: "message with from_address field",
			tx: &TxResponse{
				Body: TxBody{
					Messages: []json.RawMessage{
						json.RawMessage(`{"@type":"/ibc.transfer","from_address":"lumera1from"}`),
					},
				},
			},
			wantAddr: "lumera1from",
		},
		{
			name: "invalid json",
			tx: &TxResponse{
				Body: TxBody{
					Messages: []json.RawMessage{
						json.RawMessage(`{invalid json}`),
					},
				},
			},
			wantAddr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTxSigner(tt.tx)
			if got != tt.wantAddr {
				t.Errorf("extractTxSigner() = %v, want %v", got, tt.wantAddr)
			}
		})
	}
}

// TestParseTxResult tests the parseTxResult function
func TestParseTxResult(t *testing.T) {
	client := &Client{}
	moduleAddr := "lumera1module"

	action := &db.Action{
		ActionID: 123,
		Creator:  "lumera1creator",
	}

	txResult := TxResult{
		TxHash:    "ABCDEF123456",
		Height:    "12345",
		Timestamp: "2024-01-15T10:30:00Z",
		GasWanted: "200000",
		GasUsed:   "150000",
		Events: []Event{
			{
				Type: "transfer",
				Attributes: []Attribute{
					{Key: "sender", Value: "lumera1creator"},
					{Key: "recipient", Value: "lumera1module"},
					{Key: "amount", Value: "10000ulume"},
				},
			},
		},
	}

	tx := &TxResponse{
		Body: TxBody{
			Messages: []json.RawMessage{
				json.RawMessage(`{"@type":"/lumera.action.MsgRegisterAction","creator":"lumera1creator"}`),
			},
		},
		AuthInfo: AuthInfo{
			Fee: Fee{
				Amount: []Coin{
					{Denom: "ulume", Amount: "500"},
				},
			},
		},
	}

	got := client.parseTxResult(action, "register", txResult, tx, moduleAddr)

	if got == nil {
		t.Fatal("parseTxResult() = nil, want non-nil")
	}

	if got.ActionID != 123 {
		t.Errorf("ActionID = %v, want action123", got.ActionID)
	}
	if got.TxType != "register" {
		t.Errorf("TxType = %v, want register", got.TxType)
	}
	if got.TxHash != "ABCDEF123456" {
		t.Errorf("TxHash = %v, want ABCDEF123456", got.TxHash)
	}
	if got.Height != 12345 {
		t.Errorf("Height = %v, want 12345", got.Height)
	}
	if got.GasWanted == nil || *got.GasWanted != 200000 {
		t.Errorf("GasWanted = %v, want 200000", got.GasWanted)
	}
	if got.GasUsed == nil || *got.GasUsed != 150000 {
		t.Errorf("GasUsed = %v, want 150000", got.GasUsed)
	}
	if got.TxFee == nil || *got.TxFee != "500" {
		t.Errorf("TxFee = %v, want 500", got.TxFee)
	}
	if got.TxFeeDenom == nil || *got.TxFeeDenom != "ulume" {
		t.Errorf("TxFeeDenom = %v, want ulume", got.TxFeeDenom)
	}
	if got.ActionPrice == nil || *got.ActionPrice != "10000" {
		t.Errorf("ActionPrice = %v, want 10000", got.ActionPrice)
	}
	if got.ActionPriceDenom == nil || *got.ActionPriceDenom != "ulume" {
		t.Errorf("ActionPriceDenom = %v, want ulume", got.ActionPriceDenom)
	}
	if got.FlowPayer == nil || *got.FlowPayer != "lumera1creator" {
		t.Errorf("FlowPayer = %v, want lumera1creator", got.FlowPayer)
	}
	if got.FlowPayee == nil || *got.FlowPayee != "lumera1module" {
		t.Errorf("FlowPayee = %v, want lumera1module", got.FlowPayee)
	}
}

// TestGetActionModuleAccount tests the GetActionModuleAccount function with caching
func TestGetActionModuleAccount(t *testing.T) {
	moduleAddr := "lumera1moduleaccountaddr"
	callCount := 0

	// Create a mock server that returns module account info
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/cosmos/auth/v1beta1/module_accounts/action" {
			response := ModuleAccountResponse{
				Account: struct {
					Type        string `json:"@type"`
					BaseAccount struct {
						Address string `json:"address"`
					} `json:"base_account"`
					Name        string   `json:"name"`
					Permissions []string `json:"permissions"`
				}{
					Type: "/cosmos.auth.v1beta1.ModuleAccount",
					BaseAccount: struct {
						Address string `json:"address"`
					}{
						Address: moduleAddr,
					},
					Name:        "action",
					Permissions: []string{},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)

	// First call - should fetch from API
	addr1, err := client.GetActionModuleAccount(context.Background())
	if err != nil {
		t.Fatalf("GetActionModuleAccount() error = %v", err)
	}
	if addr1 != moduleAddr {
		t.Errorf("GetActionModuleAccount() = %v, want %v", addr1, moduleAddr)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call, got %d", callCount)
	}

	// Second call - should use cached value
	addr2, err := client.GetActionModuleAccount(context.Background())
	if err != nil {
		t.Fatalf("GetActionModuleAccount() second call error = %v", err)
	}
	if addr2 != moduleAddr {
		t.Errorf("GetActionModuleAccount() second call = %v, want %v", addr2, moduleAddr)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call (cached), got %d", callCount)
	}
}

// TestSetActionModuleAccount tests the SetActionModuleAccount function
func TestSetActionModuleAccount(t *testing.T) {
	client := NewClient("http://example.com", 5*time.Second)

	// Set module address manually
	client.SetActionModuleAccount("lumera1testmodule")

	// Get should return the set value without API call
	addr, err := client.GetActionModuleAccount(context.Background())
	if err != nil {
		t.Fatalf("GetActionModuleAccount() error = %v", err)
	}
	if addr != "lumera1testmodule" {
		t.Errorf("GetActionModuleAccount() = %v, want lumera1testmodule", addr)
	}
}

// TestGetActionTransactions tests the full GetActionTransactions flow with mocked HTTP
func TestGetActionTransactions(t *testing.T) {
	moduleAddr := "lumera1module"
	
	// Create a mock server that returns transaction search results and module account
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle module account query
		if r.URL.Path == "/cosmos/auth/v1beta1/module_accounts/action" {
			response := ModuleAccountResponse{
				Account: struct {
					Type        string `json:"@type"`
					BaseAccount struct {
						Address string `json:"address"`
					} `json:"base_account"`
					Name        string   `json:"name"`
					Permissions []string `json:"permissions"`
				}{
					Type: "/cosmos.auth.v1beta1.ModuleAccount",
					BaseAccount: struct {
						Address string `json:"address"`
					}{
						Address: moduleAddr,
					},
					Name:        "action",
					Permissions: []string{},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle tx search queries
		query := r.URL.Query().Get("query")
		
		var response TxSearchResponse
		
		if query == "action_registered.action_id=123" {
			response = TxSearchResponse{
				Txs: []TxResponse{
					{
						Body: TxBody{
							Messages: []json.RawMessage{
								json.RawMessage(`{"@type":"/lumera.action.MsgRegisterAction","creator":"lumera1creator"}`),
							},
						},
						AuthInfo: AuthInfo{
							Fee: Fee{
								Amount: []Coin{{Denom: "ulume", Amount: "1000"}},
							},
						},
					},
				},
				TxResponses: []TxResult{
					{
						TxHash:    "REGISTER_TX_HASH",
						Height:    "100",
						Timestamp: "2024-01-15T10:00:00Z",
						GasWanted: "200000",
						GasUsed:   "150000",
						Events: []Event{
							{
								Type: "transfer",
								Attributes: []Attribute{
									{Key: "sender", Value: "lumera1creator"},
									{Key: "recipient", Value: moduleAddr},
									{Key: "amount", Value: "10000ulume"},
								},
							},
						},
					},
				},
			}
		} else if query == "action_finalized.action_id=123" {
			response = TxSearchResponse{
				Txs: []TxResponse{
					{
						Body: TxBody{
							Messages: []json.RawMessage{
								json.RawMessage(`{"@type":"/lumera.action.MsgFinalizeAction","creator":"lumera1supernode"}`),
							},
						},
						AuthInfo: AuthInfo{
							Fee: Fee{
								Amount: []Coin{{Denom: "ulume", Amount: "500"}},
							},
						},
					},
				},
				TxResponses: []TxResult{
					{
						TxHash:    "FINALIZE_TX_HASH",
						Height:    "200",
						Timestamp: "2024-01-15T11:00:00Z",
						GasWanted: "100000",
						GasUsed:   "80000",
						Events: []Event{
							{
								Type: "transfer",
								Attributes: []Attribute{
									{Key: "sender", Value: moduleAddr},
									{Key: "recipient", Value: "lumera1supernode"},
									{Key: "amount", Value: "8000ulume"},
								},
							},
						},
					},
				},
			}
		} else if query == "action_approved.action_id=123" {
			// No approve tx for this action
			response = TxSearchResponse{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)

	action := &db.Action{
		ActionID:         123,
		Creator:          "lumera1creator",
		SupernodeAccount: "lumera1supernode",
	}

	txs, err := client.GetActionTransactions(context.Background(), action)
	if err != nil {
		t.Fatalf("GetActionTransactions() error = %v", err)
	}

	if len(txs) != 2 {
		t.Fatalf("GetActionTransactions() returned %d txs, want 2", len(txs))
	}

	// Check register tx
	var registerTx, finalizeTx *db.ActionTransaction
	for _, tx := range txs {
		switch tx.TxType {
		case "register":
			registerTx = tx
		case "finalize":
			finalizeTx = tx
		}
	}

	if registerTx == nil {
		t.Fatal("Missing register transaction")
	}
	if registerTx.TxHash != "REGISTER_TX_HASH" {
		t.Errorf("Register TxHash = %v, want REGISTER_TX_HASH", registerTx.TxHash)
	}
	if registerTx.Height != 100 {
		t.Errorf("Register Height = %v, want 100", registerTx.Height)
	}
	// Check that the register flow has module as payee
	if registerTx.FlowPayee == nil || *registerTx.FlowPayee != moduleAddr {
		t.Errorf("Register FlowPayee = %v, want %v", strPtrVal(registerTx.FlowPayee), moduleAddr)
	}

	if finalizeTx == nil {
		t.Fatal("Missing finalize transaction")
	}
	if finalizeTx.TxHash != "FINALIZE_TX_HASH" {
		t.Errorf("Finalize TxHash = %v, want FINALIZE_TX_HASH", finalizeTx.TxHash)
	}
	if finalizeTx.Height != 200 {
		t.Errorf("Finalize Height = %v, want 200", finalizeTx.Height)
	}
	// Check that the finalize flow has module as payer and supernode as payee
	if finalizeTx.FlowPayer == nil || *finalizeTx.FlowPayer != moduleAddr {
		t.Errorf("Finalize FlowPayer = %v, want %v", strPtrVal(finalizeTx.FlowPayer), moduleAddr)
	}
	if finalizeTx.FlowPayee == nil || *finalizeTx.FlowPayee != "lumera1supernode" {
		t.Errorf("Finalize FlowPayee = %v, want lumera1supernode", strPtrVal(finalizeTx.FlowPayee))
	}
}

// Helper functions for tests
func strPtr(s string) *string {
	return &s
}

func strPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func strPtrVal(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
