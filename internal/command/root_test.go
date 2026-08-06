package command

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommandAndArgumentExitCodes(t *testing.T) {
	var output bytes.Buffer
	dependencies := Dependencies{Stdout: &output, Stderr: &output, Version: "1.2.3"}
	if code := Execute(dependencies, []string{"version"}); code != 0 {
		t.Fatalf("version exit code = %d", code)
	}
	if !strings.Contains(output.String(), "1.2.3") {
		t.Fatalf("version output = %q", output.String())
	}
	output.Reset()
	if code := Execute(dependencies, []string{"version", "extra"}); code != 2 {
		t.Fatalf("argument exit code = %d, output = %q", code, output.String())
	}
}
