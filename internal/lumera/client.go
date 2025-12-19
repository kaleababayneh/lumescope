package lumera

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"lumescope/internal/db"
)

// Client is a minimal Lumera/Cosmos SDK REST client using stdlib only.
type Client struct {
	BaseURL           string
	HTTP              *http.Client
	UserAgent         string
	actionModuleAddr  string // cached action module address
	moduleAddrFetched bool   // whether we've fetched the module address
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		HTTP:      &http.Client{Timeout: timeout},
		UserAgent: "lumescope/preview",
	}
}

// ModuleAccountResponse represents the response from /cosmos/auth/v1beta1/module_accounts/{name}
type ModuleAccountResponse struct {
	Account struct {
		Type        string `json:"@type"`
		BaseAccount struct {
			Address string `json:"address"`
		} `json:"base_account"`
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	} `json:"account"`
}

// GetActionModuleAccount returns the cached action module address, fetching it if necessary.
// The address is cached after the first successful fetch.
func (c *Client) GetActionModuleAccount(ctx context.Context) (string, error) {
	// Return cached value if available
	if c.moduleAddrFetched && c.actionModuleAddr != "" {
		return c.actionModuleAddr, nil
	}

	// Fetch from API
	var resp ModuleAccountResponse
	err := c.doJSON(ctx, http.MethodGet, "/cosmos/auth/v1beta1/module_accounts/action", nil, &resp)
	if err != nil {
		return "", fmt.Errorf("failed to fetch action module account: %w", err)
	}

	if resp.Account.BaseAccount.Address == "" {
		return "", fmt.Errorf("action module account address is empty")
	}

	// Cache the result
	c.actionModuleAddr = resp.Account.BaseAccount.Address
	c.moduleAddrFetched = true

	log.Printf("Fetched and cached action module address: %s", c.actionModuleAddr)
	return c.actionModuleAddr, nil
}

// SetActionModuleAccount sets the action module address (useful for testing).
func (c *Client) SetActionModuleAccount(addr string) {
	c.actionModuleAddr = addr
	c.moduleAddrFetched = true
}

func (c *Client) doJSON(ctx context.Context, method, path string, q url.Values, v any) error {
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("http %s %s: %d: %s", method, u, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(v)
}

// Validators

type ValidatorsResponse struct {
	Validators []Validator `json:"validators"`
	Pagination *Pagination `json:"pagination"`
}

type Validator struct {
	OperatorAddress string `json:"operator_address"`
	Jailed          bool   `json:"jailed"`
	Status          string `json:"status"`
	Description     struct {
		Moniker string `json:"moniker"`
	} `json:"description"`
}

// GetValidators fetches validators (all statuses).
func (c *Client) GetValidators(ctx context.Context, nextKey string, limit int) (vals []Validator, newNextKey string, err error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("pagination.limit", fmt.Sprint(limit))
	}
	if nextKey != "" {
		q.Set("pagination.key", nextKey)
	}
	var out ValidatorsResponse
	err = c.doJSON(ctx, http.MethodGet, "/cosmos/staking/v1beta1/validators", q, &out)
	if err != nil {
		return nil, "", err
	}
	if out.Pagination != nil {
		newNextKey = out.Pagination.NextKey
	}
	return out.Validators, newNextKey, nil
}

// Supernodes

type ListSupernodesResponse struct {
	Supernodes []Supernode `json:"supernodes"`
	Pagination *Pagination `json:"pagination"`
}

type SupernodeState struct {
	State  string `json:"state"`
	Height string `json:"height"`
}

type PrevIPAddress struct {
	Address string `json:"address"`
	Height  string `json:"height"`
}

type PrevSupernodeAccount struct {
	Account string `json:"account"`
	Height  string `json:"height"`
}

type Evidence struct {
	ActionID         string `json:"action_id"`
	Description      string `json:"description"`
	EvidenceType     string `json:"evidence_type"`
	Height           int32  `json:"height"`
	ReporterAddress  string `json:"reporter_address"`
	Severity         string `json:"severity"`
	ValidatorAddress string `json:"validator_address"`
}

type MetricsAggregate struct {
	Metrics     map[string]any `json:"metrics"`
	ReportCount string         `json:"report_count"`
	Height      string         `json:"height"`
}

type Supernode struct {
	ValidatorAddress      string                 `json:"validator_address"`
	States                []SupernodeState       `json:"states"`
	Evidence              []Evidence             `json:"evidence"`
	PrevIPAddresses       []PrevIPAddress        `json:"prev_ip_addresses"`
	Note                  string                 `json:"note"` // protocol version note, e.g., "1.0.0"
	Metrics               MetricsAggregate       `json:"metrics"`
	SupernodeAccount      string                 `json:"supernode_account"`
	P2PPortStr            string                 `json:"p2p_port"`
	PrevSupernodeAccounts []PrevSupernodeAccount `json:"prev_supernode_accounts"`
}

func (c *Client) GetSupernodes(ctx context.Context, nextKey string, limit int) (sns []Supernode, newNextKey string, err error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("pagination.limit", fmt.Sprint(limit))
	}
	if nextKey != "" {
		q.Set("pagination.key", nextKey)
	}
	var out ListSupernodesResponse
	err = c.doJSON(ctx, http.MethodGet, "/LumeraProtocol/lumera/supernode/v1/list_super_nodes", q, &out)
	if err != nil {
		return nil, "", err
	}
	if out.Pagination != nil {
		newNextKey = out.Pagination.NextKey
	}
	return out.Supernodes, newNextKey, nil
}

// Actions

type ListActionsResponse struct {
	Actions    []Action    `json:"actions"`
	Pagination *Pagination `json:"pagination"`
	Total      string      `json:"total"`
}

// PriceField handles unmarshaling price from both string ("10090ulume") and struct formats
type PriceField struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

// UnmarshalJSON implements custom unmarshaling for PriceField to handle both string and struct formats
func (p *PriceField) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a struct first
	var priceStruct struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	}
	if err := json.Unmarshal(data, &priceStruct); err == nil && (priceStruct.Denom != "" || priceStruct.Amount != "") {
		p.Denom = priceStruct.Denom
		p.Amount = priceStruct.Amount
		return nil
	}

	// Try to unmarshal as a string (e.g., "10090ulume")
	var priceStr string
	if err := json.Unmarshal(data, &priceStr); err == nil {
		// Parse the coin string format: "10090ulume" -> amount="10090", denom="ulume"
		p.Amount, p.Denom = parseCoinString(priceStr)
		return nil
	}

	// If both fail, set empty values
	p.Denom = ""
	p.Amount = ""
	return nil
}

// parseCoinString parses a coin string like "10090ulume" into amount and denom
func parseCoinString(s string) (amount, denom string) {
	if s == "" {
		return "", ""
	}
	// Find where the numeric part ends
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9') {
		i++
	}
	return s[:i], s[i:]
}

type Action struct {
	Creator        string     `json:"creator"`
	ActionID       string     `json:"actionID"`
	ActionType     string     `json:"actionType"`
	MetadataB64    string     `json:"metadata"`
	Price          PriceField `json:"price"`
	ExpirationTime string     `json:"expirationTime"`
	State          string     `json:"state"`
	BlockHeight    string     `json:"blockHeight"`
	SuperNodes     []string   `json:"superNodes"`
}

func (c *Client) GetActions(ctx context.Context, actionType, actionState, nextKey string, limit int) (as []Action, newNextKey string, err error) {
	q := url.Values{}
	if actionType != "" {
		q.Set("actionType", actionType)
	} else {
		q.Set("actionType", "ACTION_TYPE_UNSPECIFIED")
	}
	if actionState != "" {
		q.Set("actionState", actionState)
	} else {
		q.Set("actionState", "ACTION_STATE_UNSPECIFIED")
	}
	if limit > 0 {
		q.Set("pagination.limit", fmt.Sprint(limit))
	}
	if nextKey != "" {
		q.Set("pagination.key", nextKey)
	}
	var out ListActionsResponse
	err = c.doJSON(ctx, http.MethodGet, "/LumeraProtocol/lumera/action/v1/list_actions", q, &out)
	if err != nil {
		return nil, "", err
	}
	if out.Pagination != nil {
		newNextKey = out.Pagination.NextKey
	}
	return out.Actions, newNextKey, nil
}

// Shared

type Pagination struct {
	NextKey string `json:"next_key"`
	Total   string `json:"total"`
}

// Utilities

// IsValidIP reports whether s is a valid IPv4/IPv6 literal.
func IsValidIP(s string) bool { return net.ParseIP(s) != nil }

var ErrInvalidBaseURL = errors.New("invalid base URL")

// Transaction search types for Cosmos SDK tx_search endpoint

// TxSearchResponse represents the response from /cosmos/tx/v1beta1/txs
type TxSearchResponse struct {
	Txs         []TxResponse `json:"txs"`
	TxResponses []TxResult   `json:"tx_responses"`
	Pagination  *Pagination  `json:"pagination"`
}

// TxResponse contains the raw transaction
type TxResponse struct {
	Body     TxBody   `json:"body"`
	AuthInfo AuthInfo `json:"auth_info"`
}

// TxBody contains transaction messages
type TxBody struct {
	Messages []json.RawMessage `json:"messages"`
}

// AuthInfo contains fee information
type AuthInfo struct {
	Fee         Fee          `json:"fee"`
	SignerInfos []SignerInfo `json:"signer_infos"`
}

// SignerInfo contains signer information
type SignerInfo struct {
	PublicKey json.RawMessage `json:"public_key"`
	Sequence  string          `json:"sequence"`
}

// Fee contains fee amount
type Fee struct {
	Amount []Coin `json:"amount"`
}

// Coin represents a denomination and amount
type Coin struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

// TxResult contains the transaction execution result
type TxResult struct {
	TxHash    string    `json:"txhash"`
	Height    string    `json:"height"`
	Timestamp string    `json:"timestamp"`
	GasWanted string    `json:"gas_wanted"`
	GasUsed   string    `json:"gas_used"`
	Events    []Event   `json:"events"`
	RawLog    string    `json:"raw_log"`
	Logs      []ABCILog `json:"logs"`
}

// Event represents a transaction event
type Event struct {
	Type       string      `json:"type"`
	Attributes []Attribute `json:"attributes"`
}

// Attribute is a key-value pair in an event
type Attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ABCILog represents a log entry from transaction execution
type ABCILog struct {
	MsgIndex int     `json:"msg_index"`
	Events   []Event `json:"events"`
}

// GetActionTransactions fetches transaction details for an action's lifecycle events.
// It queries for register, finalize, and approve transactions based on action events.
// Returns ActionTransaction records ready to be persisted.
func (c *Client) GetActionTransactions(ctx context.Context, action *db.Action) ([]*db.ActionTransaction, error) {
	var results []*db.ActionTransaction

	// Fetch module account address for proper transfer flow parsing
	moduleAddr, err := c.GetActionModuleAccount(ctx)
	if err != nil {
		// Log but continue - we can still parse with fallbacks
		log.Printf("GetActionTransactions: failed to get module account address: %v", err)
	}

	// Query patterns for different transaction types
	queries := []struct {
		eventType string
		txType    string
	}{
		{"action_registered.action_id", "register"},
		{"action_finalized.action_id", "finalize"},
		{"action_approved.action_id", "approve"},
	}

	for _, q := range queries {
		// Convert uint64 ActionID to string for API query
		txs, err := c.searchTxsByEvent(ctx, q.eventType, strconv.FormatUint(action.ActionID, 10))
		if err != nil {
			// Log but continue with other queries
			continue
		}

		for i, txResult := range txs.TxResponses {
			var tx *TxResponse
			if i < len(txs.Txs) {
				tx = &txs.Txs[i]
			}

			actionTx := c.parseTxResult(action, q.txType, txResult, tx, moduleAddr)
			if actionTx != nil {
				results = append(results, actionTx)
			}
		}
	}

	return results, nil
}

// searchTxsByEvent queries the Cosmos SDK tx_search endpoint for transactions
// matching a specific event type and value.
// It implements a fallback strategy: first tries without quotes, then with quotes if 0 results.
func (c *Client) searchTxsByEvent(ctx context.Context, eventType, value string) (*TxSearchResponse, error) {
	// First attempt: without quotes around value
	// Format: query=action_registered.action_id=ACTION_ID
	q := url.Values{}
	q.Set("query", fmt.Sprintf("%s=%s", eventType, value))
	q.Set("pagination.limit", "10")

	fullURL := c.BaseURL + "/cosmos/tx/v1beta1/txs?" + q.Encode()
	log.Printf("searchTxsByEvent: querying %s", fullURL)

	var out TxSearchResponse
	err := c.doJSON(ctx, http.MethodGet, "/cosmos/tx/v1beta1/txs", q, &out)
	if err != nil {
		log.Printf("searchTxsByEvent: error querying %s: %v", fullURL, err)
		return nil, err
	}

	// If we got results, return them
	if len(out.TxResponses) > 0 {
		log.Printf("searchTxsByEvent: got %d tx_responses for event %s=%s", len(out.TxResponses), eventType, value)
	}

	return &out, nil
}

// parseTxResult extracts transaction details and flow information from a transaction result.
func (c *Client) parseTxResult(action *db.Action, txType string, txResult TxResult, tx *TxResponse, moduleAddr string) *db.ActionTransaction {
	height, _ := strconv.ParseInt(txResult.Height, 10, 64)
	gasWanted, _ := strconv.ParseInt(txResult.GasWanted, 10, 64)
	gasUsed, _ := strconv.ParseInt(txResult.GasUsed, 10, 64)

	// Parse timestamp
	blockTime, _ := time.Parse(time.RFC3339, txResult.Timestamp)

	actionTx := &db.ActionTransaction{
		ActionID:  action.ActionID,
		TxType:    txType,
		TxHash:    txResult.TxHash,
		Height:    height,
		BlockTime: blockTime,
		GasWanted: &gasWanted,
		GasUsed:   &gasUsed,
	}

	// Extract fee information and set TxFee fields
	if tx != nil && len(tx.AuthInfo.Fee.Amount) > 0 {
		fee := tx.AuthInfo.Fee.Amount[0]
		actionTx.TxFee = &fee.Amount
		actionTx.TxFeeDenom = &fee.Denom
	}

	// Extract transaction signer from the message
	txSigner := extractTxSigner(tx)

	// Extract flow information from transfer events
	flow := c.extractTransferFlow(action, txType, txResult, moduleAddr, txSigner)
	if flow != nil {
		actionTx.ActionPrice = flow.Amount
		actionTx.ActionPriceDenom = flow.Denom
		actionTx.FlowPayer = flow.Payer
		actionTx.FlowPayee = flow.Payee
	}

	return actionTx
}

// extractTxSigner extracts the transaction signer (creator) from the first message.
// It looks for common fields like "creator", "sender", or "from_address" in the message.
func extractTxSigner(tx *TxResponse) string {
	if tx == nil || len(tx.Body.Messages) == 0 {
		return ""
	}

	// Parse the first message to extract signer
	var msgMap map[string]interface{}
	if err := json.Unmarshal(tx.Body.Messages[0], &msgMap); err != nil {
		return ""
	}

	// Check common signer field names
	signerFields := []string{"creator", "sender", "from_address", "signer"}
	for _, field := range signerFields {
		if val, ok := msgMap[field]; ok {
			if strVal, ok := val.(string); ok && strVal != "" {
				return strVal
			}
		}
	}

	return ""
}

// TransferFlow represents a token transfer in a transaction
type TransferFlow struct {
	Amount *string
	Denom  *string
	Payer  *string
	Payee  *string
}

// extractTransferFlow parses transfer events to identify token flows.
// For 'register': finds transfer where recipient == Action Module Address (creator pays to module)
// For 'finalize': finds transfer where sender == Action Module Address AND recipient == tx signer
// For 'approve': similar to finalize
func (c *Client) extractTransferFlow(action *db.Action, txType string, txResult TxResult, moduleAddr, txSigner string) *TransferFlow {
	// Look through all events for transfer events
	var transfers []TransferFlow

	// Check events at top level
	for _, event := range txResult.Events {
		if event.Type == "transfer" {
			tf := parseTransferEvent(event.Attributes)
			if tf != nil {
				transfers = append(transfers, *tf)
			}
		}
	}

	// Also check events in logs (some Cosmos SDK versions put them there)
	for _, log := range txResult.Logs {
		for _, event := range log.Events {
			if event.Type == "transfer" {
				tf := parseTransferEvent(event.Attributes)
				if tf != nil {
					transfers = append(transfers, *tf)
				}
			}
		}
	}

	if len(transfers) == 0 {
		return nil
	}

	// Select the appropriate transfer based on transaction type
	switch txType {
	case "register":
		// For registration, find transfer where recipient == module address
		// The creator pays the actionPrice to the module account
		if moduleAddr != "" {
			for _, tf := range transfers {
				if tf.Payee != nil && *tf.Payee == moduleAddr {
					return &tf
				}
			}
		}
		// Fallback: find transfer where sender == action.Creator
		for _, tf := range transfers {
			if tf.Payer != nil && *tf.Payer == action.Creator {
				return &tf
			}
		}
		// Fallback: return first transfer
		if len(transfers) > 0 {
			return &transfers[0]
		}

	case "finalize", "approve":
		// For finalize/approve, find transfer where:
		// sender == module address AND recipient == tx signer (creator of MsgFinalizeAction)
		// The module pays out to the transaction signer
		if moduleAddr != "" && txSigner != "" {
			for _, tf := range transfers {
				if tf.Payer != nil && *tf.Payer == moduleAddr &&
					tf.Payee != nil && *tf.Payee == txSigner {
					return &tf
				}
			}
		}
		// Fallback: find transfer where sender == module address AND recipient == supernode account
		if moduleAddr != "" && action.SupernodeAccount != "" {
			for _, tf := range transfers {
				if tf.Payer != nil && *tf.Payer == moduleAddr &&
					tf.Payee != nil && *tf.Payee == action.SupernodeAccount {
					return &tf
				}
			}
		}
		// Fallback: find transfer where sender == module address
		if moduleAddr != "" {
			for _, tf := range transfers {
				if tf.Payer != nil && *tf.Payer == moduleAddr {
					return &tf
				}
			}
		}
		// Fallback: find transfer where recipient == tx signer
		if txSigner != "" {
			for _, tf := range transfers {
				if tf.Payee != nil && *tf.Payee == txSigner {
					return &tf
				}
			}
		}
		// Fallback: find transfer where recipient == supernode account
		if action.SupernodeAccount != "" {
			for _, tf := range transfers {
				if tf.Payee != nil && *tf.Payee == action.SupernodeAccount {
					return &tf
				}
			}
		}
		// Alternative: find transfer where sender is NOT the creator (likely module account)
		for _, tf := range transfers {
			if tf.Payer != nil && *tf.Payer != action.Creator {
				return &tf
			}
		}
		// Fallback: return first transfer
		if len(transfers) > 0 {
			return &transfers[0]
		}
	}

	return nil
}

// parseTransferEvent extracts transfer details from event attributes.
// Expects attributes: sender, recipient, amount
func parseTransferEvent(attrs []Attribute) *TransferFlow {
	tf := &TransferFlow{}

	for _, attr := range attrs {
		switch attr.Key {
		case "sender":
			tf.Payer = &attr.Value
		case "recipient":
			tf.Payee = &attr.Value
		case "amount":
			// Amount format: "10090ulume"
			amount, denom := parseCoinString(attr.Value)
			if amount != "" {
				tf.Amount = &amount
			}
			if denom != "" {
				tf.Denom = &denom
			}
		}
	}

	// Only return if we have at least sender or recipient
	if tf.Payer == nil && tf.Payee == nil {
		return nil
	}

	return tf
}
