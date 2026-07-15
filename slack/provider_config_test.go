package slack

import (
	"strings"
	"testing"
)

func TestValidateSlackToken(t *testing.T) {
	valid := []string{
		"xoxb-1234-abcd",
		"xoxp-1234-abcd",
		"xoxa-1234-abcd",
		"xoxe-1234-abcd",
		"xoxr-1234-abcd",
		"xapp-1-A1-abcd",
	}
	for _, token := range valid {
		if err := validateSlackToken(token); err != nil {
			t.Errorf("expected token %q to be valid, got: %s", token, err)
		}
	}

	invalid := []string{
		"",
		"not-a-token",
		"xox-1234",
		"Bearer xoxb-1234",
	}
	for _, token := range invalid {
		err := validateSlackToken(token)
		if err == nil {
			t.Errorf("expected token %q to be rejected", token)
			continue
		}
		if token != "" && !strings.Contains(err.Error(), "xoxb-") {
			t.Errorf("expected error for %q to list valid prefixes, got: %s", token, err)
		}
	}
}

// TestRemove_DoesNotMutateInput guards against the previous in-place
// implementation, which reordered/overwrote the caller's backing array.
func TestRemove_DoesNotMutateInput(t *testing.T) {
	input := []string{"a", "b", "c"}

	got := remove(input, "b")

	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("expected [a c], got: %v", got)
	}
	if input[0] != "a" || input[1] != "b" || input[2] != "c" {
		t.Errorf("input slice was mutated: %v", input)
	}

	if got := remove(input, "missing"); len(got) != 3 {
		t.Errorf("expected all 3 elements when removing a missing value, got: %v", got)
	}
	if got := remove([]string{"x", "x", "y"}, "x"); len(got) != 1 || got[0] != "y" {
		t.Errorf("expected all occurrences removed, got: %v", got)
	}
}
