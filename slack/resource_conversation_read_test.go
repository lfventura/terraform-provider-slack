package slack

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/slack-go/slack"
)

// newConversationAPIServer mocks conversations.info and a paginated
// conversations.members endpoint. membersPages are served in order, with a
// next_cursor pointing to the following page. membersCalls counts how many
// times conversations.members was hit.
func newConversationAPIServer(t *testing.T, channelJSON string, membersPages [][]string, membersCalls *int32) *slack.Client {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/conversations.info", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"channel":%s}`, channelJSON)
	})
	mux.HandleFunc("/conversations.members", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(membersCalls, 1)
		page := 0
		if c := r.FormValue("cursor"); c != "" {
			if _, err := fmt.Sscanf(c, "page-%d", &page); err != nil {
				t.Errorf("unexpected cursor format %q: %s", c, err)
			}
		}
		next := ""
		if page < len(membersPages)-1 {
			next = fmt.Sprintf("page-%d", page+1)
		}
		members := "[]"
		if page < len(membersPages) {
			members = `["` + membersPages[page][0] + `"`
			for _, m := range membersPages[page][1:] {
				members += `,"` + m + `"`
			}
			members += "]"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"members":%s,"response_metadata":{"next_cursor":"%s"}}`, members, next)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return slack.New("xoxp-test", slack.OptionAPIURL(server.URL+"/"))
}

// TestGetAllUsersInConversation_Paginates ensures every page is fetched; a
// single unpaginated call only returned the first page, causing false
// permanent_members drift on large channels.
func TestGetAllUsersInConversation_Paginates(t *testing.T) {
	var calls int32
	client := newConversationAPIServer(t, `{"id":"C123"}`, [][]string{
		{"U1", "U2"},
		{"U3"},
		{"U4"},
	}, &calls)

	members, err := getAllUsersInConversation(context.Background(), client, "C123")
	if err != nil {
		t.Fatalf("expected no error, got: %s", err)
	}

	want := []string{"U1", "U2", "U3", "U4"}
	if len(members) != len(want) {
		t.Fatalf("expected %v, got: %v", want, members)
	}
	for i, m := range want {
		if members[i] != m {
			t.Errorf("expected member %d to be %s, got %s", i, m, members[i])
		}
	}
	if calls != 3 {
		t.Errorf("expected 3 paginated calls, got %d", calls)
	}
}

// TestConversationRead_PermanentMembersDrift verifies that Read stores the
// intersection of configured permanent_members with the channel's actual
// members: a configured member who left shows up as drift (removed from
// state) while organic members never enter the state.
func TestConversationRead_PermanentMembersDrift(t *testing.T) {
	var calls int32
	// U2 left the channel; U9 joined organically and is not managed.
	client := newConversationAPIServer(t, `{"id":"C123","name":"test-channel"}`, [][]string{
		{"U1", "U9"},
	}, &calls)

	d := schema.TestResourceDataRaw(t, resourceSlackConversation().Schema, map[string]interface{}{
		"name":              "test-channel",
		"permanent_members": []interface{}{"U1", "U2"},
	})
	d.SetId("C123")

	diags := resourceSlackConversationRead(context.Background(), d, client)
	if diags.HasError() {
		t.Fatalf("expected no error, got: %v", diags)
	}

	got := schemaSetToSlice(d.Get("permanent_members").(*schema.Set))
	if len(got) != 1 || got[0] != "U1" {
		t.Errorf("expected permanent_members [U1], got: %v", got)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 conversations.members call, got %d", calls)
	}
}

// TestConversationRead_NoPermanentMembersSkipsMembersCall ensures channels
// without managed members do not pay the members API call and produce no
// drift.
func TestConversationRead_NoPermanentMembersSkipsMembersCall(t *testing.T) {
	var calls int32
	client := newConversationAPIServer(t, `{"id":"C123","name":"test-channel"}`, [][]string{
		{"U1"},
	}, &calls)

	d := schema.TestResourceDataRaw(t, resourceSlackConversation().Schema, map[string]interface{}{
		"name": "test-channel",
	})
	d.SetId("C123")

	diags := resourceSlackConversationRead(context.Background(), d, client)
	if diags.HasError() {
		t.Fatalf("expected no error, got: %v", diags)
	}

	if calls != 0 {
		t.Errorf("expected conversations.members not to be called, got %d calls", calls)
	}
	if got := schemaSetToSlice(d.Get("permanent_members").(*schema.Set)); len(got) != 0 {
		t.Errorf("expected empty permanent_members, got: %v", got)
	}
}
