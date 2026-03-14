package consumer

import (
	"sync"
)

type PartitionWatermark struct {
	mu        sync.Mutex
	watermark int64
	completed map[int64]bool
}

func NewPartitionWatermark(startOffset int64) *PartitionWatermark {
	return &PartitionWatermark{
		watermark: startOffset,
		completed: make(map[int64]bool),
	}
}

func (pw *PartitionWatermark) MarkDone(offset int64) (int64, bool) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	if offset > pw.watermark {
		pw.completed[offset] = true
		return pw.watermark, false
	}

	if offset == pw.watermark {
		pw.watermark++
		for pw.completed[pw.watermark] {
			delete(pw.completed, pw.watermark)
			pw.watermark++
		}
		return pw.watermark - 1, true
	}

	return pw.watermark - 1, false
}
