package slack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/slack-go/slack"
)

func resourceSlackUserGroup() *schema.Resource {
	return &schema.Resource{
		ReadContext:   resourceSlackUserGroupRead,
		CreateContext: resourceSlackUserGroupCreate,
		UpdateContext: resourceSlackUserGroupUpdate,
		DeleteContext: resourceSlackUserGroupDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"channels": {
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Set:      schema.HashString,
				Optional: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"handle": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"users": {
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Set:      schema.HashString,
				Optional: true,
			},
			"team_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceSlackUserGroupCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*slack.Client)

	name := d.Get("name").(string)
	description := d.Get("description").(string)
	handle := d.Get("handle").(string)
	channels := d.Get("channels").(*schema.Set)
	users := d.Get("users").(*schema.Set)
	teamID := d.Get("team_id").(string)

	userGroup := slack.UserGroup{
		Name:        name,
		Description: description,
		Handle:      handle,
		Prefs: slack.UserGroupPrefs{
			Channels: schemaSetToSlice(channels),
		},
		TeamID: teamID,
	}

	createdUserGroup, err := client.CreateUserGroupContext(ctx, userGroup)
	if err != nil {
		if err.Error() != "name_already_exists" && err.Error() != "handle_already_exists" {
			return diag.Errorf("could not create usergroup %s: %v", name, err)
		}
		conflict := err.Error()
		group, err := findUserGroupByName(ctx, name, true, teamID, m)
		if err != nil {
			return diag.Errorf("usergroup %q already exists in the Slack workspace (Slack API returned %q), but the provider could not find the existing usergroup to import it into the Terraform state: %v. This usually happens when the existing usergroup belongs to a different team_id than the one configured (team_id=%q) or its name/handle does not match exactly. Set the correct team_id, or import the existing usergroup with 'terraform import'.", name, conflict, err, teamID)
		}
		_, err = client.EnableUserGroupContext(ctx, group.ID, slack.EnableUserGroupOptionTeamID(teamID))
		if err != nil {
			if err.Error() != "already_enabled" {
				return diag.Errorf("could not enable usergroup %s (%s): %v", name, group.ID, err)
			}
		}
		_, err = client.UpdateUserGroupContext(ctx, group.ID, slack.UpdateUserGroupsOptionTeamID(teamID))
		if err != nil {
			return diag.Errorf("could not update usergroup %s (%s): %v", name, group.ID, err)
		}
		d.SetId(group.ID)
	} else {
		d.SetId(createdUserGroup.ID)
	}

	if users.Len() > 0 {
		_, err := client.UpdateUserGroupMembersContext(ctx, d.Id(), strings.Join(schemaSetToSlice(users), ","), slack.UpdateUserGroupMembersOptionTeamID(teamID))
		if err != nil {
			return diag.Errorf("could not update usergroup members(b) %s: %v, ids %s\n", name, err, strings.Join(schemaSetToSlice(users), ","))
		}
		schemaSetToSlice(users)
	}
	return resourceSlackUserGroupRead(ctx, d, m)
}

func resourceSlackUserGroupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*slack.Client)
	id := d.Id()
	teamID := d.Get("team_id").(string)
	attempt := 0
	var userGroups []slack.UserGroup
	var err error
	var diags diag.Diagnostics
	for {
		attempt++
		userGroups, err = client.GetUserGroupsContext(ctx, slack.GetUserGroupsOptionIncludeUsers(true), slack.GetUserGroupsOptionTeamID(teamID))
		if err != nil {
			if rateLimitedError, ok := err.(*slack.RateLimitedError); ok {
				time.Sleep(rateLimitedError.RetryAfter)
			} else {
				return diag.FromErr(fmt.Errorf("couldn't get usergroups: %w", err))
			}
		} else {
			break
		}
		if attempt > maxRateLimitRetries {
			return diag.FromErr(fmt.Errorf("couldn't get usergroups after waiting for rate limit: %w", err))
		}
	}

	for _, userGroup := range userGroups {
		if userGroup.ID == id {
			return updateUserGroupData(d, userGroup)
		}
	}
	diags = append(diags, diag.Diagnostic{
		Severity: diag.Warning,
		Summary:  fmt.Sprintf("usergroup with ID %s not found, removing from state", id),
	})
	d.SetId("")
	return diags
}

func findUserGroupByName(ctx context.Context, name string, includeDisabled bool, teamID string, m interface{}) (slack.UserGroup, error) {
	client := m.(*slack.Client)
	userGroups, err := client.GetUserGroupsContext(ctx, slack.GetUserGroupsOptionIncludeDisabled(includeDisabled), slack.GetUserGroupsOptionIncludeUsers(true), slack.GetUserGroupsOptionTeamID(teamID))
	if err != nil {
		return slack.UserGroup{}, err
	}

	for _, userGroup := range userGroups {
		if userGroup.Name == name {
			return userGroup, nil
		}
	}

	return slack.UserGroup{}, fmt.Errorf("could not find usergroup %s", name)
}

func findUserGroupByID(ctx context.Context, id string, includeDisabled bool, teamID string, m interface{}) (slack.UserGroup, error) {
	client := m.(*slack.Client)
	userGroups, err := client.GetUserGroupsContext(ctx, slack.GetUserGroupsOptionIncludeDisabled(includeDisabled), slack.GetUserGroupsOptionIncludeUsers(true), slack.GetUserGroupsOptionTeamID(teamID))
	if err != nil {
		return slack.UserGroup{}, err
	}

	for _, userGroup := range userGroups {
		if userGroup.ID == id {
			return userGroup, nil
		}
	}

	return slack.UserGroup{}, fmt.Errorf("could not find usergroup %s", id)
}

func resourceSlackUserGroupUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*slack.Client)

	id := d.Id()
	name := d.Get("name").(string)
	description := d.Get("description").(string)
	handle := d.Get("handle").(string)
	channels := d.Get("channels").(*schema.Set)
	users := d.Get("users").(*schema.Set)
	teamID := d.Get("team_id").(string)

	updateUserGroupOptions := []slack.UpdateUserGroupsOption{
		slack.UpdateUserGroupsOptionName(name),
		slack.UpdateUserGroupsOptionChannels(schemaSetToSlice(channels)),
		slack.UpdateUserGroupsOptionDescription(&description),
		slack.UpdateUserGroupsOptionHandle(handle),
		slack.UpdateUserGroupsOptionTeamID(teamID),
	}
	_, err := client.UpdateUserGroupContext(ctx, id, updateUserGroupOptions...)
	if err != nil {
		return diag.Errorf("could not update usergroup %s: %s", name, err)
	}

	if d.HasChanges("users") {
		UpdateUserGroupMembersOptions := []slack.UpdateUserGroupMembersOption{
			slack.UpdateUserGroupMembersOptionTeamID(teamID),
		}

		_, err := client.UpdateUserGroupMembersContext(ctx, id, strings.Join(schemaSetToSlice(users), ","), UpdateUserGroupMembersOptions...)
		if err != nil {
			return diag.Errorf("could not update usergroup members(b) %s: %v, ids %s\n", name, err, strings.Join(schemaSetToSlice(users), ","))
		}
		schemaSetToSlice(users)
	}
	return resourceSlackUserGroupRead(ctx, d, m)
}

func resourceSlackUserGroupDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	client := m.(*slack.Client)

	id := d.Id()
	teamID := d.Get("team_id").(string)

	_, err := client.DisableUserGroupContext(ctx, id, slack.DisableUserGroupOptionTeamID(teamID))
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func updateUserGroupData(d *schema.ResourceData, userGroup slack.UserGroup) diag.Diagnostics {
	if userGroup.ID == "" {
		return diag.Errorf("error setting id: returned usergroup does not have an id")
	}
	d.SetId(userGroup.ID)

	if err := d.Set("name", userGroup.Name); err != nil {
		return diag.Errorf("error setting name: %s", err)
	}

	if err := d.Set("handle", userGroup.Handle); err != nil {
		return diag.Errorf("error setting handle: %s", err)
	}

	if err := d.Set("description", userGroup.Description); err != nil {
		return diag.Errorf("error setting description: %s", err)
	}

	if err := d.Set("channels", userGroup.Prefs.Channels); err != nil {
		return diag.Errorf("error setting channels: %s", err)
	}

	if err := d.Set("users", userGroup.Users); err != nil {
		return diag.Errorf("error setting users: %s", err)
	}

	if err := d.Set("team_id", userGroup.TeamID); err != nil {
		return diag.Errorf("error setting team_id: %s", err)
	}

	return nil
}
