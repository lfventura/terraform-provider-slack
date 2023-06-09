package slack

import (
	"context"
	"fmt"
	"github.com/lfventura/slack-go"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceAllUsers() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAllUsersRead,

		Schema: map[string]*schema.Schema{
			"team_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			// Computed Attributes
			"list": {
				Type:        schema.TypeList,
				Description: "Set of users",
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
				},
			},
		},
	}
}

func dataSourceAllUsersRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	client := m.(*slack.Client)

	var resultingList = make([]map[string]interface{}, 0)

	teamId := d.Get("team_id").(string)

	users, err := client.GetUsersContext(ctx, slack.GetUsersOptionTeamID(teamId))
	if err != nil {
		return diag.FromErr(fmt.Errorf("Error searching for all users %s", err))
	}

	for _, j := range users {
		resultingList = append(resultingList, map[string]interface{}{
			"id":    j.ID,
			"name":  j.Name,
			"email": j.Profile.Email,
			// "deleted": v.Deleted
			// "real_name": v.Profile.RealName
			// "display_name": v.Profile.DisplayName
			// "title": v.Profile.Title
			// "is_admin": v.IsAdmin
			// "is_owner": v.IsOwner
			// "is_primary_owner": v.IsPrimaryOwner
			// "is_restricted": v.IsRestricted
			// "is_ultrarestricted": v.IsUltraRestricted
			// "is_invited_user": v.IsInvitedUser
			// "has_2fa": v.Has2FA
		})
	}

	d.Set("list", resultingList)
	d.SetId(strconv.Itoa(len(resultingList)))

	return diags
}
