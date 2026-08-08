package slack

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/slack-go/slack"
)

var slackClient *slack.Client

func TestMain(m *testing.M) {
	resource.TestMain(m)
}

// sharedSlackClient returns the provider meta, matching what Terraform hands
// resources at runtime, so sweepers can use the same *Meta assertion.
func sharedSlackClient() (interface{}, error) {
	if slackClient != nil {
		return testMeta(slackClient), nil
	}

	token, ok := os.LookupEnv("SLACK_TOKEN")
	if !ok {
		return nil, fmt.Errorf("could not initialize Slack client. Set environment variable SLACK_TOKEN")
	}

	return testMeta(slack.New(token)), nil
}
