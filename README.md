# Terraform Provider for Slack


### This is a Fork!!!

Some notes about this fork...

- It uses a fork for slack-go library to allow create usergroups in Slack Enterprise Grid accounts. It uses a Fork based on https://github.com/slack-go/slack v0.12.2 that is available in https://github.com/lfventura/slack-go. In my repository I am just applying the PR https://github.com/slack-go/slack/pull/1179 that still open at slack-go official repository. 
- This provider as been created based on https://github.com/pablovarela/terraform-provider-slack v1.2.2 just adding support for Team ID to create conversations (Natively supported and can be merged to the original repository through a PR) and for usergroups (Which uses the trick described in the previous bullet, and because of that I can not open a PR for the original repository with my changes)
- When slack-go applies the indicated PR, the code that is over here will be proposed as a PR to the original pablovarela repository


Some notes for reference...

If you want to locally prove the provider without running any scripts...

```shell
go build -ldflags="-X main.version=0.0.1 -X main.commit=n/a"
mv terraform-provider-slack ~/.terraform.d/plugins/terraform.local/local/slack/0.0.1/darwin_arm64/terraform-provider-slack_v0.0.1
```

and in your providers.tf

```shell
    slack = {
      source = "terraform.local/local/slack"
      version = "0.0.1"
    }
```


How to release

1. create a new tag in github
2. /opt/homebrew/Cellar/goreleaser/1.18.2/bin/goreleaser release --clean

---

[![Go Report Card](https://goreportcard.com/badge/github.com/pablovarela/terraform-provider-slack)](https://goreportcard.com/report/github.com/pablovarela/terraform-provider-slack) <a href="https://github.com/pablovarela/terraform-provider-slack/actions?query=workflow%3ABuild">![build](https://github.com/pablovarela/terraform-provider-slack/workflows/Build/badge.svg)</a> <a href="https://github.com/pablovarela/terraform-provider-slack/actions?query=workflow%3Arelease">![release](https://github.com/pablovarela/terraform-provider-slack/workflows/release/badge.svg)</a>

The Terraform Provider for Slack is a plugin for Terraform that allows the
management of Slack resources.

### Quick Start

- [Using the provider ](https://registry.terraform.io/providers/pablovarela/slack/latest/docs)

### Documentation

Full, comprehensive documentation is available on the Terraform Registry: https://registry.terraform.io/providers/pablovarela/slack/latest/docs
