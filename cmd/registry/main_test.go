package main

import (
	"strings"
	"testing"
)

const mainTestPrefix = "cmd/registry:main_test"

func TestUsage_NonEmpty(t *testing.T) {
	if len(usage) == 0 {
		t.Fatalf("%s - usage string is empty", mainTestPrefix)
	}
}

func TestUsage_ContainsCommands(t *testing.T) {
	required := []string{"serve", "migrate", "clear", "seed", "DATABASE_URL"}
	for _, word := range required {
		if !strings.Contains(usage, word) {
			t.Errorf("%s - usage should contain %q", mainTestPrefix, word)
		}
	}
}
