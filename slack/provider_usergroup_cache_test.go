package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/slack-go/slack"
)

// testMeta wraps a client in the provider meta type that resources and
// datasources assert on, with the usergroup cache initialized.
func testMeta(client *slack.Client) *Meta {
	return &Meta{
		Client:  client,
		ugCache: make(map[usergroupCacheKey][]slack.UserGroup),
	}
}

// newCachingTestMeta returns a Meta whose client talks to a local stub of
// usergroups.list, together with a counter of how many requests actually
// reached the server and a recorder of the query params of each request.
func newCachingTestMeta(t *testing.T) (*Meta, *int32, *[]url.Values, func()) {
	t.Helper()

	var calls int32
	var mu sync.Mutex
	var seen []url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)

		if err := r.ParseForm(); err != nil {
			t.Errorf("could not parse request form: %s", err)
		}
		mu.Lock()
		seen = append(seen, r.Form)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"ok":true,"usergroups":[{"id":"S123","name":"group-one"}]}`)); err != nil {
			t.Errorf("could not write response: %s", err)
		}
	}))

	meta := testMeta(slack.New("xoxb-test-token", slack.OptionAPIURL(server.URL+"/")))

	return meta, &calls, &seen, server.Close
}

// TestGetUserGroups_MemoizesIdenticalParams is the core of the fix: N resource
// reads with the same parameters must collapse to a single API call.
func TestGetUserGroups_MemoizesIdenticalParams(t *testing.T) {
	meta, calls, _, closeServer := newCachingTestMeta(t)
	defer closeServer()

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		groups, err := meta.getUserGroups(ctx, "T1", false, true)
		if err != nil {
			t.Fatalf("unexpected error on call %d: %s", i, err)
		}
		if len(groups) != 1 || groups[0].ID != "S123" {
			t.Fatalf("unexpected groups on call %d: %v", i, groups)
		}
	}

	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("expected 10 reads to issue 1 API call, got %d", got)
	}
}

// TestGetUserGroups_KeyedOnParams guards the bug the composite key exists to
// prevent: the create-conflict path (includeDisabled=true) must never be served
// an enabled-only list cached by a read or datasource path, and vice versa.
func TestGetUserGroups_KeyedOnParams(t *testing.T) {
	meta, calls, seen, closeServer := newCachingTestMeta(t)
	defer closeServer()

	ctx := context.Background()

	cases := []struct {
		name            string
		teamID          string
		includeDisabled bool
		includeUsers    bool
	}{
		{"read path", "T1", false, true},
		{"create-conflict recovery", "T1", true, true},
		{"different team", "T2", false, true},
		{"without users", "T1", false, false},
	}

	for _, tc := range cases {
		if _, err := meta.getUserGroups(ctx, tc.teamID, tc.includeDisabled, tc.includeUsers); err != nil {
			t.Fatalf("%s: unexpected error: %s", tc.name, err)
		}
	}

	if got := atomic.LoadInt32(calls); got != int32(len(cases)) {
		t.Fatalf("expected %d distinct param sets to issue %d calls, got %d", len(cases), len(cases), got)
	}

	// Repeating them all must add no further traffic.
	for _, tc := range cases {
		if _, err := meta.getUserGroups(ctx, tc.teamID, tc.includeDisabled, tc.includeUsers); err != nil {
			t.Fatalf("%s (repeat): unexpected error: %s", tc.name, err)
		}
	}
	if got := atomic.LoadInt32(calls); got != int32(len(cases)) {
		t.Errorf("expected repeats to be served from cache, call count rose to %d", got)
	}

	// include_disabled must be sent only for the recovery path. slack-go omits
	// the param entirely when false, which is what Slack defaults to.
	var withDisabled int
	for _, form := range *seen {
		if form.Get("include_disabled") == "true" {
			withDisabled++
		}
	}
	if withDisabled != 1 {
		t.Errorf("expected exactly 1 request with include_disabled=true, got %d", withDisabled)
	}
}

// TestInvalidateUserGroups_ForcesRefetch covers the write path: without
// invalidation a post-write Read is served the pre-write list and Terraform
// persists stale state.
func TestInvalidateUserGroups_ForcesRefetch(t *testing.T) {
	meta, calls, _, closeServer := newCachingTestMeta(t)
	defer closeServer()

	ctx := context.Background()

	if _, err := meta.getUserGroups(ctx, "T1", false, true); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, err := meta.getUserGroups(ctx, "T1", false, true); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Fatalf("expected 1 call before invalidation, got %d", got)
	}

	meta.invalidateUserGroups()

	if _, err := meta.getUserGroups(ctx, "T1", false, true); err != nil {
		t.Fatalf("unexpected error after invalidation: %s", err)
	}
	if got := atomic.LoadInt32(calls); got != 2 {
		t.Errorf("expected invalidation to force a refetch (2 calls), got %d", got)
	}
}

// TestGetUserGroups_ConcurrentReadsCoalesce models Terraform reading resources
// in parallel (-parallelism, default 10). Concurrent misses must coalesce onto
// one in-flight request rather than stampeding the Tier-2 endpoint.
func TestGetUserGroups_ConcurrentReadsCoalesce(t *testing.T) {
	meta, calls, _, closeServer := newCachingTestMeta(t)
	defer closeServer()

	ctx := context.Background()
	const goroutines = 10

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := meta.getUserGroups(ctx, "T1", false, true); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("unexpected error from concurrent read: %s", err)
	}

	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("expected %d concurrent reads to coalesce into 1 API call, got %d", goroutines, got)
	}
}

// TestProviderConfigure_InitializesCache guards the nil-map write panic and
// pins the provider meta type the resources assert on.
func TestProviderConfigure_InitializesCache(t *testing.T) {
	provider := Provider()
	data := schema.TestResourceDataRaw(t, provider.Schema, map[string]interface{}{
		"token": "xoxb-1234-abcd",
	})

	config, diags := providerConfigure(context.Background(), data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	meta, ok := config.(*Meta)
	if !ok {
		t.Fatalf("expected provider meta to be *Meta, got %T", config)
	}
	if meta.Client == nil {
		t.Error("expected a non-nil Slack client")
	}
	if meta.ugCache == nil {
		t.Error("expected ugCache to be initialized (a nil map panics on write)")
	}
}
