package decoder

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

// DecodeActionMetadata decodes the base64-encoded metadata according to the action type
// and returns the raw bytes plus a JSON-serializable map representation.
func DecodeActionMetadata(actionType string, metadataB64 string) (raw []byte, decodedMap map[string]any, err error) {
	raw, err = base64.StdEncoding.DecodeString(metadataB64)
	if err != nil {
		return nil, nil, fmt.Errorf("base64 decode: %w", err)
	}
	var msg gogoproto.Message
	switch actionType {
	case "ACTION_TYPE_CASCADE":
		msg = &actiontypes.CascadeMetadata{}
	case "ACTION_TYPE_SENSE":
		msg = &actiontypes.SenseMetadata{}
	default:
		// Unknown type: return raw only
		return raw, nil, nil
	}
	if err := gogoproto.Unmarshal(raw, msg); err != nil {
		return raw, nil, fmt.Errorf("proto unmarshal: %w", err)
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return raw, nil, fmt.Errorf("json marshal: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return raw, nil, err
	}
	return raw, m, nil
}
