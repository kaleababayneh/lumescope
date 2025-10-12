package background

import (
	"testing"

	lclient "lumescope/internal/lumera"
)

func TestLatestState(t *testing.T) {
	tests := []struct {
		name       string
		states     []lclient.SupernodeState
		wantState  string
		wantHeight string
	}{
		{
			name:       "empty states",
			states:     []lclient.SupernodeState{},
			wantState:  "SUPERNODE_STATE_UNKNOWN",
			wantHeight: "",
		},
		{
			name: "single state",
			states: []lclient.SupernodeState{
				{State: "SUPERNODE_STATE_ACTIVE", Height: "412540"},
			},
			wantState:  "SUPERNODE_STATE_ACTIVE",
			wantHeight: "412540",
		},
		{
			name: "multiple states - highest is last",
			states: []lclient.SupernodeState{
				{State: "SUPERNODE_STATE_ACTIVE", Height: "412540"},
				{State: "SUPERNODE_STATE_DISABLED", Height: "517710"},
				{State: "SUPERNODE_STATE_ACTIVE", Height: "890403"},
			},
			wantState:  "SUPERNODE_STATE_ACTIVE",
			wantHeight: "890403",
		},
		{
			name: "multiple states - highest is NOT last (unordered)",
			states: []lclient.SupernodeState{
				{State: "SUPERNODE_STATE_ACTIVE", Height: "412540"},
				{State: "SUPERNODE_STATE_DISABLED", Height: "890403"}, // highest
				{State: "SUPERNODE_STATE_ACTIVE", Height: "517710"},
			},
			wantState:  "SUPERNODE_STATE_DISABLED",
			wantHeight: "890403",
		},
		{
			name: "real world example from issue",
			states: []lclient.SupernodeState{
				{State: "SUPERNODE_STATE_ACTIVE", Height: "412540"},
				{State: "SUPERNODE_STATE_DISABLED", Height: "517710"},
				{State: "SUPERNODE_STATE_ACTIVE", Height: "517799"},
				{State: "SUPERNODE_STATE_DISABLED", Height: "517863"},
				{State: "SUPERNODE_STATE_ACTIVE", Height: "517994"},
				{State: "SUPERNODE_STATE_DISABLED", Height: "518191"},
				{State: "SUPERNODE_STATE_ACTIVE", Height: "518838"},
				{State: "SUPERNODE_STATE_DISABLED", Height: "518897"},
				{State: "SUPERNODE_STATE_ACTIVE", Height: "518939"},
				{State: "SUPERNODE_STATE_DISABLED", Height: "518978"},
				{State: "SUPERNODE_STATE_ACTIVE", Height: "519085"},
				{State: "SUPERNODE_STATE_DISABLED", Height: "663601"},
				{State: "SUPERNODE_STATE_ACTIVE", Height: "666506"},
				{State: "SUPERNODE_STATE_STOPPED", Height: "890394"},
				{State: "SUPERNODE_STATE_ACTIVE", Height: "890403"}, // highest
			},
			wantState:  "SUPERNODE_STATE_ACTIVE",
			wantHeight: "890403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotHeight := latestState(tt.states)
			if gotState != tt.wantState {
				t.Errorf("latestState() state = %v, want %v", gotState, tt.wantState)
			}
			if gotHeight != tt.wantHeight {
				t.Errorf("latestState() height = %v, want %v", gotHeight, tt.wantHeight)
			}
		})
	}
}

func TestLatestIPAddress(t *testing.T) {
	tests := []struct {
		name      string
		addresses []lclient.PrevIPAddress
		want      string
	}{
		{
			name:      "empty addresses",
			addresses: []lclient.PrevIPAddress{},
			want:      "",
		},
		{
			name: "single address",
			addresses: []lclient.PrevIPAddress{
				{Address: "62.169.16.57:4444", Height: "412540"},
			},
			want: "62.169.16.57:4444",
		},
		{
			name: "multiple addresses - highest is last",
			addresses: []lclient.PrevIPAddress{
				{Address: "62.169.16.57:4444", Height: "412540"},
				{Address: "152.53.137.213:4444", Height: "836657"},
				{Address: "152.53.138.217:4444", Height: "951118"},
			},
			want: "152.53.138.217:4444",
		},
		{
			name: "multiple addresses - highest is NOT last (unordered)",
			addresses: []lclient.PrevIPAddress{
				{Address: "62.169.16.57:4444", Height: "412540"},
				{Address: "152.53.138.217:4444", Height: "951118"}, // highest
				{Address: "152.53.137.213:4444", Height: "836657"},
			},
			want: "152.53.138.217:4444",
		},
		{
			name: "real world example from issue",
			addresses: []lclient.PrevIPAddress{
				{Address: "62.169.16.57:4444", Height: "412540"},
				{Address: "152.53.137.213:4444", Height: "836657"},
				{Address: "152.53.137.213:4444 ", Height: "879597"}, // note trailing space
				{Address: "152.53.138.217:4444", Height: "951118"},  // highest
			},
			want: "152.53.138.217:4444",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := latestIPAddress(tt.addresses); got != tt.want {
				t.Errorf("latestIPAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
