package store

import (
	"context"
	"fmt"
	"time"
)

type schemaMigration struct {
	version int64
	name    string
	up      func(context.Context) error
}

func (s *Store) schemaMigrations() []schemaMigration {
	return []schemaMigration{{version: 1, name: "initial cross-database schema", up: s.createInitialSchema}}
}

func (s *Store) migrate(ctx context.Context) error {
	unlock, err := s.acquireMigrationLock(ctx)
	if err != nil {
		return err
	}
	defer unlock()

	if _, err := s.orm.NewCreateTable().Model((*schemaMigrationModel)(nil)).IfNotExists().Exec(ctx); err != nil {
		return err
	}
	for index, migration := range s.schemaMigrations() {
		count, err := s.orm.NewSelect().Model((*schemaMigrationModel)(nil)).Where("version = ?", migration.version).Count(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if index == 0 {
			if err := s.optimizeMySQLTable(ctx, "meerkit_schema_migrations"); err != nil {
				return err
			}
		}
		if err := migration.up(ctx); err != nil {
			return err
		}
		model := &schemaMigrationModel{Version: migration.version, Name: migration.name, AppliedAt: timestamp(time.Now())}
		if _, err := s.orm.NewInsert().Model(model).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) validateSchema(ctx context.Context) error {
	for _, migration := range s.schemaMigrations() {
		count, err := s.orm.NewSelect().Model((*schemaMigrationModel)(nil)).Where("version = ?", migration.version).Count(ctx)
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("schema version %d is not installed; enable auto_migrate", migration.version)
		}
	}
	return nil
}

func (s *Store) createInitialSchema(ctx context.Context) error {
	if s.databaseType == DatabaseMySQL {
		if _, err := s.db.ExecContext(ctx, "SET default_storage_engine=InnoDB"); err != nil {
			return err
		}
	}
	models := []struct {
		name  string
		model any
	}{
		{name: "monitors", model: (*monitorModel)(nil)},
		{name: "monitor_records", model: (*monitorRecordModel)(nil)},
		{name: "status_board_items", model: (*statusBoardItemModel)(nil)},
		{name: "status_board_shares", model: (*statusBoardShareModel)(nil)},
		{name: "notification_channels", model: (*notificationChannelModel)(nil)},
		{name: "notification_deliveries", model: (*notificationDeliveryModel)(nil)},
		{name: "plugins", model: (*pluginModel)(nil)},
		{name: "plugin_trusted_signers", model: (*trustedPluginSignerModel)(nil)},
		{name: "module_descriptor_snapshots", model: (*moduleDescriptorSnapshotModel)(nil)},
		{name: "system_configs", model: (*systemConfigModel)(nil)},
		{name: "admin_sessions", model: (*adminSessionModel)(nil)},
		{name: "api_tokens", model: (*apiTokenModel)(nil)},
	}
	for _, value := range models {
		query := s.orm.NewCreateTable().Model(value.model).IfNotExists()
		if s.databaseType == DatabaseSQLite {
			query = query.WithForeignKeys()
		}
		if _, err := query.Exec(ctx); err != nil {
			return err
		}
		if err := s.optimizeMySQLTable(ctx, value.name); err != nil {
			return err
		}
	}
	if err := s.ensureMySQLForeignKeys(ctx); err != nil {
		return err
	}
	indexes := []struct {
		name    string
		model   any
		columns []string
		unique  bool
	}{
		{name: "idx_monitors_module_type", model: (*monitorModel)(nil), columns: []string{"module_type"}},
		{name: "idx_monitors_status", model: (*monitorModel)(nil), columns: []string{"enabled", "condition_active", "last_success"}},
		{name: "idx_monitor_records_monitor_time", model: (*monitorRecordModel)(nil), columns: []string{"monitor_id", "started_at"}},
		{name: "idx_monitor_records_status_time", model: (*monitorRecordModel)(nil), columns: []string{"monitor_id", "success", "started_at"}},
		{name: "idx_monitor_records_event_time", model: (*monitorRecordModel)(nil), columns: []string{"monitor_id", "event_type", "started_at"}},
		{name: "idx_status_board_items_monitor_created", model: (*statusBoardItemModel)(nil), columns: []string{"monitor_id", "created_at"}},
		{name: "idx_status_board_shares_token", model: (*statusBoardShareModel)(nil), columns: []string{"token"}, unique: true},
		{name: "idx_notification_channels_builtin_key", model: (*notificationChannelModel)(nil), columns: []string{"builtin_key"}, unique: true},
		{name: "idx_notification_deliveries_event_channel", model: (*notificationDeliveryModel)(nil), columns: []string{"event_id", "channel_id"}, unique: true},
		{name: "idx_notification_deliveries_record", model: (*notificationDeliveryModel)(nil), columns: []string{"record_id", "created_at"}},
		{name: "idx_notification_deliveries_status", model: (*notificationDeliveryModel)(nil), columns: []string{"status", "created_at"}},
		{name: "idx_notification_deliveries_inapp", model: (*notificationDeliveryModel)(nil), columns: []string{"notifier_type", "is_read", "created_at"}},
		{name: "idx_plugins_signer_fingerprint", model: (*pluginModel)(nil), columns: []string{"signer_fingerprint"}},
		{name: "idx_admin_sessions_expiry", model: (*adminSessionModel)(nil), columns: []string{"expires_at"}},
		{name: "idx_api_tokens_name", model: (*apiTokenModel)(nil), columns: []string{"name"}, unique: true},
		{name: "idx_api_tokens_type", model: (*apiTokenModel)(nil), columns: []string{"type"}},
		{name: "idx_api_tokens_expires", model: (*apiTokenModel)(nil), columns: []string{"expires_at"}},
		{name: "idx_api_tokens_revoked", model: (*apiTokenModel)(nil), columns: []string{"revoked_at"}},
	}
	for _, index := range indexes {
		exists, err := s.indexExists(ctx, index.name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		query := s.orm.NewCreateIndex().Model(index.model).Index(index.name).Column(index.columns...)
		if index.unique {
			query = query.Unique()
		}
		if _, err := query.Exec(ctx); err != nil {
			return err
		}
	}
	return s.seedBuiltInChannel(ctx)
}

func (s *Store) ensureMySQLForeignKeys(ctx context.Context) error {
	if s.databaseType != DatabaseMySQL {
		return nil
	}
	constraints := []struct {
		name  string
		table string
		sql   string
	}{
		{name: "fk_monitor_records_monitor", table: "monitor_records", sql: "ALTER TABLE `monitor_records` ADD CONSTRAINT `fk_monitor_records_monitor` FOREIGN KEY (`monitor_id`) REFERENCES `monitors` (`id`) ON DELETE CASCADE"},
		{name: "fk_status_board_items_monitor", table: "status_board_items", sql: "ALTER TABLE `status_board_items` ADD CONSTRAINT `fk_status_board_items_monitor` FOREIGN KEY (`monitor_id`) REFERENCES `monitors` (`id`) ON DELETE CASCADE"},
	}
	for _, constraint := range constraints {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name=? AND constraint_name=?`, constraint.table, constraint.name).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if _, err := s.db.ExecContext(ctx, constraint.sql); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) optimizeMySQLTable(ctx context.Context, table string) error {
	if s.databaseType != DatabaseMySQL {
		return nil
	}
	// Names are compile-time migration constants, never user input.
	_, err := s.db.ExecContext(ctx, "ALTER TABLE `"+table+"` ENGINE=InnoDB, CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	return err
}

func (s *Store) indexExists(ctx context.Context, name string) (bool, error) {
	var count int
	var err error
	if s.databaseType == DatabaseMySQL {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND index_name=?`, name).Scan(&count)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&count)
	}
	return count > 0, err
}

func (s *Store) seedBuiltInChannel(ctx context.Context) error {
	key := "inapp"
	now := timestamp(time.Now())
	model := &notificationChannelModel{
		ID:           "builtin-inapp",
		BuiltinKey:   &key,
		Name:         "站内通知",
		NotifierType: "inapp",
		Enabled:      true,
		ConfigJSON:   `{"title_template":"{{monitor.name}} · {{event.type}}","body_template":"{{event.summary}}"}`,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_, err := s.orm.NewInsert().Model(model).Ignore().Exec(ctx)
	return err
}

func (s *Store) acquireMigrationLock(ctx context.Context) (func(), error) {
	if s.databaseType != DatabaseMySQL {
		return func() {}, nil
	}
	connection, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var acquired int
	if err := connection.QueryRowContext(ctx, `SELECT GET_LOCK('meerkit_schema_migration', 30)`).Scan(&acquired); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if acquired != 1 {
		_ = connection.Close()
		return nil, fmt.Errorf("could not acquire mysql migration lock")
	}
	return func() {
		var released int
		_ = connection.QueryRowContext(context.Background(), `SELECT RELEASE_LOCK('meerkit_schema_migration')`).Scan(&released)
		_ = connection.Close()
	}, nil
}

func timestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func unixMicros(value time.Time) int64 {
	return value.UTC().UnixMicro()
}
