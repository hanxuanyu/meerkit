package tcpmonitor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
)

func TestDescriptorDeclaresAddressSummary(t *testing.T) {
	descriptor := (&Module{}).Descriptor()
	if descriptor.ListSummary == nil {
		t.Fatal("TCP list summary is missing")
	}
	if len(descriptor.ListSummary.Fields) != 2 || descriptor.ListSummary.Fields[0] != "host" || descriptor.ListSummary.Fields[1] != "port" || descriptor.ListSummary.Separator != ":" {
		t.Fatalf("unexpected TCP list summary: %#v", descriptor.ListSummary)
	}
}

func TestExecuteLogsConnectionProgressWithoutPayload(t *testing.T) {
	clientConnection, serverConnection := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverConnection.Close()
		buffer := make([]byte, 32)
		_, _ = serverConnection.Read(buffer)
		_, _ = io.WriteString(serverConnection, "private-response")
	}()

	var logs bytes.Buffer
	module := &Module{Logger: slog.New(slog.NewTextHandler(&logs, nil)), DialContext: func(context.Context, string, string) (net.Conn, error) { return clientConnection, nil }}
	config, _ := json.Marshal(map[string]any{"host": "test.local", "port": 8080, "send": "private-payload", "read_response": true})
	if _, err := module.Execute(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	<-done
	output := logs.String()
	for _, expected := range []string{"tcp connection opening", "tcp connection established", "tcp payload sent", "tcp response read", "bytes_read=16"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("missing %q in plugin logs:\n%s", expected, output)
		}
	}
	for _, secret := range []string{"private-payload", "private-response"} {
		if strings.Contains(output, secret) {
			t.Fatalf("plugin logs leaked %q:\n%s", secret, output)
		}
	}
}
