---
subcategory: "Slack"
page_title: "Slack: slack_users"
---

# slack_users Data Source

Use this data source to get a list with information for all users in Slack

## Required scopes

This resource requires the following scopes:

- [users:read](https://api.slack.com/scopes/users:read)
- [users:read.email](https://api.slack.com/scopes/users:read.email)

The Slack API methods used by the resource are:

- [users.lookupByEmail](https://api.slack.com/methods/users.lookupByEmail)
- [users.list](https://api.slack.com/methods/users.list)

If you get `missing_scope` errors while using this resource check the scopes against
the documentation for the methods above.

## Example Usage

```hcl
data "slack_users" "all_users" {
  team_id = "xxxx"
}
```

## Argument Reference

The following arguments are supported:

- `team_id` - (Optional) The team_id of the slack workspace

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

In the top level

- `id` - The ID of the query, currently defaults to the number of users in the list

In the list level

- `id` - The ID of the user
- `name` - The Name of the user
- `email` - The Email of the user
