---
subcategory: "Slack"
page_title: "Slack: slack_user"
---

# slack_user Data Source

Use this data source to get information about a user for use in other
resources.

## Required scopes

This resource requires the following scopes:

- [users:read](https://api.slack.com/scopes/users:read)
- [users:read.email](https://api.slack.com/scopes/users:read.email)

The Slack API methods used by the resource are:

- [users.lookupByEmail](https://api.slack.com/methods/users.lookupByEmail)
- [users.list](https://api.slack.com/methods/users.list)

If you get `missing_scope` errors while using this resource check the scopes against
the documentation for the methods above.

If you get `missing_argument` errors while using this resource you need to pass your team_id for your Slack API to work properly

## Example Usage

```hcl
data "slack_user" "by_name" {
  name = "my-user"
}

data "slack_user" "by_email" {
  email = "my-user@example.com"
}
```

## Argument Reference

The following arguments are supported:

- `name` - (Optional) The name of the user
- `email` - (Optional) The email of the user
- `team_id` - (Optional) The Slack Team ID (Slack Enterprise Grid)

The data source expects exactly one of these fields, you can't set both.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

- `id` - The ID of the user
- `email` - The Email of the user
- `name` - The Name of the user
- `team_id` - The TeamID of the user (Slack Enterprise Grid)