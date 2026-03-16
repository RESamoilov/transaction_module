package consumer

import "testing"

func TestPartitionWatermarkAdvancesInOrder(t *testing.T) {
	t.Parallel()

	pw := NewPartitionWatermark(5)

	if committed, ok := pw.MarkDone(7); ok || committed != 5 {
		t.Fatalf("MarkDone(7) = (%d, %v), want (5, false)", committed, ok)
	}
	if committed, ok := pw.MarkDone(5); !ok || committed != 5 {
		t.Fatalf("MarkDone(5) = (%d, %v), want (5, true)", committed, ok)
	}
	if committed, ok := pw.MarkDone(6); !ok || committed != 7 {
		t.Fatalf("MarkDone(6) = (%d, %v), want (7, true)", committed, ok)
	}
}

func TestPartitionWatermarkIgnoresOldOffsets(t *testing.T) {
	t.Parallel()

	pw := NewPartitionWatermark(3)
	if _, ok := pw.MarkDone(3); !ok {
		t.Fatal("initial offset should commit")
	}

	if committed, ok := pw.MarkDone(2); ok || committed != 3 {
		t.Fatalf("MarkDone(2) = (%d, %v), want (3, false)", committed, ok)
	}
}
