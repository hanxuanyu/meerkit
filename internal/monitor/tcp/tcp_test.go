package tcpmonitor

import "testing"

func TestDescriptorDeclaresAddressSummary(t *testing.T) {
	descriptor := (&Module{}).Descriptor()
	if descriptor.ListSummary == nil {
		t.Fatal("TCP list summary is missing")
	}
	if len(descriptor.ListSummary.Fields) != 2 || descriptor.ListSummary.Fields[0] != "host" || descriptor.ListSummary.Fields[1] != "port" || descriptor.ListSummary.Separator != ":" {
		t.Fatalf("unexpected TCP list summary: %#v", descriptor.ListSummary)
	}
}
