package analytics

import (
	"io"
	"log/slog"
	"time"
)

func NewWithSources(logger *slog.Logger, keyRand io.Reader, now func() time.Time) *Recorder {
	return &Recorder{logger: logger, keyRand: keyRand, now: now}
}
