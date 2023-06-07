package slack

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/lfventura/slack-go"
)

func dataSourceAllUsers() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAllUsersRead,

		Schema: map[string]*schema.Schema{
			"team_id": {
				Type:		  schema.TypeString,
				Optional:     true,
				ForceNew:     true,
			},
			// Computed Attributes
			"list": {
				Type:		  schema.TypeList,
				Description:  "Set of users",
				Computed:     true,
				Elem:         &schema.Schema{
					            Type: schema.TypeMap,
				              },
			},
		},
	}
}

func dataSourceAllUsersRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	client := m.(*slack.Client)

	var matchingUsers []slack.User
	var myList = make([]map[string]interface{}, 0)

	team_id := d.Get("team_id").(string)

	users, err := client.GetUsersContext(ctx, slack.GetUsersOptionTeamID(team_id))
	if err != nil {
		return diag.FromErr(fmt.Errorf("Error searching for all users %s", err))
	}

	for _, user := range users {
		matchingUsers = append(matchingUsers, user)
	}
	
	for _,v := range matchingUsers {
		newMap := make(map[string]interface{})
		newMap["id"] = v.ID
		newMap["name"] = v.Name
		newMap["email"] = v.Profile.Email
		// newMap["deleted"] = v.Deleted
		// newMap["real_name"] = v.Profile.RealName
		// newMap["display_name"] = v.Profile.DisplayName
		// newMap["title"] = v.Profile.Title
		// newMap["is_admin"] = v.IsAdmin
		// newMap["is_owner"] = v.IsOwner
		// newMap["is_primary_owner"] = v.IsPrimaryOwner
		// newMap["is_restricted"] = v.IsRestricted
		// newMap["is_ultrarestricted"] = v.IsUltraRestricted
		// newMap["is_invited_user"] = v.IsInvitedUser
		// newMap["has_2fa"] = v.Has2FA
		myList = append(myList, newMap)
	}

	d.Set("list", myList)
	d.SetId(strconv.Itoa(len(myList)))

	return diags
}
