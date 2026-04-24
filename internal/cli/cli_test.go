package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelpUsesCommandNameAndSucceeds(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run("tod", []string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "Usage of tod:") {
		t.Fatalf("stderr = %q, want usage for tod", got)
	}
}
