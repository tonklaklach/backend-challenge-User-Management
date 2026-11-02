package worker

import (
	"context"
	"log/slog"
	"time"
)

const userCountInterval = 10 * time.Second

type UserCounter interface {
	Count(context.Context) (int64, error)
}

func LogUserCount(ctx context.Context, counter UserCounter, logger *slog.Logger) {
	ticker := time.NewTicker(userCountInterval)
	defer ticker.Stop()
	logUserCount(ctx, counter, logger, ticker.C)
}

func logUserCount(ctx context.Context, counter UserCounter, logger *slog.Logger, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			count, err := counter.Count(queryCtx)
			cancel()
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				logger.Error("count users failed", "error", err)
				continue
			}
			logger.Info("user count", "total", count)
		}
	}
}
