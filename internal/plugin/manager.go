package plugin

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hanxuanyu/meerkit/sdk"
	"github.com/hashicorp/go-hclog"
	hplugin "github.com/hashicorp/go-plugin"
	"go.yaml.in/yaml/v3"
	"google.golang.org/grpc/health/grpc_health_v1"
	"meerkit/internal/core"
	"meerkit/internal/monitor"
	"meerkit/internal/store"
)

type ImportOptions struct {
	Enable          bool
	Official        bool
	AllowUnverified bool
}

const (
	trustStateOfficial    = "official"
	trustStateTrusted     = "trusted"
	trustStateUntrusted   = "untrusted"
	trustStateUnsigned    = "unsigned"
	trustStateDevelopment = "development"
)

var ErrNoDevelopmentPlugins = errors.New("no development plugins found")

type signatureInfo struct {
	Signed      bool
	KeyID       string
	PublicKey   string
	Fingerprint string
}

type ManagerOptions struct {
	DataDir     string
	TrustedKeys map[string]ed25519.PublicKey
	Logger      *slog.Logger
	LogLevel    string
	LogFormat   string
}

type process struct {
	client  *hplugin.Client
	gate    *monitor.ExecutionGate
	version string
	logFile *os.File
}
type Manager struct {
	store              store.PluginRepository
	registry           *monitor.Registry
	root               string
	logger             *slog.Logger
	pluginLogLevel     string
	pluginLogFormat    string
	mu                 sync.Mutex
	processes          map[string]*process
	watcher            *fsnotify.Watcher
	watchCancel        context.CancelFunc
	developmentBuilder func(context.Context, string, string) error
}

func NewManager(database store.PluginRepository, registry *monitor.Registry, options ManagerOptions) (*Manager, error) {
	dataDir, err := filepath.Abs(options.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve plugin data directory: %w", err)
	}
	pluginLogLevel := strings.TrimSpace(options.LogLevel)
	if pluginLogLevel == "" {
		pluginLogLevel = "info"
	}
	pluginLogFormat := strings.TrimSpace(options.LogFormat)
	if pluginLogFormat == "" {
		pluginLogFormat = "simple"
	}
	manager := &Manager{store: database, registry: registry, root: filepath.Join(dataDir, "plugins"), logger: options.Logger, pluginLogLevel: pluginLogLevel, pluginLogFormat: pluginLogFormat, processes: make(map[string]*process), developmentBuilder: buildDevelopmentPlugin}
	for _, directory := range []string{"inbox", "staging", "packages", "installed", "development", "rejected", "logs"} {
		if err := os.MkdirAll(filepath.Join(manager.root, directory), 0o750); err != nil {
			return nil, err
		}
	}
	for keyID, publicKey := range options.TrustedKeys {
		encoded := base64.StdEncoding.EncodeToString(publicKey)
		signer := core.TrustedPluginSigner{Fingerprint: publicKeyFingerprint(publicKey), KeyID: keyID, PublicKey: encoded, Source: "config"}
		if err := database.TrustPluginSigner(context.Background(), signer); err != nil {
			return nil, err
		}
	}
	return manager, nil
}

func (m *Manager) Root() string { return m.root }
func (m *Manager) List(ctx context.Context) ([]core.PluginInstallation, error) {
	return m.store.ListPlugins(ctx)
}

func (m *Manager) UpdateLogConfig(ctx context.Context, level, format string) error {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "warning" {
		level = "warn"
	}
	if level != "debug" && level != "info" && level != "warn" && level != "error" {
		return fmt.Errorf("plugins.log_level must be debug, info, warn, or error")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "text" && format != "simple" && format != "json" {
		return fmt.Errorf("plugins.log_format must be text, simple, or json")
	}
	m.mu.Lock()
	m.pluginLogLevel, m.pluginLogFormat = level, format
	active := make(map[string]string, len(m.processes))
	for id, value := range m.processes {
		active[id] = value.version
	}
	m.mu.Unlock()
	for id, version := range active {
		if err := m.stopActive(id); err != nil {
			return err
		}
		if err := m.Enable(ctx, id, version, true); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Details(ctx context.Context, id, version string) (core.PluginDetails, error) {
	value, err := m.store.GetPlugin(ctx, id, version)
	if err != nil {
		return core.PluginDetails{}, err
	}
	descriptors := make([]core.ModuleDescriptor, 0, len(value.Modules))
	for _, module := range value.Modules {
		descriptor, descriptorErr := m.store.GetDescriptorSnapshot(ctx, module.Type, module.Version)
		if errors.Is(descriptorErr, sql.ErrNoRows) {
			continue
		}
		if descriptorErr != nil {
			return core.PluginDetails{}, fmt.Errorf("load descriptor for plugin module %s: %w", module.Type, descriptorErr)
		}
		descriptors = append(descriptors, descriptor)
	}
	return core.PluginDetails{PluginInstallation: value, Readme: value.Readme, ModuleDescriptors: descriptors}, nil
}

func (m *Manager) Import(ctx context.Context, archivePath string, options ImportOptions) (core.PluginInstallation, error) {
	if m.logger != nil {
		m.logger.Info("plugin import started", "package", filepath.Base(archivePath), "official", options.Official, "enable", options.Enable)
	}
	if !isArchive(archivePath) {
		return core.PluginInstallation{}, fmt.Errorf("plugin package must be a .zip or .tar.gz archive")
	}
	stage := filepath.Join(m.root, "staging", core.NewID())
	if err := os.MkdirAll(stage, 0o750); err != nil {
		return core.PluginInstallation{}, err
	}
	defer os.RemoveAll(stage)
	if err := extractArchive(archivePath, stage); err != nil {
		return core.PluginInstallation{}, fmt.Errorf("inspect plugin package: %w", err)
	}
	manifestPath := filepath.Join(stage, "meerkit-plugin.yaml")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return core.PluginInstallation{}, fmt.Errorf("read meerkit-plugin.yaml: %w", err)
	}
	var manifest Manifest
	if err := yaml.Unmarshal(manifestBytes, &manifest); err != nil {
		return core.PluginInstallation{}, fmt.Errorf("decode plugin manifest: %w", err)
	}
	if err := manifest.Validate(sdk.ProtocolVersion); err != nil {
		return core.PluginInstallation{}, err
	}
	signature, err := inspectSignature(stage, manifestBytes)
	if err != nil {
		return core.PluginInstallation{}, fmt.Errorf("verify plugin signature: %w", err)
	}
	readme, err := readPackageReadme(stage)
	if err != nil {
		return core.PluginInstallation{}, err
	}
	artifact, err := manifest.CurrentArtifact()
	if err != nil {
		return core.PluginInstallation{}, err
	}
	binarySource := filepath.Join(stage, filepath.FromSlash(artifact.Path))
	artifactHash, artifactSize, err := hashFile(binarySource)
	if err != nil {
		return core.PluginInstallation{}, fmt.Errorf("read plugin artifact: %w", err)
	}
	if artifactSize != artifact.Size || !strings.EqualFold(artifactHash, artifact.SHA256) {
		return core.PluginInstallation{}, fmt.Errorf("plugin artifact hash or size does not match manifest")
	}
	packageHash, _, err := hashFile(archivePath)
	if err != nil {
		return core.PluginInstallation{}, err
	}
	if existing, getErr := m.store.GetPlugin(ctx, manifest.ID, manifest.Version); getErr == nil {
		if existing.PackageSHA256 == packageHash {
			if options.Enable && !existing.Enabled {
				if err := m.Enable(ctx, existing.ID, existing.Version, options.AllowUnverified || options.Official); err != nil {
					return existing, err
				}
				return m.store.GetPlugin(ctx, existing.ID, existing.Version)
			}
			return existing, nil
		}
		return core.PluginInstallation{}, fmt.Errorf("plugin %s version %s is already installed with different contents", manifest.ID, manifest.Version)
	} else if !errors.Is(getErr, sql.ErrNoRows) {
		return core.PluginInstallation{}, getErr
	}
	verified := options.Official
	trustState := trustStateUnsigned
	if options.Official {
		trustState = trustStateOfficial
	}
	if signature.Signed {
		_, trustErr := m.store.GetTrustedPluginSigner(ctx, signature.Fingerprint)
		trusted := trustErr == nil
		if trustErr != nil && !errors.Is(trustErr, sql.ErrNoRows) {
			return core.PluginInstallation{}, trustErr
		}
		if options.Official && !trusted {
			signer := core.TrustedPluginSigner{Fingerprint: signature.Fingerprint, KeyID: signature.KeyID, PublicKey: signature.PublicKey, Vendor: manifest.Vendor, Source: "official"}
			if err := m.store.TrustPluginSigner(ctx, signer); err != nil {
				return core.PluginInstallation{}, err
			}
			trusted = true
		}
		if trusted {
			verified = true
			if !options.Official {
				trustState = trustStateTrusted
			}
		} else {
			trustState = trustStateUntrusted
		}
	}
	packageDir := filepath.Join(m.root, "packages", manifest.ID, manifest.Version)
	installDir := filepath.Join(m.root, "installed", manifest.ID, manifest.Version, runtime.GOOS+"-"+runtime.GOARCH)
	if err := os.MkdirAll(packageDir, 0o750); err != nil {
		return core.PluginInstallation{}, err
	}
	if err := os.MkdirAll(installDir, 0o750); err != nil {
		return core.PluginInstallation{}, err
	}
	packagePath := filepath.Join(packageDir, filepath.Base(archivePath))
	if err := atomicCopy(archivePath, packagePath, 0o600); err != nil {
		return core.PluginInstallation{}, err
	}
	binaryName := "plugin"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(installDir, binaryName)
	if err := atomicCopy(binarySource, binaryPath, 0o700); err != nil {
		return core.PluginInstallation{}, err
	}
	manifestJSON, _ := json.Marshal(manifest)
	now := time.Now().UTC()
	installation := core.PluginInstallation{ID: manifest.ID, Name: manifest.Name, Version: manifest.Version, Vendor: manifest.Vendor, Description: manifest.Description, URL: manifest.URL, Enabled: false, Verified: verified, Official: options.Official, TrustState: trustState, SignerKeyID: signature.KeyID, SignerFingerprint: signature.Fingerprint, SignerPublicKey: signature.PublicKey, Status: "installed", PackagePath: packagePath, BinaryPath: binaryPath, Readme: readme, PackageName: filepath.Base(packagePath), PackageSHA256: packageHash, Manifest: manifestJSON, Modules: manifest.Modules, CreatedAt: now, UpdatedAt: now}
	if err := m.store.UpsertPlugin(ctx, installation); err != nil {
		return core.PluginInstallation{}, err
	}
	if m.logger != nil {
		m.logger.Info("plugin installed", "plugin_id", installation.ID, "name", installation.Name, "version", installation.Version, "trust_state", installation.TrustState, "verified", installation.Verified)
	}
	if options.Enable {
		if err := m.Enable(ctx, manifest.ID, manifest.Version, options.AllowUnverified || options.Official); err != nil {
			return installation, err
		}
		return m.store.GetPlugin(ctx, manifest.ID, manifest.Version)
	}
	return installation, nil
}

func (m *Manager) Enable(ctx context.Context, id, version string, allowUnverified bool) error {
	installation, err := m.store.GetPlugin(ctx, id, version)
	if err != nil {
		return err
	}
	if installation.TrustState == trustStateUntrusted {
		return fmt.Errorf("plugin %s has a signed but untrusted publisher; trust its signing fingerprint before enabling", id)
	}
	if !installation.Verified && !allowUnverified {
		return fmt.Errorf("plugin %s is unverified; explicit risk confirmation is required", id)
	}
	if m.logger != nil {
		m.logger.Info("plugin activation started", "plugin_id", id, "name", installation.Name, "version", installation.Version, "log_format", m.pluginLogFormat, "log_level", m.pluginLogLevel)
	}
	logPath := filepath.Join(m.root, "logs", id+"-"+version+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return m.markFailure(ctx, installation, err)
	}
	command, err := commandForInstallation(installation)
	if err != nil {
		_ = logFile.Close()
		return m.markFailure(ctx, installation, err)
	}
	command.Env = append(os.Environ(),
		"MEERKIT_PLUGIN_ID="+installation.ID,
		"MEERKIT_PLUGIN_NAME="+installation.Name,
		"MEERKIT_PLUGIN_VERSION="+installation.Version,
		"MEERKIT_PLUGIN_LOG_LEVEL="+m.pluginLogLevel,
		"MEERKIT_PLUGIN_LOG_FORMAT="+m.pluginLogFormat,
	)
	client := hplugin.NewClient(&hplugin.ClientConfig{HandshakeConfig: sdk.Handshake, Plugins: map[string]hplugin.Plugin{"monitor": &sdk.MonitorPlugin{}}, Cmd: command, AllowedProtocols: []hplugin.Protocol{hplugin.ProtocolGRPC}, Managed: true, Stderr: logFile, SyncStdout: logFile, SyncStderr: logFile, Logger: hclog.NewNullLogger()})
	cleanupCandidate := func() { client.Kill(); _ = logFile.Close() }
	rpcClient, err := client.Client()
	if err != nil {
		cleanupCandidate()
		return m.markFailure(ctx, installation, err)
	}
	if m.logger != nil {
		m.logger.Info("plugin process connected", "plugin_id", id, "version", installation.Version)
	}
	grpcClient, ok := rpcClient.(*hplugin.GRPCClient)
	if !ok {
		cleanupCandidate()
		return m.markFailure(ctx, installation, fmt.Errorf("plugin negotiated an incompatible RPC protocol"))
	}
	standardHealthCtx, standardHealthCancel := context.WithTimeout(ctx, 5*time.Second)
	standardHealth, err := grpc_health_v1.NewHealthClient(grpcClient.Conn).Check(standardHealthCtx, &grpc_health_v1.HealthCheckRequest{Service: hplugin.GRPCServiceName})
	standardHealthCancel()
	if err != nil || standardHealth.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		cleanupCandidate()
		if err != nil {
			return m.markFailure(ctx, installation, fmt.Errorf("standard gRPC health check: %w", err))
		}
		return m.markFailure(ctx, installation, fmt.Errorf("standard gRPC health status is %s, want SERVING", standardHealth.Status))
	}
	dispensed, err := rpcClient.Dispense("monitor")
	if err != nil {
		cleanupCandidate()
		return m.markFailure(ctx, installation, err)
	}
	provider, ok := dispensed.(sdk.Provider)
	if !ok {
		cleanupCandidate()
		return m.markFailure(ctx, installation, fmt.Errorf("plugin returned an incompatible client"))
	}
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = provider.Health(healthCtx)
	cancel()
	if err != nil {
		cleanupCandidate()
		return m.markFailure(ctx, installation, err)
	}
	if m.logger != nil {
		m.logger.Info("plugin health check passed", "plugin_id", id, "version", installation.Version)
	}
	descriptors, err := provider.ListModules()
	if err != nil {
		cleanupCandidate()
		return m.markFailure(ctx, installation, err)
	}
	if err := verifyDescriptors(installation.Modules, descriptors); err != nil {
		cleanupCandidate()
		return m.markFailure(ctx, installation, err)
	}
	declaredModules := make(map[string]core.PluginModule, len(installation.Modules))
	for _, module := range installation.Modules {
		declaredModules[module.Type] = module
	}
	for index := range descriptors {
		declared := declaredModules[descriptors[index].Type]
		descriptors[index].ConfigVersion = declared.ConfigVersion
		descriptors[index].ResultSchemaVersion = declared.ResultSchemaVersion
	}
	gate := monitor.NewExecutionGate()
	modules := make([]core.MonitorModule, 0, len(descriptors))
	moduleTypes := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		module, err := monitor.NewRemoteModule(provider, descriptor, gate)
		if err != nil {
			cleanupCandidate()
			return m.markFailure(ctx, installation, err)
		}
		modules = append(modules, module)
		moduleTypes = append(moduleTypes, descriptor.Type)
	}
	if m.logger != nil {
		m.logger.Info("plugin modules registered", "plugin_id", id, "version", installation.Version, "module_count", len(moduleTypes), "modules", moduleTypes)
	}
	if err := m.registry.ValidateReplaceOwner(id, moduleTypes); err != nil {
		cleanupCandidate()
		return m.markFailure(ctx, installation, err)
	}
	for _, module := range modules {
		if err := m.store.SaveDescriptorSnapshot(ctx, module.Descriptor()); err != nil {
			cleanupCandidate()
			return m.markFailure(ctx, installation, err)
		}
	}
	m.mu.Lock()
	previous := m.processes[id]
	m.mu.Unlock()
	if previous != nil {
		previous.gate.Stop()
		previous.gate.Wait()
	}
	targets := make(map[string]store.MonitorMigrationTarget, len(installation.Modules))
	for _, module := range installation.Modules {
		targets[module.Type] = store.MonitorMigrationTarget{ModuleVersion: module.Version, ConfigVersion: module.ConfigVersion}
	}
	if err := m.store.MigrateMonitorConfigs(ctx, targets, func(callCtx context.Context, moduleType, fromVersion, toVersion string, config json.RawMessage) (json.RawMessage, error) {
		return provider.MigrateConfig(callCtx, moduleType, fromVersion, toVersion, config)
	}); err != nil {
		if previous != nil {
			previous.gate.Start()
		}
		cleanupCandidate()
		return m.markFailure(ctx, installation, err)
	}
	if err := m.registry.ReplaceOwner(id, modules); err != nil {
		if previous != nil {
			previous.gate.Start()
		}
		cleanupCandidate()
		return m.markFailure(ctx, installation, err)
	}
	m.mu.Lock()
	m.processes[id] = &process{client: client, gate: gate, version: installation.Version, logFile: logFile}
	m.mu.Unlock()
	if previous != nil {
		previous.gate.Stop()
		previous.gate.Wait()
		previous.client.Kill()
		if previous.logFile != nil {
			_ = previous.logFile.Close()
		}
	}
	installation.Enabled, installation.Status, installation.Error, installation.UpdatedAt = true, "healthy", "", time.Now().UTC()
	if err := m.store.DisableOtherPluginVersions(ctx, id, installation.Version); err != nil {
		return err
	}
	if err := m.store.UpsertPlugin(ctx, installation); err != nil {
		_ = m.stopActive(id)
		return err
	}
	if m.logger != nil {
		m.logger.Info("plugin activated", "plugin_id", id, "name", installation.Name, "version", installation.Version, "module_count", len(moduleTypes), "status", installation.Status)
	}
	return nil
}

func (m *Manager) TrustPublisher(ctx context.Context, id, version, fingerprint string) (core.PluginInstallation, error) {
	installation, err := m.store.GetPlugin(ctx, id, version)
	if err != nil {
		return core.PluginInstallation{}, err
	}
	if installation.SignerFingerprint == "" || installation.SignerPublicKey == "" {
		return core.PluginInstallation{}, fmt.Errorf("plugin %s is unsigned and has no publisher identity to trust", id)
	}
	if fingerprint != installation.SignerFingerprint {
		return core.PluginInstallation{}, fmt.Errorf("signer fingerprint changed; reload plugin details and verify it again")
	}
	publicKey, err := base64.StdEncoding.DecodeString(installation.SignerPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || publicKeyFingerprint(publicKey) != fingerprint {
		return core.PluginInstallation{}, fmt.Errorf("stored plugin signer identity is invalid")
	}
	signer := core.TrustedPluginSigner{Fingerprint: fingerprint, KeyID: installation.SignerKeyID, PublicKey: installation.SignerPublicKey, Vendor: installation.Vendor, Source: "user"}
	if err := m.store.TrustPluginSigner(ctx, signer); err != nil {
		return core.PluginInstallation{}, err
	}
	return m.store.GetPlugin(ctx, id, version)
}

func (m *Manager) Disable(ctx context.Context, id, version string) error {
	if m.logger != nil {
		m.logger.Info("plugin disable started", "plugin_id", id, "version", version)
	}
	if err := m.stopActive(id); err != nil {
		return err
	}
	installation, err := m.store.GetPlugin(ctx, id, version)
	if err != nil {
		return err
	}
	installation.Enabled, installation.Status, installation.Error, installation.UpdatedAt = false, "disabled", "", time.Now().UTC()
	if err := m.store.UpsertPlugin(ctx, installation); err != nil {
		return err
	}
	if m.logger != nil {
		m.logger.Info("plugin disabled", "plugin_id", id, "version", version, "status", installation.Status)
	}
	return nil
}
func (m *Manager) stopActive(id string) error {
	m.registry.RemoveOwner(id)
	m.mu.Lock()
	active := m.processes[id]
	delete(m.processes, id)
	m.mu.Unlock()
	if active != nil {
		if m.logger != nil {
			m.logger.Info("plugin process stopping", "plugin_id", id, "version", active.version)
		}
		active.gate.Stop()
		active.gate.Wait()
		active.client.Kill()
		if active.logFile != nil {
			_ = active.logFile.Close()
		}
		if m.logger != nil {
			m.logger.Info("plugin process stopped", "plugin_id", id, "version", active.version)
		}
	}
	return nil
}

func (m *Manager) Uninstall(ctx context.Context, id, version string) error {
	installation, err := m.store.GetPlugin(ctx, id, version)
	if err != nil {
		return err
	}
	types := make([]string, 0, len(installation.Modules))
	for _, module := range installation.Modules {
		types = append(types, module.Type)
	}
	count, err := m.store.CountMonitorReferences(ctx, types)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("plugin is referenced by %d monitor(s)", count)
	}
	_ = m.stopActive(id)
	if err := m.store.DeletePlugin(ctx, id, version); err != nil {
		return err
	}
	_ = os.RemoveAll(filepath.Join(m.root, "installed", id, version))
	_ = os.RemoveAll(filepath.Join(m.root, "packages", id, version))
	if m.logger != nil {
		m.logger.Info("plugin uninstalled", "plugin_id", id, "version", version)
	}
	return nil
}

func (m *Manager) Export(ctx context.Context, id, version string) (string, error) {
	value, err := m.store.GetPlugin(ctx, id, version)
	if err != nil {
		return "", err
	}
	if value.TrustState == trustStateDevelopment || strings.TrimSpace(value.PackagePath) == "" {
		return "", errors.New("development source plugins do not have an exportable package")
	}
	return value.PackagePath, nil
}
func (m *Manager) Logs(id, version string, maximum int64) ([]byte, error) {
	if maximum <= 0 || maximum > 1<<20 {
		maximum = 128 << 10
	}
	path := filepath.Join(m.root, "logs", id+"-"+version+".log")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	offset := info.Size() - maximum
	if offset < 0 {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(file, maximum))
}
func (m *Manager) Scan(ctx context.Context) ([]core.PluginInstallation, error) {
	entries, err := os.ReadDir(filepath.Join(m.root, "inbox"))
	if err != nil {
		return nil, err
	}
	result := []core.PluginInstallation{}
	for _, entry := range entries {
		if entry.IsDir() || !isArchive(entry.Name()) {
			continue
		}
		source := filepath.Join(m.root, "inbox", entry.Name())
		value, importErr := m.Import(ctx, source, ImportOptions{})
		if importErr != nil {
			if m.logger != nil {
				m.logger.Error("plugin inbox import rejected", "package", entry.Name(), "error", importErr)
			}
			_ = m.reject(source, importErr)
			continue
		}
		_ = os.Remove(source)
		result = append(result, value)
	}
	if len(result) > 0 && m.logger != nil {
		m.logger.Info("plugin inbox scan completed", "imported", len(result))
	}
	return result, nil
}

func (m *Manager) Start(ctx context.Context) error {
	plugins, err := m.store.ListPlugins(ctx)
	if err != nil {
		return err
	}
	enabled := 0
	for _, value := range plugins {
		if value.Enabled {
			enabled++
		}
	}
	if m.logger != nil {
		m.logger.Info("plugin manager starting", "installed", len(plugins), "enabled", enabled, "plugin_log_format", m.pluginLogFormat, "plugin_log_level", m.pluginLogLevel)
	}
	for _, value := range plugins {
		if value.Enabled {
			if err := m.Enable(ctx, value.ID, value.Version, true); err != nil && m.logger != nil {
				m.logger.Error("restore plugin failed", "plugin_id", value.ID, "version", value.Version, "error", err)
			}
		}
	}
	if _, err := m.Scan(ctx); err != nil && m.logger != nil {
		m.logger.Warn("plugin inbox scan failed", "error", err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := watcher.Add(filepath.Join(m.root, "inbox")); err != nil {
		watcher.Close()
		return err
	}
	watchCtx, cancel := context.WithCancel(ctx)
	m.watcher, m.watchCancel = watcher, cancel
	go m.watch(watchCtx)
	go m.supervise(watchCtx)
	if m.logger != nil {
		m.logger.Info("plugin manager started", "active", m.activeCount(), "inbox", filepath.Join(m.root, "inbox"))
	}
	return nil
}

func (m *Manager) supervise(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			active := make(map[string]*process, len(m.processes))
			for id, value := range m.processes {
				active[id] = value
			}
			m.mu.Unlock()
			for id, value := range active {
				if value.client.Exited() {
					m.registry.RemoveOwner(id)
					if m.logger != nil {
						m.logger.Error("plugin process exited", "plugin_id", id, "version", value.version)
						m.logger.Info("plugin restart started", "plugin_id", id, "version", value.version)
					}
					if err := m.Enable(ctx, id, value.version, true); err != nil {
						if m.logger != nil {
							m.logger.Error("plugin restart failed", "plugin_id", id, "version", value.version, "error", err)
						}
					} else if m.logger != nil {
						m.logger.Info("plugin restart completed", "plugin_id", id, "version", value.version)
					}
				}
			}
		}
	}
}
func (m *Manager) watch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Rename|fsnotify.Write) != 0 && isArchive(event.Name) {
				go m.importWhenStable(ctx, event.Name)
			}
		case err := <-m.watcher.Errors:
			if err != nil && m.logger != nil {
				m.logger.Error("plugin inbox watcher failed", "error", err)
			}
		}
	}
}
func (m *Manager) importWhenStable(ctx context.Context, path string) {
	var previousSize int64 = -1
	for attempt := 0; attempt < 4; attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
		info, err := os.Stat(path)
		if err != nil {
			return
		}
		if info.Size() == previousSize {
			if _, err := m.Import(ctx, path, ImportOptions{}); err != nil {
				_ = m.reject(path, err)
			} else {
				_ = os.Remove(path)
			}
			return
		}
		previousSize = info.Size()
	}
}
func (m *Manager) Close() {
	if m.logger != nil {
		m.logger.Info("plugin manager stopping", "active", m.activeCount())
	}
	if m.watchCancel != nil {
		m.watchCancel()
	}
	if m.watcher != nil {
		_ = m.watcher.Close()
	}
	m.mu.Lock()
	ids := make([]string, 0, len(m.processes))
	for id := range m.processes {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.stopActive(id)
	}
	if m.logger != nil {
		m.logger.Info("plugin manager stopped")
	}
}

func (m *Manager) activeCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.processes)
}

func (m *Manager) SeedOfficial(ctx context.Context, directory string) error {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	installed, err := m.store.ListPlugins(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]struct{}, len(installed))
	for _, value := range installed {
		known[value.ID+"@"+value.Version] = struct{}{}
	}
	for _, entry := range entries {
		if entry.IsDir() || !isArchive(entry.Name()) {
			continue
		}
		archivePath := filepath.Join(directory, entry.Name())
		value, err := m.Import(ctx, archivePath, ImportOptions{Official: true, AllowUnverified: true})
		if err != nil {
			return err
		}
		if _, exists := known[value.ID+"@"+value.Version]; !exists {
			if err := m.Enable(ctx, value.ID, value.Version, true); err != nil {
				return err
			}
		}
	}
	return nil
}

// ClearDevelopment removes source-mode records before a packaged host starts.
// This prevents a data directory used by go run from retaining a development binary.
func (m *Manager) ClearDevelopment(ctx context.Context) error {
	if err := m.store.DeleteDevelopmentPlugins(ctx); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(m.root, "development"))
}

// SyncDevelopment builds publishable source plugins and registers them as official
// development installations. It intentionally runs at startup so a changed source
// tree is picked up by the next `go run .` without creating an archive first.
func (m *Manager) SyncDevelopment(ctx context.Context, directory string) ([]core.PluginInstallation, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return nil, errors.New("development plugin source directory cannot be empty")
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoDevelopmentPlugins
	}
	if err != nil {
		return nil, fmt.Errorf("read development plugin directory: %w", err)
	}
	if m.logger != nil {
		m.logger.Info("development plugin synchronization started", "source_dir", directory)
	}
	result := make([]core.PluginInstallation, 0)
	active := make(map[string]struct{})
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "template" || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		sourceDir := filepath.Join(directory, entry.Name())
		manifestPath := filepath.Join(sourceDir, "meerkit-plugin.yaml")
		manifestBytes, readErr := os.ReadFile(manifestPath)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return nil, readErr
		}
		var manifest Manifest
		if err := yaml.Unmarshal(manifestBytes, &manifest); err != nil {
			return nil, fmt.Errorf("decode development plugin %s: %w", entry.Name(), err)
		}
		if err := manifest.Validate(sdk.ProtocolVersion); err != nil {
			return nil, fmt.Errorf("development plugin %s: %w", entry.Name(), err)
		}
		if m.logger != nil {
			m.logger.Info("development plugin build started", "plugin_id", manifest.ID, "version", manifest.Version, "source_dir", sourceDir)
		}
		if _, err := os.Stat(filepath.Join(sourceDir, "go.mod")); err != nil {
			return nil, fmt.Errorf("development plugin %s has no go.mod: %w", entry.Name(), err)
		}
		binaryName := "plugin"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		binaryPath := filepath.Join(m.root, "development", manifest.ID, manifest.Version, runtime.GOOS+"-"+runtime.GOARCH, binaryName)
		if err := os.MkdirAll(filepath.Dir(binaryPath), 0o750); err != nil {
			return nil, err
		}
		stage, err := os.MkdirTemp(filepath.Join(m.root, "staging"), "development-")
		if err != nil {
			return nil, err
		}
		temporaryBinary := filepath.Join(stage, binaryName)
		buildErr := m.developmentBuilder(ctx, sourceDir, temporaryBinary)
		if buildErr != nil {
			_ = os.RemoveAll(stage)
			return nil, fmt.Errorf("build development plugin %s: %w", manifest.ID, buildErr)
		}
		copyErr := atomicCopy(temporaryBinary, binaryPath, 0o700)
		_ = os.RemoveAll(stage)
		if copyErr != nil {
			return nil, fmt.Errorf("install development plugin %s: %w", manifest.ID, copyErr)
		}
		binaryHash, _, err := hashFile(binaryPath)
		if err != nil {
			return nil, err
		}
		readme, err := readPackageReadme(sourceDir)
		if err != nil {
			return nil, err
		}
		existing, getErr := m.store.GetPlugin(ctx, manifest.ID, manifest.Version)
		if getErr != nil && !errors.Is(getErr, sql.ErrNoRows) {
			return nil, getErr
		}
		now := time.Now().UTC()
		createdAt := now
		enabled := true
		if getErr == nil {
			createdAt = existing.CreatedAt
			if !existing.Enabled && existing.Status == "disabled" {
				enabled = false
			}
		}
		manifestJSON, _ := json.Marshal(manifest)
		installation := core.PluginInstallation{ID: manifest.ID, Name: manifest.Name, Version: manifest.Version, Vendor: manifest.Vendor, Description: manifest.Description, URL: manifest.URL, Enabled: enabled, Verified: true, Official: true, TrustState: trustStateDevelopment, Status: "installed", BinaryPath: binaryPath, PackageName: "本地源码", PackageSHA256: binaryHash, Readme: readme, Manifest: manifestJSON, Modules: manifest.Modules, CreatedAt: createdAt, UpdatedAt: now}
		if !enabled {
			installation.Status = "disabled"
		}
		if err := m.store.UpsertPlugin(ctx, installation); err != nil {
			return nil, err
		}
		if enabled {
			if err := m.store.DisableOtherPluginVersions(ctx, manifest.ID, manifest.Version); err != nil {
				return nil, err
			}
		}
		result = append(result, installation)
		if m.logger != nil {
			m.logger.Info("development plugin build completed", "plugin_id", installation.ID, "name", installation.Name, "version", installation.Version, "enabled", installation.Enabled)
		}
		active[installation.ID+"@"+installation.Version] = struct{}{}
	}
	if len(result) == 0 {
		return nil, ErrNoDevelopmentPlugins
	}
	installed, err := m.store.ListPlugins(ctx)
	if err != nil {
		return nil, err
	}
	for _, value := range installed {
		if value.TrustState != trustStateDevelopment {
			continue
		}
		if _, exists := active[value.ID+"@"+value.Version]; exists {
			continue
		}
		if err := m.store.DeletePlugin(ctx, value.ID, value.Version); err != nil {
			return nil, err
		}
		_ = os.RemoveAll(filepath.Join(m.root, "development", value.ID, value.Version))
	}
	if m.logger != nil {
		m.logger.Info("development plugin synchronization completed", "plugins", len(result))
	}
	return result, nil
}

func buildDevelopmentPlugin(ctx context.Context, sourceDir, outputPath string) error {
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", outputPath, ".")
	command.Dir = sourceDir
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("%w: %s", err, message)
		}
		return err
	}
	return nil
}

func (m *Manager) markFailure(ctx context.Context, value core.PluginInstallation, cause error) error {
	value.Enabled, value.Status, value.Error, value.UpdatedAt = false, "degraded", cause.Error(), time.Now().UTC()
	_ = m.store.UpsertPlugin(ctx, value)
	if m.logger != nil {
		m.logger.Error("plugin status changed", "plugin_id", value.ID, "name", value.Name, "version", value.Version, "status", value.Status, "error", cause)
	}
	return cause
}
func (m *Manager) reject(source string, cause error) error {
	if _, err := os.Stat(source); err != nil {
		return nil
	}
	destination := filepath.Join(m.root, "rejected", time.Now().UTC().Format("20060102T150405.000000000Z")+"-"+filepath.Base(source))
	if err := os.Rename(source, destination); err != nil {
		return err
	}
	return os.WriteFile(destination+".error.txt", []byte(cause.Error()+"\n"), 0o600)
}
func inspectSignature(stage string, manifest []byte) (signatureInfo, error) {
	documents := make(map[string][]byte, len(signedDocumentNames))
	for _, name := range signedDocumentNames {
		data, readErr := os.ReadFile(filepath.Join(stage, name))
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return signatureInfo{}, readErr
		}
		documents[name] = data
	}
	data, err := os.ReadFile(filepath.Join(stage, "meerkit-plugin.sig"))
	if errors.Is(err, os.ErrNotExist) {
		return signatureInfo{}, nil
	}
	if err != nil {
		return signatureInfo{}, err
	}
	var signature struct {
		Version   int    `json:"version"`
		KeyID     string `json:"key_id"`
		PublicKey string `json:"public_key"`
		Signature string `json:"signature"`
	}
	if err := json.Unmarshal(data, &signature); err != nil {
		return signatureInfo{}, fmt.Errorf("decode meerkit-plugin.sig: %w", err)
	}
	if signature.Version != 1 || strings.TrimSpace(signature.KeyID) == "" {
		return signatureInfo{}, fmt.Errorf("signature version 1 and key_id are required")
	}
	publicKey, err := base64.StdEncoding.DecodeString(signature.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return signatureInfo{}, fmt.Errorf("signature contains an invalid Ed25519 public key")
	}
	decoded, err := base64.StdEncoding.DecodeString(signature.Signature)
	payload := SignaturePayload(manifest, documents)
	if err != nil || len(decoded) != ed25519.SignatureSize || !ed25519.Verify(ed25519.PublicKey(publicKey), payload, decoded) {
		return signatureInfo{}, fmt.Errorf("manifest signature is invalid")
	}
	return signatureInfo{Signed: true, KeyID: signature.KeyID, PublicKey: base64.StdEncoding.EncodeToString(publicKey), Fingerprint: publicKeyFingerprint(publicKey)}, nil
}

func publicKeyFingerprint(publicKey []byte) string {
	digest := sha256.Sum256(publicKey)
	return "SHA256:" + strings.ToUpper(hex.EncodeToString(digest[:]))
}

func readPackageReadme(stage string) (string, error) {
	data, err := os.ReadFile(filepath.Join(stage, "README.md"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read plugin README.md: %w", err)
	}
	if len(data) > 1<<20 {
		return "", fmt.Errorf("plugin README.md exceeds 1 MiB")
	}
	return string(data), nil
}

func verifyDescriptors(declared []core.PluginModule, actual []sdk.ModuleDescriptor) error {
	expected := map[string]string{}
	for _, module := range declared {
		expected[module.Type] = module.Version
	}
	if len(expected) != len(actual) {
		return fmt.Errorf("plugin returned %d modules, manifest declares %d", len(actual), len(expected))
	}
	for _, descriptor := range actual {
		version, ok := expected[descriptor.Type]
		if !ok || version != descriptor.Version {
			return fmt.Errorf("module %s version %s does not match manifest", descriptor.Type, descriptor.Version)
		}
	}
	return nil
}
func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(digest.Sum(nil)), size, nil
}
func atomicCopy(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	temporary := destination + ".tmp-" + core.NewID()
	output, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(temporary)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return closeErr
	}
	if err := os.Rename(temporary, destination); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return os.Chmod(destination, mode)
}
func isArchive(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.gz")
}
