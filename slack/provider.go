package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/slack-go/slack"
)

// Provider returns a *schema.Provider
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"token": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				DefaultFunc: schema.EnvDefaultFunc("SLACK_TOKEN", nil),
				Description: "The Slack token",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"slack_conversation": resourceSlackConversation(),
			"slack_usergroup":    resourceSlackUserGroup(),
		},

		DataSourcesMap: map[string]*schema.Resource{
			"slack_conversation": dataSourceConversation(),
			"slack_user":         dataSourceUser(),
			"slack_users":        dataSourceAllUsers(),
			"slack_usergroup":    dataSourceUserGroup(),
		},

		ConfigureContextFunc: providerConfigure,
	}
}

// usergroupCacheKey identifies one distinct usergroups.list query. The call
// sites do not share query params: the create-conflict recovery path needs
// disabled groups so it can re-enable them, while the read and datasource
// paths do not. Keying on the full parameter set keeps those lists separate --
// a single shared list would serve an enabled-only result to the recovery path
// and silently break "usergroup already exists" reconciliation.
//
// includeUsers is currently true at every call site, but it is part of the key
// so that a future caller passing false cannot reintroduce that class of bug.
type usergroupCacheKey struct {
	teamID          string
	includeDisabled bool
	includeUsers    bool
}

// Meta is the provider-instance state handed to every resource and datasource.
// It carries the Slack client plus a per-run memoization of usergroups.list.
type Meta struct {
	Client *slack.Client

	ugMu    sync.Mutex
	ugCache map[usergroupCacheKey][]slack.UserGroup
}

// getUserGroups returns the workspace usergroup list for the given parameters,
// fetching it at most once per distinct parameter set per provider instance.
//
// Terraform reads resources concurrently (up to -parallelism, default 10).
// ugMu is deliberately held across the fetch so concurrent misses coalesce
// onto a single in-flight API call instead of firing N identical requests.
func (m *Meta) getUserGroups(ctx context.Context, teamID string, includeDisabled, includeUsers bool) ([]slack.UserGroup, error) {
	key := usergroupCacheKey{
		teamID:          teamID,
		includeDisabled: includeDisabled,
		includeUsers:    includeUsers,
	}

	m.ugMu.Lock()
	defer m.ugMu.Unlock()

	if groups, ok := m.ugCache[key]; ok {
		return groups, nil
	}

	groups, err := m.fetchUserGroups(ctx, teamID, includeDisabled, includeUsers)
	if err != nil {
		return nil, err
	}

	if m.ugCache == nil {
		m.ugCache = make(map[usergroupCacheKey][]slack.UserGroup)
	}
	m.ugCache[key] = groups
	return groups, nil
}

// fetchUserGroups performs the actual usergroups.list call, retrying while the
// API reports rate limiting and honoring the Retry-After it returns.
func (m *Meta) fetchUserGroups(ctx context.Context, teamID string, includeDisabled, includeUsers bool) ([]slack.UserGroup, error) {
	attempt := 0
	for {
		attempt++
		groups, err := m.Client.GetUserGroupsContext(ctx,
			slack.GetUserGroupsOptionIncludeDisabled(includeDisabled),
			slack.GetUserGroupsOptionIncludeUsers(includeUsers),
			slack.GetUserGroupsOptionTeamID(teamID),
		)
		if err == nil {
			return groups, nil
		}

		rateLimitedError, ok := err.(*slack.RateLimitedError)
		if !ok {
			return nil, fmt.Errorf("couldn't get usergroups: %w", err)
		}
		if attempt > maxRateLimitRetries {
			return nil, fmt.Errorf("couldn't get usergroups after waiting for rate limit: %w", err)
		}
		time.Sleep(rateLimitedError.RetryAfter)
	}
}

// invalidateUserGroups drops every cached list. It must be called after any
// write to a usergroup, otherwise a subsequent read is served from a list
// captured before the write and Terraform persists stale state.
func (m *Meta) invalidateUserGroups() {
	m.ugMu.Lock()
	defer m.ugMu.Unlock()
	m.ugCache = make(map[usergroupCacheKey][]slack.UserGroup)
}

func providerConfigure(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	token, ok := d.GetOk("token")
	if !ok {
		return nil, diag.Errorf("could not create slack client. Please provide a token.")
	}

	tokenStr := token.(string)
	if err := validateSlackToken(tokenStr); err != nil {
		return nil, diag.FromErr(err)
	}

	slackClient := slack.New(tokenStr)
	return &Meta{
		Client:  slackClient,
		ugCache: make(map[usergroupCacheKey][]slack.UserGroup),
	}, diags
}

// validateSlackToken fails fast on tokens that cannot be valid, instead of
// letting every API call fail later with invalid_auth.
func validateSlackToken(token string) error {
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Slack token prefixes:
	// xoxb- bot tokens, xoxp- user tokens, xoxa- app-level (legacy),
	// xoxe- Enterprise Grid, xoxr- refresh tokens, xapp- app tokens.
	validPrefixes := []string{"xoxb-", "xoxp-", "xoxa-", "xoxe-", "xoxr-", "xapp-"}

	for _, prefix := range validPrefixes {
		if strings.HasPrefix(token, prefix) {
			return nil
		}
	}

	return fmt.Errorf("invalid token format: Slack tokens must start with one of: %s", strings.Join(validPrefixes, ", "))
}

func schemaSetToSlice(set *schema.Set) []string {
	s := make([]string, len(set.List()))
	for i, v := range set.List() {
		s[i] = v.(string)
	}
	return s
}

// remove returns a copy of s without r, leaving the input slice untouched
// (the previous in-place implementation mutated the caller's backing array).
func remove(s []string, r string) []string {
	result := make([]string, 0, len(s))
	for _, v := range s {
		if v != r {
			result = append(result, v)
		}
	}
	return result
}
