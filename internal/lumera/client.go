package lumera

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a minimal Lumera/Cosmos SDK REST client using stdlib only.
type Client struct {
	BaseURL    string
	HTTP       *http.Client
	UserAgent  string
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP: &http.Client{Timeout: timeout},
		UserAgent: "lumescope/preview",
	}
}

func (c *Client) doJSON(ctx context.Context, method, path string, q url.Values, v any) error {
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil { return err }
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" { req.Header.Set("User-Agent", c.UserAgent) }
	resp, err := c.HTTP.Do(req)
	if err != nil { return err }
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
	Jailed         bool   `json:"jailed"`
	Status         string `json:"status"`
	Description    struct {
		Moniker string `json:"moniker"`
	} `json:"description"`
}

// GetValidators fetches validators (all statuses). If status is empty, UNSPECIFIED is used.
func (c *Client) GetValidators(ctx context.Context, nextKey string, limit int) (vals []Validator, newNextKey string, err error) {
	q := url.Values{}
	q.Set("status", "BOND_STATUS_UNSPECIFIED")
	if limit > 0 { q.Set("pagination.limit", fmt.Sprint(limit)) }
	if nextKey != "" { q.Set("pagination.key", nextKey) }
	var out ValidatorsResponse
	err = c.doJSON(ctx, http.MethodGet, "/cosmos/staking/v1beta1/validators", q, &out)
	if err != nil { return nil, "", err }
	if out.Pagination != nil { newNextKey = out.Pagination.NextKey }
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

type Supernode struct {
	ValidatorAddress   string `json:"validator_address"`
	States             []SupernodeState `json:"states"`
	Evidence           any    `json:"evidence"`
	PrevIPAddresses    []PrevIPAddress `json:"prev_ip_addresses"`
	Note               string `json:"note"` // protocol version note, e.g., "1.0.0"
	Metrics            struct {
		Metrics    map[string]any `json:"metrics"`
		ReportCount string        `json:"report_count"`
		Height      string        `json:"height"`
	} `json:"metrics"`
	SupernodeAccount   string `json:"supernode_account"`
	P2PPortStr         string `json:"p2p_port"`
	PrevSupernodeAccounts []string `json:"prev_supernode_accounts"`
}

func (c *Client) GetSupernodes(ctx context.Context, nextKey string, limit int) (sns []Supernode, newNextKey string, err error) {
	q := url.Values{}
	if limit > 0 { q.Set("pagination.limit", fmt.Sprint(limit)) }
	if nextKey != "" { q.Set("pagination.key", nextKey) }
	var out ListSupernodesResponse
	err = c.doJSON(ctx, http.MethodGet, "/LumeraProtocol/lumera/supernode/list_super_nodes", q, &out)
	if err != nil { return nil, "", err }
	if out.Pagination != nil { newNextKey = out.Pagination.NextKey }
	return out.Supernodes, newNextKey, nil
}

// Actions

type ListActionsResponse struct {
	Actions    []Action `json:"actions"`
	Pagination *Pagination `json:"pagination"`
	Total      string   `json:"total"`
}

type Action struct {
	Creator       string    `json:"creator"`
	ActionID      string    `json:"actionID"`
	ActionType    string    `json:"actionType"`
	MetadataB64   string    `json:"metadata"`
	Price         struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"price"`
	ExpirationTime string   `json:"expirationTime"`
	State          string   `json:"state"`
	BlockHeight    string   `json:"blockHeight"`
	SuperNodes     []string `json:"superNodes"`
}

func (c *Client) GetActions(ctx context.Context, actionType, actionState, nextKey string, limit int) (as []Action, newNextKey string, err error) {
	q := url.Values{}
	if actionType != "" { q.Set("actionType", actionType) } else { q.Set("actionType", "ACTION_TYPE_UNSPECIFIED") }
	if actionState != "" { q.Set("actionState", actionState) } else { q.Set("actionState", "ACTION_STATE_UNSPECIFIED") }
	if limit > 0 { q.Set("pagination.limit", fmt.Sprint(limit)) }
	if nextKey != "" { q.Set("pagination.key", nextKey) }
	var out ListActionsResponse
	err = c.doJSON(ctx, http.MethodGet, "/LumeraProtocol/lumera/action/list_actions", q, &out)
	if err != nil { return nil, "", err }
	if out.Pagination != nil { newNextKey = out.Pagination.NextKey }
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
