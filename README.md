# Terraform Provider for Slack

[![Release](https://img.shields.io/github/v/release/lfventura/terraform-provider-slack)](https://github.com/lfventura/terraform-provider-slack/releases)
[![Registry](https://img.shields.io/badge/terraform-registry-623CE4)](https://registry.terraform.io/providers/lfventura/slack/latest)
[![Build](https://github.com/lfventura/terraform-provider-slack/actions/workflows/build.yml/badge.svg)](https://github.com/lfventura/terraform-provider-slack/actions/workflows/build.yml)
[![CodeQL](https://github.com/lfventura/terraform-provider-slack/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/lfventura/terraform-provider-slack/actions/workflows/codeql-analysis.yml)

Manage Slack conversations, user groups and memberships with Terraform.

Fork of [pablovarela/terraform-provider-slack](https://github.com/pablovarela/terraform-provider-slack) with first-class support for **Slack Enterprise Grid** (`team_id` on conversations and user groups) and additional fixes. Since v2.0.0 it depends on the upstream [slack-go/slack](https://github.com/slack-go/slack) library.

## Usage

```hcl
terraform {
  required_providers {
    slack = {
      source  = "lfventura/slack"
      version = "~> 2.0"
    }
  }
}

provider "slack" {
  token = var.slack_token # or set SLACK_TOKEN
}

data "slack_user" "someone" {
  name = "some.user"
}

resource "slack_usergroup" "team" {
  name        = "My Team"
  handle      = "my-team"
  description = "Managed by Terraform"
  users       = [data.slack_user.someone.id]
}

resource "slack_conversation" "channel" {
  name              = "my-channel"
  topic             = "Managed by Terraform"
  permanent_members = slack_usergroup.team.users
  is_private        = true
}
```

Full documentation on the [Terraform Registry](https://registry.terraform.io/providers/lfventura/slack/latest/docs).

## Resources and data sources

| Type | Name | Description |
|---|---|---|
| Resource | `slack_conversation` | Channels: create, archive, rename, topic/purpose, members, public/private |
| Resource | `slack_usergroup` | User groups: create, members, handle, channels |
| Data source | `slack_conversation` | Look up a channel by ID or name |
| Data source | `slack_user` | Look up a user by name or email |
| Data source | `slack_users` | List all workspace users |
| Data source | `slack_usergroup` | Look up a user group by ID or name |

## Authentication

Create a [Slack App](https://api.slack.com/apps), install it to your workspace and use its token via the `token` argument or the `SLACK_TOKEN` environment variable.

Required scopes:

- `channels:read`, `channels:write`, `groups:read`, `groups:write`
- `usergroups:read`, `usergroups:write`
- `users:read`, `users:read.email`

## Enterprise Grid

- `team_id` is supported on `slack_conversation`, `slack_usergroup` and the data sources, so you can target a specific workspace within an org.
- Changing `is_private` on an existing channel converts it in place through the [Admin API](https://api.slack.com/methods/admin.conversations.convertToPrivate). This requires a user token (`xoxp`) with the `admin.conversations:write` scope. Outside Enterprise Grid, recreate the channel instead (`terraform apply -replace`).

## Development

Requirements: [Go](https://go.dev/) (see `go.mod` for the minimum version).

```shell
go build ./...        # compile
go test ./...         # unit tests (acceptance tests skip without TF_ACC)
golangci-lint run     # lint
```

Acceptance tests run against a real workspace and create/destroy resources. Use a dedicated test workspace:

```shell
SLACK_TOKEN=xoxp-... TF_ACC=1 go test ./... -v
```

To try a local build with Terraform:

```shell
go build -ldflags="-X main.version=0.0.1 -X main.commit=n/a" -o terraform-provider-slack_v0.0.1
mkdir -p ~/.terraform.d/plugins/terraform.local/local/slack/0.0.1/$(go env GOOS)_$(go env GOARCH)
mv terraform-provider-slack_v0.0.1 ~/.terraform.d/plugins/terraform.local/local/slack/0.0.1/$(go env GOOS)_$(go env GOARCH)/
```

```hcl
terraform {
  required_providers {
    slack = {
      source  = "terraform.local/local/slack"
      version = "0.0.1"
    }
  }
}
```

## Releasing

Push a `v*` tag. The [Release workflow](.github/workflows/release.yml) builds, signs and publishes the release with GoReleaser, and the Terraform Registry picks it up automatically.

## Credits

Based on [pablovarela/terraform-provider-slack](https://github.com/pablovarela/terraform-provider-slack). Licensed under the [Apache License 2.0](LICENSE).
