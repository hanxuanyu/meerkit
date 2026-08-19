package dnsmonitor

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestExecuteFallsBackToTCPAndNormalizesAnswers(t *testing.T) {
	var networks []string
	module := &Module{Exchange: func(_ context.Context, network, _ string, request *dns.Msg) (*dns.Msg, error) {
		networks = append(networks, network)
		response := new(dns.Msg)
		response.SetReply(request)
		if network == "udp" {
			response.Truncated = true
			return response, nil
		}
		response.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120}, A: net.ParseIP("192.0.2.10")}}
		return response, nil
	}}
	raw, _ := json.Marshal(map[string]any{"name": "example.com", "server": "1.1.1.1", "record_type": "A", "transport": "auto"})
	observation, err := module.Execute(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(networks) != 2 || networks[0] != "udp" || networks[1] != "tcp" {
		t.Fatalf("unexpected transports: %#v", networks)
	}
	query := observation.ResultSets["query"]
	if query["transport"] != "tcp" || query["answer_count"] != 1 {
		t.Fatalf("unexpected result: %#v", query)
	}
	values := query["answer_values"].([]string)
	if len(values) != 1 || values[0] != "192.0.2.10" {
		t.Fatalf("unexpected answer values: %#v", values)
	}
}

func TestPTRIPAddressUsesReverseName(t *testing.T) {
	if actual := questionName("192.0.2.1", "PTR"); actual != "1.2.0.192.in-addr.arpa." {
		t.Fatalf("unexpected reverse name %q", actual)
	}
}

func TestValidateConfigRejectsUnsupportedType(t *testing.T) {
	raw := json.RawMessage(`{"name":"example.com","server":"1.1.1.1","record_type":"HTTPS"}`)
	if err := New().ValidateConfig(raw); err == nil {
		t.Fatal("expected record type validation error")
	}
}
