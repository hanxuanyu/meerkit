package runtime

import (
	"context"
	"log/slog"
	"time"

	"meerkit/internal/app"
	"meerkit/internal/store"
)

type CleanupWorker struct {
	store                 *store.Store
	interval              time.Duration
	recordRetention       time.Duration
	notificationRetention time.Duration
	logger                *slog.Logger
	onNotificationsPruned func(int)
}

func NewCleanupWorker(database *store.Store, config app.Config, logger *slog.Logger, onNotificationsPruned func(int)) *CleanupWorker {
	return &CleanupWorker{
		store: database, interval: config.CleanupIntervalDuration(), recordRetention: config.RetentionDuration(),
		notificationRetention: config.NotificationRetentionDuration(), logger: logger, onNotificationsPruned: onNotificationsPruned,
	}
}

func (w *CleanupWorker) Start(ctx context.Context) {
	if w.logger != nil {
		w.logger.Info("storage cleanup started", "interval", w.interval.String(), "record_retention", w.recordRetention.String(), "notification_retention", w.notificationRetention.String())
	}
	w.run(ctx, time.Now())
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	defer func() {
		if w.logger != nil {
			w.logger.Info("storage cleanup stopped")
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			w.run(ctx, now)
		}
	}
}

func (w *CleanupWorker) run(ctx context.Context, now time.Time) {
	recordBefore := now.Add(-w.recordRetention)
	recordsDeleted, recordErr := w.store.PruneRecords(ctx, recordBefore)
	if recordErr != nil {
		if w.logger != nil {
			w.logger.Error("prune monitor records failed", "error", recordErr)
		}
	} else if w.logger != nil {
		w.logger.Info("monitor records pruned", "deleted", recordsDeleted, "before", recordBefore)
	}

	notificationBefore := now.Add(-w.notificationRetention)
	notificationsDeleted, notificationErr := w.store.PruneInAppNotifications(ctx, notificationBefore)
	if notificationErr != nil {
		if w.logger != nil {
			w.logger.Error("prune in-app notifications failed", "error", notificationErr)
		}
		return
	}
	if w.logger != nil {
		w.logger.Info("in-app notifications pruned", "deleted", notificationsDeleted, "before", notificationBefore)
	}
	if notificationsDeleted > 0 && w.onNotificationsPruned != nil {
		unreadCount, err := w.store.CountUnreadInAppNotifications(ctx)
		if err != nil {
			if w.logger != nil {
				w.logger.Error("count unread notifications after cleanup failed", "error", err)
			}
			return
		}
		w.onNotificationsPruned(unreadCount)
	}
}
