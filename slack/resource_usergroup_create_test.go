package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/slack-go/slack"
)

// newMockSlackAPIServer spins up an httptest server that returns the provided
// JSON body for each Slack API method path (e.g. "usergroups.create"). It lets
// us unit test the create flow without talking to the real Slack API.
func newMockSlackAPIServer(t *testing.T, responses map[string]string) *slack.Client {
	t.Helper()

	mux := http.NewServeMux()
	for path, body := range responses {
		body := body
		mux.HandleFunc("/"+path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
	}

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return slack.New("test-token", slack.OptionAPIURL(server.URL+"/"))
}

// TestResourceSlackUserGroupCreate_NameConflictNotFound reproduces the scenario
// where the usergroup name already exists in the workspace (Slack returns
// name_already_exists on create) but the provider cannot locate the existing
// usergroup to adopt it. The resulting error must clearly explain the conflict
// instead of the misleading "could not find usergroup" message.
func TestResourceSlackUserGroupCreate_NameConflictNotFound(t *testing.T) {
	client := newMockSlackAPIServer(t, map[string]string{
		"usergroups.create": `{"ok":false,"error":"name_already_exists"}`,
		"usergroups.list":   `{"ok":true,"usergroups":[]}`,
	})

	d := schema.TestResourceDataRaw(t, resourceSlackUserGroup().Schema, map[string]interface{}{
		"name": "Example Group",
	})

	diags := resourceSlackUserGroupCreate(context.Background(), d, client)

	if !diags.HasError() {
		t.Fatalf("expected an error diagnostic, got none")
	}

	summary := diags[0].Summary
	for _, want := range []string{
		`usergroup "Example Group" already exists in the Slack workspace`,
		"name_already_exists",
		"terraform import",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("expected error message to contain %q, got: %s", want, summary)
		}
	}

	if strings.HasPrefix(summary, "could not find usergroup") {
		t.Errorf("error message should not use the misleading 'could not find usergroup' prefix, got: %s", summary)
	}
}

// TestResourceSlackUserGroupCreate_HandleConflictNotFound covers the same
// recovery path but triggered by a handle_already_exists conflict, ensuring the
// conflict reason is surfaced in the message.
func TestResourceSlackUserGroupCreate_HandleConflictNotFound(t *testing.T) {
	client := newMockSlackAPIServer(t, map[string]string{
		"usergroups.create": `{"ok":false,"error":"handle_already_exists"}`,
		"usergroups.list":   `{"ok":true,"usergroups":[]}`,
	})

	d := schema.TestResourceDataRaw(t, resourceSlackUserGroup().Schema, map[string]interface{}{
		"name":   "Example Group",
		"handle": "example-group",
	})

	diags := resourceSlackUserGroupCreate(context.Background(), d, client)

	if !diags.HasError() {
		t.Fatalf("expected an error diagnostic, got none")
	}

	summary := diags[0].Summary
	if !strings.Contains(summary, "handle_already_exists") {
		t.Errorf("expected error message to surface the handle_already_exists conflict, got: %s", summary)
	}
	if !strings.Contains(summary, "already exists in the Slack workspace") {
		t.Errorf("expected the conflict message, got: %s", summary)
	}
}

// TestResourceSlackUserGroupCreate_OtherErrorKeepsGenericMessage ensures a
// non-conflict create failure still returns the generic create error and does
// not use the new conflict wording.
func TestResourceSlackUserGroupCreate_OtherErrorKeepsGenericMessage(t *testing.T) {
	client := newMockSlackAPIServer(t, map[string]string{
		"usergroups.create": `{"ok":false,"error":"invalid_auth"}`,
	})

	d := schema.TestResourceDataRaw(t, resourceSlackUserGroup().Schema, map[string]interface{}{
		"name": "Example Group",
	})

	diags := resourceSlackUserGroupCreate(context.Background(), d, client)

	if !diags.HasError() {
		t.Fatalf("expected an error diagnostic, got none")
	}

	summary := diags[0].Summary
	if !strings.Contains(summary, "could not create usergroup Example Group") {
		t.Errorf("expected the generic create error, got: %s", summary)
	}
	if strings.Contains(summary, "already exists in the Slack workspace") {
		t.Errorf("did not expect the conflict message for a non-conflict error, got: %s", summary)
	}
}

// TestResourceSlackUserGroupCreate_NameConflictAdoptsExisting verifies the
// happy recovery path: when the name already exists and the provider can locate
// the existing usergroup, it adopts it into state without error.
func TestResourceSlackUserGroupCreate_NameConflictAdoptsExisting(t *testing.T) {
	group := `{"id":"S123","team_id":"","name":"Example Group","handle":"example-group","description":"desc","prefs":{"channels":[],"groups":[]},"users":[]}`
	client := newMockSlackAPIServer(t, map[string]string{
		"usergroups.create": `{"ok":false,"error":"name_already_exists"}`,
		"usergroups.list":   `{"ok":true,"usergroups":[` + group + `]}`,
		"usergroups.enable": `{"ok":true,"usergroup":` + group + `}`,
		"usergroups.update": `{"ok":true,"usergroup":` + group + `}`,
	})

	d := schema.TestResourceDataRaw(t, resourceSlackUserGroup().Schema, map[string]interface{}{
		"name": "Example Group",
	})

	diags := resourceSlackUserGroupCreate(context.Background(), d, client)

	if diags.HasError() {
		t.Fatalf("expected no error, got: %v", diags)
	}
	if d.Id() != "S123" {
		t.Errorf("expected the existing usergroup to be adopted as S123, got: %q", d.Id())
	}
}
