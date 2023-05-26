package slack

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/lfventura/slack-go"
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
				Type:	  schema.TypeString,
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
	team_id := d.Get("team_id").(string)

	userGroup := slack.UserGroup{
		Name:        name,
		Description: description,
		Handle:      handle,
		Prefs: slack.UserGroupPrefs{
			Channels: schemaSetToSlice(channels),
		},
		TeamID:		 team_id,
	}


	createdUserGroup, err := client.CreateUserGroupContext(ctx, userGroup)
	if err != nil {
		if err.Error() != "name_already_exists" && err.Error() != "handle_already_exists" {
			return diag.Errorf("could not create usergroup %s: %s", name, err)
		}
		group, err := findUserGroupByName(ctx, name, true, team_id, m)
		if err != nil {
			return diag.Errorf("could not find usergroup %s: %s", name, err)
		}
		_, err = client.EnableUserGroupContext(ctx, group.ID, slack.DisableUserGroupOptionTeamID(team_id))
		if err != nil {
			if err.Error() != "already_enabled" {
				return diag.Errorf("could not enable usergroup %s (%s): %s", name, group.ID, err)
			}
		}
		_, err = client.UpdateUserGroupContext(ctx, group.ID, slack.UpdateUserGroupsOptionTeamID(&team_id))
		if err != nil {
			return diag.Errorf("could not update usergroup %s (%s): %s", name, group.ID, err)
		}
		d.SetId(group.ID)
	} else {
		d.SetId(createdUserGroup.ID)
	}

	if users.Len() > 0 {
		_, err := client.UpdateUserGroupMembersContext(ctx, d.Id(), strings.Join(schemaSetToSlice(users), ","), slack.UpdateUserGroupMembersOptionTeamID(team_id))
		if err != nil {
			return diag.Errorf("could not update usergroup members %s: %s", name, err)
		}
		schemaSetToSlice(users)
	}
	return resourceSlackUserGroupRead(ctx, d, m)
}

func resourceSlackUserGroupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*slack.Client)
	id := d.Id()
	team_id := d.Get("team_id").(string)
	var diags diag.Diagnostics
	userGroups, err := client.GetUserGroupsContext(ctx, slack.GetUserGroupsOptionIncludeUsers(true), slack.GetUserGroupsOptionTeamID(team_id))
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't get usergroups: %w", err))
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

func findUserGroupByName(ctx context.Context, name string, includeDisabled bool, team_id string, m interface{}) (slack.UserGroup, error) {
	client := m.(*slack.Client)
	userGroups, err := client.GetUserGroupsContext(ctx, slack.GetUserGroupsOptionIncludeDisabled(includeDisabled), slack.GetUserGroupsOptionIncludeUsers(true), slack.GetUserGroupsOptionTeamID(team_id))
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

func findUserGroupByID(ctx context.Context, id string, includeDisabled bool, team_id string, m interface{}) (slack.UserGroup, error) {
	client := m.(*slack.Client)
	userGroups, err := client.GetUserGroupsContext(ctx, slack.GetUserGroupsOptionIncludeDisabled(includeDisabled), slack.GetUserGroupsOptionIncludeUsers(true), slack.GetUserGroupsOptionTeamID(team_id))
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
	team_id := d.Get("team_id").(string)

	updateUserGroupOptions := []slack.UpdateUserGroupsOption{
		slack.UpdateUserGroupsOptionName(name),
		slack.UpdateUserGroupsOptionChannels(schemaSetToSlice(channels)),
		slack.UpdateUserGroupsOptionDescription(&description),
		slack.UpdateUserGroupsOptionHandle(handle),
		slack.UpdateUserGroupsOptionTeamID(&team_id),
	}
	_, err := client.UpdateUserGroupContext(ctx, id, updateUserGroupOptions...)
	if err != nil {
		return diag.Errorf("could not update usergroup %s: %s", name, err)
	}

	if d.HasChanges("users") {
		UpdateUserGroupMembersOptions := []slack.UpdateUserGroupMembersOption{
			slack.UpdateUserGroupMembersOptionTeamID(team_id),
		}

		_, err := client.UpdateUserGroupMembersContext(ctx, id, strings.Join(schemaSetToSlice(users), ","), UpdateUserGroupMembersOptions...)
		if err != nil {
			return diag.Errorf("could not update usergroup members %s: %s", name, err)
		}
		schemaSetToSlice(users)
	}
	return resourceSlackUserGroupRead(ctx, d, m)
}

func resourceSlackUserGroupDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	client := m.(*slack.Client)

	id := d.Id()
	_, err := client.DisableUserGroupContext(ctx, id)
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
