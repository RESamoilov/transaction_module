package dto

import (
	"testing"
	"time"

	"github.com/meindokuse/transaction-module/internal/domain"
)

func TestEncodeDecodeCursorRoundTrip(t *testing.T) {
	t.Parallel()

	want := CursorPayload{
		LastID:        "tx-1",
		LastTimestamp: time.Date(2026, 3, 14, 10, 20, 30, 40, time.UTC).UnixNano(),
	}

	encoded, err := EncodeCursor(want)
	if err != nil {
		t.Fatalf("EncodeCursor() error = %v", err)
	}

	got, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor() error = %v", err)
	}

	if got == nil {
		t.Fatal("DecodeCursor() returned nil payload")
	}

	if *got != want {
		t.Fatalf("DecodeCursor() = %+v, want %+v", *got, want)
	}
}

func TestDecodeCursorInvalidBase64(t *testing.T) {
	t.Parallel()

	if _, err := DecodeCursor("%%%"); err == nil {
		t.Fatal("DecodeCursor() error = nil, want error for invalid base64")
	}
}

func TestBuildPageRequest(t *testing.T) {
	t.Parallel()

	payload := &CursorPayload{
		LastID:        "tx-7",
		LastTimestamp: time.Date(2026, 3, 14, 12, 0, 0, 55, time.UTC).UnixNano(),
	}

	got := BuildPageRequest(25, payload)

	if got.Limit != 25 {
		t.Fatalf("BuildPageRequest().Limit = %d, want 25", got.Limit)
	}
	if got.LastID != payload.LastID {
		t.Fatalf("BuildPageRequest().LastID = %q, want %q", got.LastID, payload.LastID)
	}
	if !got.LastTimestamp.Equal(time.Unix(0, payload.LastTimestamp)) {
		t.Fatalf("BuildPageRequest().LastTimestamp = %v, want %v", got.LastTimestamp, time.Unix(0, payload.LastTimestamp))
	}
}

func TestBuildResponseWithNextCursor(t *testing.T) {
	t.Parallel()

	ts1 := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	txs := []*domain.Transaction{
		{ID: "tx-1", UserID: "user-1", Amount: 10, Type: domain.TxTypeBet, Timestamp: ts1},
		{ID: "tx-2", UserID: "user-1", Amount: 20, Type: domain.TxTypeWin, Timestamp: ts2},
	}

	resp := BuildResponse(txs, 2)

	if len(resp.Data) != 2 {
		t.Fatalf("BuildResponse().Data len = %d, want 2", len(resp.Data))
	}
	if !resp.Meta.HasMore {
		t.Fatal("BuildResponse().Meta.HasMore = false, want true")
	}
	if resp.Data[0].Type != domain.TxTypeBet {
		t.Fatalf("BuildResponse().Data[0].Type = %q, want %q", resp.Data[0].Type, domain.TxTypeBet)
	}

	payload, err := DecodeCursor(resp.Meta.NextCursor)
	if err != nil {
		t.Fatalf("DecodeCursor(nextCursor) error = %v", err)
	}
	if payload == nil {
		t.Fatal("DecodeCursor(nextCursor) returned nil payload")
	}
	if payload.LastID != "tx-2" {
		t.Fatalf("next cursor LastID = %q, want %q", payload.LastID, "tx-2")
	}
}
