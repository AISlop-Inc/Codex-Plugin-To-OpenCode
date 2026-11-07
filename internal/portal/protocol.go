package portal

import (
	"bytes"
	"encoding/json"
	"errors"
)

const maxMessageSize = 32 << 20

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	wire    json.RawMessage
}

func parseRequest(wire []byte) (request, error) {
	var message request
	if json.Unmarshal(wire, &message) != nil || message.JSONRPC != "2.0" || message.Method == "" {
		return request{}, errors.New("Invalid MCP request.")
	}
	if message.ID != nil && !validID(message.ID) {
		return request{}, errors.New("Invalid MCP request ID.")
	}
	if message.Params != nil && !isObject(message.Params) {
		return request{}, errors.New("Invalid MCP request parameters.")
	}
	message.wire = bytes.Clone(wire)
	return message, nil
}

func validID(raw json.RawMessage) bool {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return false
	}
	switch value.(type) {
	case nil, string, json.Number:
		return true
	default:
		return false
	}
}

func sameID(left, right json.RawMessage) bool {
	decode := func(raw json.RawMessage) any {
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		_ = decoder.Decode(&value)
		return value
	}
	return validID(left) && validID(right) && decode(left) == decode(right)
}

func isObject(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && raw[0] == '{' && json.Valid(raw)
}

func prepareRequest(message request) ([]byte, error) {
	if message.Method != "initialize" {
		return message.wire, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(message.wire, &fields); err != nil {
		return nil, errors.New("Invalid MCP request.")
	}
	params := map[string]json.RawMessage{}
	if message.Params != nil {
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return nil, errors.New("Invalid MCP initialization parameters.")
		}
	}
	// The portal cannot service server-initiated sampling or elicitation requests.
	params["capabilities"] = json.RawMessage(`{}`)
	fields["params"], _ = json.Marshal(params)
	return json.Marshal(fields)
}

func validateResponse(wire []byte, id json.RawMessage) (json.RawMessage, error) {
	var message struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal(wire, &message) != nil || message.JSONRPC != "2.0" || !sameID(message.ID, id) {
		return nil, errors.New("MCP response does not match the request.")
	}
	if (message.Result == nil) == (message.Error == nil) {
		return nil, errors.New("Invalid MCP response.")
	}
	if message.Result != nil && !isObject(message.Result) {
		return nil, errors.New("Invalid MCP response result.")
	}
	if message.Error != nil {
		var detail struct {
			Code    *int    `json:"code"`
			Message *string `json:"message"`
		}
		if json.Unmarshal(message.Error, &detail) != nil || detail.Code == nil || detail.Message == nil {
			return nil, errors.New("Invalid MCP error response.")
		}
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, wire); err != nil {
		return nil, errors.New("Invalid MCP response.")
	}
	return compact.Bytes(), nil
}

func normalizeToolResult(wire json.RawMessage) (json.RawMessage, error) {
	var message map[string]json.RawMessage
	if err := json.Unmarshal(wire, &message); err != nil {
		return nil, errors.New("Invalid MCP tool response.")
	}
	if message["result"] == nil {
		return wire, nil
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(message["result"], &result); err != nil {
		return nil, errors.New("Invalid MCP tool result.")
	}
	if !bytes.Equal(bytes.TrimSpace(result["structuredContent"]), []byte("null")) {
		return wire, nil
	}
	// Upstream text-only results use null, but MCP requires this optional field to be an object or absent.
	delete(result, "structuredContent")
	message["result"], _ = json.Marshal(result)
	return json.Marshal(message)
}

func errorResponse(id json.RawMessage, code int, detail string) json.RawMessage {
	if id == nil {
		id = json.RawMessage(`null`)
	}
	response := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{JSONRPC: "2.0", ID: id}
	response.Error.Code = code
	response.Error.Message = detail
	wire, _ := json.Marshal(response)
	return wire
}
