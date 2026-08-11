package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"meerkit/internal/app"
	"meerkit/internal/auth"
	"meerkit/internal/core"
	"meerkit/internal/runtimeconfig"
	"meerkit/internal/store"
)

func TestAdminKeyEncryptionRequiresCorrectPassword(t *testing.T) {
	hash := "$2a$04$W1X.JN8MO7VSG7n7d0bYE.5W3UIaXTxdy8ZDHdd60FmBHKEOUmeje"
	encrypted, err := encryptPayload([]byte(hash), "a-long-export-password")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(encrypted)
	if strings.Contains(string(encoded), hash) {
		t.Fatal("encrypted envelope contains plaintext hash")
	}
	if _, err := decryptPayload(encrypted, "wrong-password"); err == nil {
		t.Fatal("wrong password decrypted administrator key")
	}
	plain, err := decryptPayload(encrypted, "a-long-export-password")
	if got := string(plain); err != nil || got != hash {
		t.Fatalf("decrypted hash = %q, err=%v", got, err)
	}
}

func TestExportEncryptsMonitorAndNotificationSecrets(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := runtimeconfig.New(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	channel := core.NotificationChannel{ID: "secret-channel", Name: "Webhook", NotifierType: "webhook", Enabled: true, Config: json.RawMessage(`{"url":"https://example.test/hook?token=channel-secret"}`), CreatedAt: now, UpdatedAt: now}
	if err := database.CreateChannel(ctx, channel); err != nil {
		t.Fatal(err)
	}
	monitor := core.Monitor{ID: "secret-monitor", Name: "Private API", ModuleType: "custom", ModuleVersion: "1", ModuleConfigVersion: "1", Schedules: []string{"@hourly"}, Enabled: true, ModuleConfig: json.RawMessage(`{"token":"monitor-secret"}`), ConditionConfig: json.RawMessage(`{"logic":"ALL","rules":[]}`), RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := database.CreateMonitor(ctx, monitor); err != nil {
		t.Fatal(err)
	}
	server := &APIServer{store: database, runtime: manager}

	missingPassword := httptest.NewRequest(http.MethodPost, "/api/v1/system/config/transfer/export", strings.NewReader(`{"encrypted":true,"monitors":true}`))
	missingResponse := httptest.NewRecorder()
	missingContext, _ := gin.CreateTestContext(missingResponse)
	missingContext.Request = missingPassword
	server.exportConfiguration(missingContext)
	if missingResponse.Code != http.StatusBadRequest {
		t.Fatalf("export without password status=%d", missingResponse.Code)
	}

	requestBody := `{"encrypted":true,"monitors":true,"notification_channels":true,"encryption_key":"configuration-password","encryption_key_confirm":"configuration-password"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/system/config/transfer/export", strings.NewReader(requestBody))
	response := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(response)
	ginContext.Request = request
	server.exportConfiguration(ginContext)
	if response.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", response.Code, response.Body.String())
	}
	var exportSummary configExportSummary
	if err := json.Unmarshal([]byte(response.Header().Get("X-Meerkit-Export-Summary")), &exportSummary); err != nil {
		t.Fatalf("invalid export summary header: %v", err)
	}
	if !exportSummary.Encrypted || exportSummary.RuntimeTypes != 5 || exportSummary.Monitors != 1 || !exportSummary.MonitorsIncluded || exportSummary.NotificationChannels != 1 || !exportSummary.NotificationChannelsIncluded || exportSummary.StatusBoardItemsIncluded || exportSummary.AdminKey {
		t.Fatalf("unexpected export summary: %+v", exportSummary)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("monitor-secret")) || bytes.Contains(response.Body.Bytes(), []byte("channel-secret")) {
		t.Fatal("ZIP contains plaintext sensitive configuration")
	}
	bundle, err := readConfigBundle(response.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	plain, err := decryptPayload(*bundle.Protected, "configuration-password")
	if err != nil {
		t.Fatal(err)
	}
	var protected configBundleData
	if err := json.Unmarshal(plain, &protected); err != nil {
		t.Fatal(err)
	}
	if len(protected.Monitors) != 1 || len(protected.NotificationChannels) != 1 || !bytes.Contains(protected.Monitors[0].ModuleConfig, []byte("monitor-secret")) || !bytes.Contains(protected.NotificationChannels[0].Config, []byte("channel-secret")) {
		t.Fatalf("unexpected protected content: %+v", protected)
	}

	plainRequest := httptest.NewRequest(http.MethodPost, "/api/v1/system/config/transfer/export", strings.NewReader(`{"monitors":true,"notification_channels":true}`))
	plainResponse := httptest.NewRecorder()
	plainContext, _ := gin.CreateTestContext(plainResponse)
	plainContext.Request = plainRequest
	server.exportConfiguration(plainContext)
	if plainResponse.Code != http.StatusOK {
		t.Fatalf("plain export status=%d body=%s", plainResponse.Code, plainResponse.Body.String())
	}
	plainBundle, err := readConfigBundle(plainResponse.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if plainBundle.Encrypted || plainBundle.Data == nil || len(plainBundle.Data.Monitors) != 1 || len(plainBundle.Data.NotificationChannels) != 1 {
		t.Fatalf("unexpected plain bundle: %+v", plainBundle)
	}
}

func TestReadConfigBundleRejectsInvalidFormatAndExtraJSON(t *testing.T) {
	valid := configBundle{Format: configBundleFormat, Version: configBundleVersion, ExportedAt: time.Now(), Data: &configBundleData{Contents: []string{"runtime"}, Runtime: app.DefaultRuntimeConfig()}}
	data, _ := json.Marshal(valid)
	archive := zipConfigForTest(t, data)
	if _, err := readConfigBundle(archive); err != nil {
		t.Fatalf("valid bundle rejected: %v", err)
	}
	invalid := append(append([]byte{}, data...), []byte(` {}`)...)
	if _, err := readConfigBundle(zipConfigForTest(t, invalid)); err == nil {
		t.Fatal("bundle with extra JSON was accepted")
	}
	valid.Version++
	data, _ = json.Marshal(valid)
	if _, err := readConfigBundle(zipConfigForTest(t, data)); err == nil {
		t.Fatal("unsupported bundle version was accepted")
	}
}

func TestImportAcceptsAdministratorArgon2idHash(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := auth.NewService(database, time.Hour).Setup(ctx, "a-secure-test-key"); err != nil {
		t.Fatal(err)
	}
	hash, err := database.AdminKeyHash(ctx)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := runtimeconfig.New(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	bundle := configBundle{
		Format:     configBundleFormat,
		Version:    configBundleVersion,
		ExportedAt: time.Now(),
		Data: &configBundleData{
			Contents:     []string{"runtime", "admin_key"},
			Runtime:      app.DefaultRuntimeConfig(),
			AdminKeyHash: hash,
		},
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	archive := zipConfigForTest(t, encoded)
	server := &APIServer{store: database, runtime: manager}

	previewResponse := httptest.NewRecorder()
	previewContext, _ := gin.CreateTestContext(previewResponse)
	previewContext.Request = configImportRequestForTest(t, "/api/v1/system/config/transfer/import/preview", archive, "merge", "")
	server.previewConfigurationImport(previewContext)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", previewResponse.Code, previewResponse.Body.String())
	}
	var preview configImportPreview
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Encrypted || preview.Mode != "merge" || len(preview.Summary.Runtime.Overwritten) != 5 || len(preview.Summary.AdminKey.Overwritten) != 1 {
		t.Fatalf("unexpected import preview: %+v", preview)
	}

	request := configImportRequestForTest(t, "/api/v1/system/config/transfer/import", archive, "merge", "")
	response := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(response)
	ginContext.Request = request
	server.importConfiguration(ginContext)
	if response.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCompareConfigTransferItemsReflectsImportMode(t *testing.T) {
	incoming := []configTransferItem{{ID: "same", Name: "更新名称"}, {ID: "new", Name: "新增项"}}
	existing := []configTransferItem{{ID: "same", Name: "原名称"}, {ID: "old", Name: "待删除项"}}

	merged := compareConfigTransferItems(incoming, existing, false)
	if len(merged.Added) != 1 || merged.Added[0].ID != "new" || len(merged.Overwritten) != 1 || merged.Overwritten[0].ID != "same" || len(merged.Deleted) != 0 {
		t.Fatalf("unexpected merge changes: %+v", merged)
	}
	replaced := compareConfigTransferItems(incoming, existing, true)
	if len(replaced.Added) != 1 || len(replaced.Overwritten) != 1 || len(replaced.Deleted) != 1 || replaced.Deleted[0].ID != "old" {
		t.Fatalf("unexpected replace changes: %+v", replaced)
	}
}

func configImportRequestForTest(t *testing.T, path string, archive []byte, mode, encryptionKey string) *http.Request {
	t.Helper()
	var requestBody bytes.Buffer
	form := multipart.NewWriter(&requestBody)
	if err := form.WriteField("mode", mode); err != nil {
		t.Fatal(err)
	}
	if err := form.WriteField("encryption_key", encryptionKey); err != nil {
		t.Fatal(err)
	}
	file, err := form.CreateFormFile("file", "meerkit-config.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(archive); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &requestBody)
	request.Header.Set("Content-Type", form.FormDataContentType())
	return request
}

func zipConfigForTest(t *testing.T, payload []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	entry, err := writer.Create(configBundleFilename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
