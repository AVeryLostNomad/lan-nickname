package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpExplainsBackgroundInstallation(t *testing.T) {
	var output bytes.Buffer
	if err := run([]string{"help"}, &output, &output); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"lan-nick install",
		"lan-nick uninstall",
		"sudo lan-nick install",
		"Administrator terminal",
		"starts it at boot",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("help does not contain %q:\n%s", expected, output.String())
		}
	}
}
