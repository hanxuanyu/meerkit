package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"meerkit/internal/core"
)

type MonitorMigrationTarget struct {
	ModuleVersion string
	ConfigVersion string
}
type MonitorConfigMigrator func(context.Context, string, string, string, json.RawMessage) (json.RawMessage, error)

const pluginSelectColumns = `id,version,name,vendor,desp,url,enabled,verified,official,trust_state,signer_key_id,signer_fingerprint,signer_public_key,status,error,package_path,binary_path,package_name,package_sha256,readme,manifest_json,modules_json,created_at,updated_at`

func (s *Store) UpsertPlugin(ctx context.Context, value core.PluginInstallation) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO plugins(`+pluginSelectColumns+`)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id,version) DO UPDATE SET name=excluded.name,vendor=excluded.vendor,desp=excluded.desp,url=excluded.url,enabled=excluded.enabled,verified=excluded.verified,official=excluded.official,trust_state=excluded.trust_state,signer_key_id=excluded.signer_key_id,signer_fingerprint=excluded.signer_fingerprint,signer_public_key=excluded.signer_public_key,status=excluded.status,error=excluded.error,package_path=excluded.package_path,binary_path=excluded.binary_path,package_name=excluded.package_name,package_sha256=excluded.package_sha256,readme=excluded.readme,manifest_json=excluded.manifest_json,modules_json=excluded.modules_json,updated_at=excluded.updated_at`, value.ID, value.Version, value.Name, value.Vendor, value.Description, value.URL, boolInt(value.Enabled), boolInt(value.Verified), boolInt(value.Official), value.TrustState, value.SignerKeyID, value.SignerFingerprint, value.SignerPublicKey, value.Status, value.Error, value.PackagePath, value.BinaryPath, value.PackageName, value.PackageSHA256, value.Readme, string(value.Manifest), jsonString(value.Modules), value.CreatedAt.UTC().Format(time.RFC3339Nano), value.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO plugin_trusted_signers(fingerprint,key_id,public_key,vendor,source,created_at,updated_at)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(fingerprint) DO UPDATE SET key_id=excluded.key_id,public_key=excluded.public_key,vendor=CASE WHEN excluded.vendor='' THEN plugin_trusted_signers.vendor ELSE excluded.vendor END,source=excluded.source,updated_at=excluded.updated_at`, signer.Fingerprint, signer.KeyID, signer.PublicKey, signer.Vendor, signer.Source, signer.CreatedAt.Format(time.RFC3339Nano), signer.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE plugins SET verified=1,trust_state=CASE WHEN official=1 THEN 'official' ELSE 'trusted' END,updated_at=? WHERE signer_fingerprint=?`, now.Format(time.RFC3339Nano), signer.Fingerprint)
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
	_, err = s.db.ExecContext(ctx, `INSERT INTO module_descriptor_snapshots(module_type,module_version,descriptor_json,created_at) VALUES(?,?,?,?) ON CONFLICT(module_type,module_version) DO UPDATE SET descriptor_json=excluded.descriptor_json`, descriptor.Type, descriptor.Version, string(data), time.Now().UTC().Format(time.RFC3339Nano))
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
