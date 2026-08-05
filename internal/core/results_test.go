package core

import "testing"

func TestWithCommonResultSetsPreservesModuleSets(t *testing.T) {
	descriptor := WithCommonResultSets(ModuleDescriptor{ResultSets: []ResultSetDescriptor{{Key: "response", Label: "响应"}}})
	if len(descriptor.ResultSets) != 2 {
		t.Fatalf("got %d result sets, want 2", len(descriptor.ResultSets))
	}
	if descriptor.ResultSets[0].Key != "summary" || descriptor.ResultSets[1].Key != "response" {
		t.Fatalf("unexpected result set order: %#v", descriptor.ResultSets)
	}
}
