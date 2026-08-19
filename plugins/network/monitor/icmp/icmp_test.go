package icmpmonitor

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestDescriptorDeclaresConnectivityFields(t *testing.T) {
	descriptor := New().Descriptor()
	if descriptor.Type != "icmp" || len(descriptor.ResultSets) != 1 || descriptor.ResultSets[0].Key != "connectivity" {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
}

func TestSelectAddressHonorsVersion(t *testing.T) {
	addresses := []net.IP{net.ParseIP("2001:db8::1"), net.ParseIP("192.0.2.1")}
	address, _, version := selectAddress(addresses, "ipv4")
	if address == nil || address.String() != "192.0.2.1" || version != "ipv4" {
		t.Fatalf("unexpected selection: %v %s", address, version)
	}
}

func TestObservationCalculatesLossAndRTT(t *testing.T) {
	value := observation("example.com", "192.0.2.1", "ipv4", "unprivileged", 4, []float64{10, 20, 30}, time.Now(), nil)
	result := value.ResultSets["connectivity"]
	if result["packets_received"] != 3 || result["packet_loss_percent"] != 25.0 || result["avg_rtt_ms"] != 20.0 || result["jitter_ms"] != 10.0 {
		t.Fatalf("unexpected statistics: %#v", result)
	}
}

func TestValidationRejectsOversizedPacket(t *testing.T) {
	raw := json.RawMessage(`{"host":"example.com","packet_size":1401}`)
	if err := New().ValidateConfig(raw); err == nil {
		t.Fatal("expected packet size validation error")
	}
}
