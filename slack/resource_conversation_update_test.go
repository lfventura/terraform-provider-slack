package slack

import (
	"context"
	"strings"
	"testing"
)

// TestConvertConversationPrivacy_ToPrivate verifies that flipping is_private
// to true calls admin.conversations.convertToPrivate (issue #3: changing
// is_private used to be silently ignored by the update flow).
func TestConvertConversationPrivacy_ToPrivate(t *testing.T) {
	client := newMockSlackAPIServer(t, map[string]string{
		"admin.conversations.convertToPrivate": `{"ok":true}`,
	})

	if err := convertConversationPrivacy(context.Background(), client, "C123", true); err != nil {
		t.Fatalf("expected no error, got: %s", err)
	}
}

// TestConvertConversationPrivacy_ToPublic verifies the private -> public
// direction calls admin.conversations.convertToPublic.
func TestConvertConversationPrivacy_ToPublic(t *testing.T) {
	client := newMockSlackAPIServer(t, map[string]string{
		"admin.conversations.convertToPublic": `{"ok":true}`,
	})

	if err := convertConversationPrivacy(context.Background(), client, "C123", false); err != nil {
		t.Fatalf("expected no error, got: %s", err)
	}
}

// TestConvertConversationPrivacy_NotAdmin ensures a clear, actionable error is
// returned when the token cannot use the Admin API (non Enterprise Grid or
// missing admin.conversations:write scope).
func TestConvertConversationPrivacy_NotAdmin(t *testing.T) {
	client := newMockSlackAPIServer(t, map[string]string{
		"admin.conversations.convertToPrivate": `{"ok":false,"error":"not_an_admin"}`,
	})

	err := convertConversationPrivacy(context.Background(), client, "C123", true)
	if err == nil {
		t.Fatalf("expected an error, got none")
	}

	for _, want := range []string{
		"not_an_admin",
		"Enterprise Grid",
		"admin.conversations:write",
		"public to private",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error message to contain %q, got: %s", want, err)
		}
	}
}

// TestConvertConversationPrivacy_RestrictedAction covers admin-side policy
// restrictions surfacing a pointer to the Slack admin settings.
func TestConvertConversationPrivacy_RestrictedAction(t *testing.T) {
	client := newMockSlackAPIServer(t, map[string]string{
		"admin.conversations.convertToPublic": `{"ok":false,"error":"restricted_action"}`,
	})

	err := convertConversationPrivacy(context.Background(), client, "C123", false)
	if err == nil {
		t.Fatalf("expected an error, got none")
	}

	for _, want := range []string{
		"restricted_action",
		"private to public",
		"admin settings",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error message to contain %q, got: %s", want, err)
		}
	}
}
