package runtime

import (
	"context"
	"log/slog"
	"time"

	"meerkit/internal/app"
	"meerkit/internal/store"
)

type CleanupWorker struct {
	store                 store.CleanupRepository
	config                func() app.RuntimeConfig
	logger                *slog.Logger
	onNotificationsPruned func(int)
	onRecordsPruned       func(int)
	changes               <-chan struct{}
}

func (w *CleanupWorker) SetRecordsPruned(callback func(int)) { w.onRecordsPruned = callback }

func NewCleanupWorker(database store.CleanupRepository, config func() app.RuntimeConfig, logger *slog.Logger, onNotificationsPruned func(int), changes ...<-chan struct{}) *CleanupWorker {
	var changeChannel <-chan struct{}
	if len(changes) > 0 {
		changeChannel = changes[0]
	}
	return &CleanupWorker{store: database, config: config, logger: logger, onNotificationsPruned: onNotificationsPruned, changes: changeChannel}
}

func (w *CleanupWorker) Start(ctx context.Context) {
	current := w.config()
	if w.logger != nil {
		retention, notificationRetention, interval := current.StorageDurations()
		w.logger.Info("storage cleanup started", "interval", interval.String(), "record_retention", retention.String(), "notification_retention", notificationRetention.String())
	}
	w.run(ctx, time.Now())
	_, _, interval := current.StorageDurations()
	if interval <= 0 {
		interval = time.Hour
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	defer func() {
		if w.logger != nil {
			w.logger.Info("storage cleanup stopped")
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.changes:
			_, _, interval := w.config().StorageDurations()
			if interval <= 0 {
				interval = time.Hour
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(interval)
		case now := <-timer.C:
			w.run(ctx, now)
			_, _, interval := w.config().StorageDurations()
			if interval <= 0 {
				interval = time.Hour
			}
			timer.Reset(interval)
		}
	}
}

func (w *CleanupWorker) run(ctx context.Context, now time.Time) {
	retention, notificationRetention, _ := w.config().StorageDurations()
	recordBefore := now.Add(-retention)
	recordsDeleted, recordErr := w.store.PruneRecords(ctx, recordBefore)
	if recordErr != nil {
		if w.logger != nil {
			w.logger.Error("prune monitor records failed", "error", recordErr)
		}
	} else if w.logger != nil {
		w.logger.Info("monitor records pruned", "deleted", recordsDeleted, "before", recordBefore)
	}
	if recordsDeleted > 0 && w.onRecordsPruned != nil {
		w.onRecordsPruned(int(recordsDeleted))
	}

	notificationBefore := now.Add(-notificationRetention)
	notificationsDeleted, notificationErr := w.store.PruneNotificationDeliveries(ctx, notificationBefore)
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
