package slack

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/slack-go/slack"
)

func TestConversationImportedDestroy(t *testing.T) {
	const channelID = "C_EXISTING"
	tests := []struct {
		name            string
		action          string
		present         bool
		initialArchived bool
		archiveError    string
		wantCalls       int32
		wantArchived    bool
		wantError       string
		wantWarning     bool
	}{
		{name: "absent defaults to archive", wantCalls: 1, wantArchived: true},
		{name: "explicit archive", action: "archive", present: true, wantCalls: 1, wantArchived: true},
		{name: "explicit none", action: "none", present: true, wantWarning: true},
		{name: "invalid action", action: "invalid", present: true, wantError: "unknown action_on_destroy value. Valid values are [none archive]"},
		{name: "explicit empty action", present: true, wantError: "unknown action_on_destroy value. Valid values are [none archive]"},
		{name: "archive permission error", archiveError: "missing_scope", wantCalls: 1, wantError: "couldn't archive conversation C_EXISTING: missing_scope"},
		{name: "already archived", initialArchived: true, wantCalls: 1, wantArchived: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var archived atomic.Bool
			archived.Store(tt.initialArchived)
			var infoCalls, archiveCalls, createCalls int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.URL.Path != "/conversations.create" && req.FormValue("channel") != channelID {
					t.Errorf("API request %s used channel %q, want %s", req.URL.Path, req.FormValue("channel"), channelID)
				}
				switch req.URL.Path {
				case "/conversations.info":
					atomic.AddInt32(&infoCalls, 1)
					fmt.Fprintf(w, `{"ok":true,"channel":{"id":%q,"name":"existing-channel","is_archived":%t}}`, channelID, archived.Load())
				case "/conversations.archive":
					atomic.AddInt32(&archiveCalls, 1)
					if tt.archiveError != "" {
						fmt.Fprintf(w, `{"ok":false,"error":%q}`, tt.archiveError)
					} else if archived.Swap(true) {
						fmt.Fprint(w, `{"ok":false,"error":"already_archived"}`)
					} else {
						fmt.Fprint(w, `{"ok":true}`)
					}
				case "/conversations.create":
					atomic.AddInt32(&createCalls, 1)
					t.Error("destroy of an imported channel must not create a channel")
					fmt.Fprint(w, `{"ok":false,"error":"unexpected_create"}`)
				default:
					t.Errorf("unexpected Slack API request: %s", req.URL.Path)
					fmt.Fprint(w, `{"ok":false,"error":"unexpected_method"}`)
				}
			}))
			t.Cleanup(server.Close)
			client := slack.New("test-token", slack.OptionAPIURL(server.URL+"/"))
			meta := testMeta(client)
			ctx := context.Background()
			r := resourceSlackConversation()
			p := &schema.Provider{ResourcesMap: map[string]*schema.Resource{"slack_conversation": r}}

			states, err := p.ImportState(ctx, &terraform.InstanceInfo{Type: "slack_conversation"}, channelID)
			if err != nil || len(states) != 1 {
				t.Fatalf("ImportState: states=%v, error=%v", states, err)
			}
			if _, exists := states[0].Attributes["action_on_destroy"]; exists {
				t.Fatal("passthrough import unexpectedly populated action_on_destroy")
			}
			// Model an explicitly stored value, including an invalid empty string.
			if tt.present {
				states[0].Attributes["action_on_destroy"] = tt.action
			}
			state, diags := r.RefreshWithoutUpgrade(ctx, states[0], meta)
			if diags.HasError() || state == nil || state.ID != channelID {
				t.Fatalf("Refresh: state=%v, diagnostics=%v", state, diags)
			}
			if state.Attributes["name"] != "existing-channel" || state.Attributes["is_archived"] != fmt.Sprint(tt.initialArchived) {
				t.Fatalf("Refresh did not read the existing channel: %v", state.Attributes)
			}
			value, exists := r.Data(state).GetOkExists("action_on_destroy")
			if exists != tt.present || value != tt.action {
				t.Fatalf("stored action presence: got (%q, %t), want (%q, %t)", value, exists, tt.action, tt.present)
			}
			if got := atomic.LoadInt32(&infoCalls); got != 1 {
				t.Fatalf("Refresh info calls = %d, want 1", got)
			}

			result, diags := r.Apply(ctx, state, &terraform.InstanceDiff{Destroy: true}, meta)
			if tt.wantError != "" {
				if !diags.HasError() || len(diags) != 1 || diags[0].Summary != tt.wantError {
					t.Errorf("Apply diagnostics = %v, want error %q", diags, tt.wantError)
				}
				if result == nil || result.ID != channelID {
					t.Errorf("failed destroy must retain imported identity: %v", result)
				}
			} else {
				if diags.HasError() || result != nil {
					t.Errorf("Apply: state=%v, diagnostics=%v; want successful destroy", result, diags)
				}
				if tt.wantWarning {
					if len(diags) != 1 || diags[0].Severity != diag.Warning || diags[0].Summary != "conversation C_EXISTING (existing-channel) won't be archived on destroy" {
						t.Errorf("none diagnostics = %v, want existing warning", diags)
					}
				} else if len(diags) != 0 {
					t.Errorf("unexpected diagnostics: %v", diags)
				}
			}
			if got := atomic.LoadInt32(&archiveCalls); got != tt.wantCalls {
				t.Errorf("archive calls = %d, want %d", got, tt.wantCalls)
			}
			if got := archived.Load(); got != tt.wantArchived {
				t.Errorf("existing channel archived = %t, want %t", got, tt.wantArchived)
			}
			if got := atomic.LoadInt32(&createCalls); got != 0 {
				t.Errorf("create calls = %d, want 0", got)
			}
		})
	}
}
