package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
	"meerkit/internal/core"
)

type MonitorMigrationTarget struct {
	ModuleVersion string
	ConfigVersion string
}
type MonitorConfigMigrator func(context.Context, string, string, string, json.RawMessage) (json.RawMessage, error)

const pluginSelectColumns = `id,version,name,vendor,desp,url,enabled,verified,official,trust_state,signer_key_id,signer_fingerprint,signer_public_key,status,error,package_path,binary_path,package_name,package_sha256,readme,manifest_json,modules_json,created_at,updated_at`

func (s *Store) UpsertPlugin(ctx context.Context, value core.PluginInstallation) error {
	return upsertPluginModel(ctx, s.orm, pluginFromDomain(value))
}

func (s *Store) ListPlugins(ctx context.Context) ([]core.PluginInstallation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+pluginSelectColumns+` FROM plugins ORDER BY name,id,version DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []core.PluginInstallation{}
	for rows.Next() {
		value, err := scanPlugin(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) GetPlugin(ctx context.Context, id, version string) (core.PluginInstallation, error) {
	if version == "" {
		return scanPlugin(s.db.QueryRowContext(ctx, `SELECT `+pluginSelectColumns+` FROM plugins WHERE id=? ORDER BY enabled DESC,version DESC LIMIT 1`, id))
	}
	return scanPlugin(s.db.QueryRowContext(ctx, `SELECT `+pluginSelectColumns+` FROM plugins WHERE id=? AND version=?`, id, version))
}

func scanPlugin(scanner interface{ Scan(...any) error }) (core.PluginInstallation, error) {
	var value core.PluginInstallation
	var enabled, verified, official int
	var manifest, modules, createdAt, updatedAt string
	if err := scanner.Scan(&value.ID, &value.Version, &value.Name, &value.Vendor, &value.Description, &value.URL, &enabled, &verified, &official, &value.TrustState, &value.SignerKeyID, &value.SignerFingerprint, &value.SignerPublicKey, &value.Status, &value.Error, &value.PackagePath, &value.BinaryPath, &value.PackageName, &value.PackageSHA256, &value.Readme, &manifest, &modules, &createdAt, &updatedAt); err != nil {
		return value, err
	}
	value.Enabled, value.Verified, value.Official = enabled == 1, verified == 1, official == 1
	value.Manifest = json.RawMessage(manifest)
	_ = json.Unmarshal([]byte(modules), &value.Modules)
	value.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	value.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return value, nil
}

func (s *Store) TrustPluginSigner(ctx context.Context, signer core.TrustedPluginSigner) error {
	now := time.Now().UTC()
	if signer.CreatedAt.IsZero() {
		signer.CreatedAt = now
	}
	signer.UpdatedAt = now
	tx, err := s.orm.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	model := &trustedPluginSignerModel{Fingerprint: signer.Fingerprint, KeyID: signer.KeyID, PublicKey: signer.PublicKey, Vendor: signer.Vendor, Source: signer.Source, CreatedAt: timestamp(signer.CreatedAt), UpdatedAt: timestamp(signer.UpdatedAt)}
	query := tx.NewUpdate().Model(model).Column("key_id", "public_key", "source", "updated_at").WherePK()
	if signer.Vendor != "" {
		query = query.Column("vendor")
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		if _, err = tx.NewInsert().Model(model).Ignore().Exec(ctx); err != nil {
			return err
		}
	}
	_, err = tx.NewUpdate().Model((*pluginModel)(nil)).Set("verified = ?", true).Set("trust_state = CASE WHEN official = ? THEN ? ELSE ? END", true, "official", "trusted").Set("updated_at = ?", timestamp(now)).Where("signer_fingerprint = ?", signer.Fingerprint).Exec(ctx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetTrustedPluginSigner(ctx context.Context, fingerprint string) (core.TrustedPluginSigner, error) {
	var signer core.TrustedPluginSigner
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `SELECT fingerprint,key_id,public_key,vendor,source,created_at,updated_at FROM plugin_trusted_signers WHERE fingerprint=?`, fingerprint).Scan(&signer.Fingerprint, &signer.KeyID, &signer.PublicKey, &signer.Vendor, &signer.Source, &createdAt, &updatedAt)
	if err != nil {
		return signer, err
	}
	signer.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	signer.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return signer, nil
}

func (s *Store) DeletePlugin(ctx context.Context, id, version string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM plugins WHERE id=? AND version=?`, id, version)
	return err
}
func (s *Store) DisableOtherPluginVersions(ctx context.Context, id, activeVersion string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE plugins SET enabled=0,status='installed',error='',updated_at=? WHERE id=? AND version<>? AND enabled=1`, time.Now().UTC().Format(time.RFC3339Nano), id, activeVersion)
	return err
}
func (s *Store) CountPlugins(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM plugins`).Scan(&count)
	return count, err
}

func (s *Store) DeleteDevelopmentPlugins(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM plugins WHERE trust_state='development'`)
	return err
}
func (s *Store) CountMonitorReferences(ctx context.Context, moduleTypes []string) (int, error) {
	if len(moduleTypes) == 0 {
		return 0, nil
	}
	query := `SELECT COUNT(*) FROM monitors WHERE module_type IN (`
	args := make([]any, len(moduleTypes))
	for index, value := range moduleTypes {
		if index > 0 {
			query += ","
		}
		query += "?"
		args[index] = value
	}
	query += ")"
	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}
func (s *Store) SaveDescriptorSnapshot(ctx context.Context, descriptor core.ModuleDescriptor) error {
	data, err := json.Marshal(descriptor)
	if err != nil {
		return err
	}
	model := &moduleDescriptorSnapshotModel{ModuleType: descriptor.Type, ModuleVersion: descriptor.Version, DescriptorJSON: string(data), CreatedAt: timestamp(time.Now())}
	result, err := s.orm.NewUpdate().Model(model).Column("descriptor_json").WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected > 0 {
		return nil
	}
	_, err = s.orm.NewInsert().Model(model).Ignore().Exec(ctx)
	return err
}
func (s *Store) GetDescriptorSnapshot(ctx context.Context, moduleType, version string) (core.ModuleDescriptor, error) {
	var data string
	if err := s.db.QueryRowContext(ctx, `SELECT descriptor_json FROM module_descriptor_snapshots WHERE module_type=? AND module_version=?`, moduleType, version).Scan(&data); err != nil {
		return core.ModuleDescriptor{}, err
	}
	var descriptor core.ModuleDescriptor
	err := json.Unmarshal([]byte(data), &descriptor)
	return descriptor, err
}
func (s *Store) MigrateMonitorConfigs(ctx context.Context, targets map[string]MonitorMigrationTarget, migrate MonitorConfigMigrator) error {
	if len(targets) == 0 {
		return nil
	}
	placeholders, args := "", make([]any, 0, len(targets))
	for moduleType := range targets {
		if placeholders != "" {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, moduleType)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,module_type,module_config_version,module_config_json FROM monitors WHERE module_type IN (`+placeholders+`)`, args...)
	if err != nil {
		return err
	}
	type item struct {
		id, moduleType, configVersion string
		config                        json.RawMessage
	}
	values := []item{}
	for rows.Next() {
		var value item
		var config string
		if err := rows.Scan(&value.id, &value.moduleType, &value.configVersion, &config); err != nil {
			rows.Close()
			return err
		}
		value.config = json.RawMessage(config)
		values = append(values, value)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, value := range values {
		target := targets[value.moduleType]
		config := value.config
		if value.configVersion != target.ConfigVersion {
			config, err = migrate(ctx, value.moduleType, value.configVersion, target.ConfigVersion, value.config)
			if err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE monitors SET module_version=?,module_config_version=?,module_config_json=?,updated_at=? WHERE id=?`, target.ModuleVersion, target.ConfigVersion, string(config), time.Now().UTC().Format(time.RFC3339Nano), value.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) IsNotFound(err error) bool { return err == sql.ErrNoRows }

func pluginFromDomain(value core.PluginInstallation) *pluginModel {
	return &pluginModel{
		ID: value.ID, Version: value.Version, Name: value.Name, Vendor: value.Vendor, Description: value.Description, URL: value.URL,
		Enabled: value.Enabled, Verified: value.Verified, Official: value.Official, TrustState: value.TrustState,
		SignerKeyID: value.SignerKeyID, SignerFingerprint: value.SignerFingerprint, SignerPublicKey: value.SignerPublicKey,
		Status: value.Status, Error: value.Error, PackagePath: value.PackagePath, BinaryPath: value.BinaryPath,
		PackageName: value.PackageName, PackageSHA256: value.PackageSHA256, Readme: value.Readme,
		ManifestJSON: string(value.Manifest), ModulesJSON: jsonString(value.Modules), CreatedAt: timestamp(value.CreatedAt), UpdatedAt: timestamp(value.UpdatedAt),
	}
}

func upsertPluginModel(ctx context.Context, db bun.IDB, model *pluginModel) error {
	columns := []string{"name", "vendor", "desp", "url", "enabled", "verified", "official", "trust_state", "signer_key_id", "signer_fingerprint", "signer_public_key", "status", "error", "package_path", "binary_path", "package_name", "package_sha256", "readme", "manifest_json", "modules_json", "updated_at"}
	result, err := db.NewUpdate().Model(model).Column(columns...).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected > 0 {
		return nil
	}
	if _, err := db.NewInsert().Model(model).Exec(ctx); err != nil {
		// A concurrent insert may win between UPDATE and INSERT.
		if _, retryErr := db.NewUpdate().Model(model).Column(columns...).WherePK().Exec(ctx); retryErr == nil {
			return nil
		}
		return err
	}
	return nil
}
