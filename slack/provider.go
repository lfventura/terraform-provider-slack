package slack

import (
	"context"
	"fmt"
	"strings"

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
	return slackClient, diags
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
