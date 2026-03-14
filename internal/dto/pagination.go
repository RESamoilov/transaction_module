package dto

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// DecodeCursor распаковывает Base64 строку -> JSON -> структуру CursorPayload
func DecodeCursor(encoded string) (*CursorPayload, error) {
	if encoded == "" {
		return nil, nil
	}

	decodedBytes, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 cursor: %w", err)
	}

	var payload CursorPayload
	if err := json.Unmarshal(decodedBytes, &payload); err != nil {
		return nil, fmt.Errorf("invalid json cursor: %w", err)
	}

	return &payload, nil
}

// EncodeCursor запаковывает структуру CursorPayload -> JSON -> Base64 строку
func EncodeCursor(payload CursorPayload) (string, error) {
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonBytes), nil
}
