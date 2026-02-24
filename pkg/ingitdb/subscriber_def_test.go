package ingitdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriberFor_Validate(t *testing.T) {
	s := &SubscriberFor{}
	assert.NoError(t, s.Validate())

	s = &SubscriberFor{Events: []TriggerEventType{"invalid"}}
	assert.ErrorContains(t, s.Validate(), "invalid event")
}

func TestWebhookDef_Validate(t *testing.T) {
	d := &WebhookDef{}
	assert.ErrorContains(t, d.Validate(), "webhook url is required")

	d.URL = "https://example.com"
	assert.NoError(t, d.Validate())
}

func TestEmailDef_Validate(t *testing.T) {
	d := &EmailDef{}
	assert.ErrorContains(t, d.Validate(), "email to addresses are required")

	d.To = []string{"test@example.com"}
	assert.ErrorContains(t, d.Validate(), "email smtp is required")

	d.SMTP = "smtp.example.com"
	assert.NoError(t, d.Validate())
}

func TestTelegramDef_Validate(t *testing.T) {
	d := &TelegramDef{}
	assert.ErrorContains(t, d.Validate(), "telegram token is required")

	d.Token = "123"
	assert.ErrorContains(t, d.Validate(), "telegram chat_id is required")

	d.ChatID = "456"
	assert.NoError(t, d.Validate())
}

func TestWhatsAppDef_Validate(t *testing.T) {
	d := &WhatsAppDef{}
	assert.ErrorContains(t, d.Validate(), "whatsapp from is required")

	d.From = "123"
	assert.ErrorContains(t, d.Validate(), "whatsapp to is required")

	d.To = "456"
	assert.ErrorContains(t, d.Validate(), "whatsapp account_sid is required")

	d.AccountSID = "AC123"
	assert.ErrorContains(t, d.Validate(), "whatsapp auth_token is required")

	d.AuthToken = "token123"
	assert.NoError(t, d.Validate())
}

func TestSlackDef_Validate(t *testing.T) {
	d := &SlackDef{}
	assert.ErrorContains(t, d.Validate(), "slack webhook_url is required")

	d.WebhookURL = "https://example.com"
	assert.NoError(t, d.Validate())
}

func TestDiscordDef_Validate(t *testing.T) {
	d := &DiscordDef{}
	assert.ErrorContains(t, d.Validate(), "discord webhook_url is required")

	d.WebhookURL = "https://example.com"
	assert.NoError(t, d.Validate())
}

func TestGitHubActionDef_Validate(t *testing.T) {
	d := &GitHubActionDef{}
	assert.ErrorContains(t, d.Validate(), "github owner is required")

	d.Owner = "owner"
	assert.ErrorContains(t, d.Validate(), "github repo is required")

	d.Repo = "repo"
	assert.ErrorContains(t, d.Validate(), "github workflow is required")

	d.Workflow = "deploy.yml"
	assert.ErrorContains(t, d.Validate(), "github ref is required")

	d.Ref = "main"
	assert.ErrorContains(t, d.Validate(), "github token is required")

	d.Token = "ghp_123"
	assert.NoError(t, d.Validate())
}

func TestGitLabCIDef_Validate(t *testing.T) {
	d := &GitLabCIDef{}
	assert.ErrorContains(t, d.Validate(), "gitlab project_id is required")

	d.ProjectID = "123"
	assert.ErrorContains(t, d.Validate(), "gitlab ref is required")

	d.Ref = "main"
	assert.ErrorContains(t, d.Validate(), "gitlab token is required")

	d.Token = "glptt-123"
	assert.NoError(t, d.Validate())
}

func TestNtfyDef_Validate(t *testing.T) {
	d := &NtfyDef{}
	assert.ErrorContains(t, d.Validate(), "ntfy topic is required")

	d.Topic = "test"
	assert.NoError(t, d.Validate())
}

func TestSMSDef_Validate(t *testing.T) {
	d := &SMSDef{}
	assert.ErrorContains(t, d.Validate(), "sms provider is required")

	d.Provider = "twilio"
	assert.ErrorContains(t, d.Validate(), "sms from is required")

	d.From = "+1"
	assert.ErrorContains(t, d.Validate(), "sms to is required")

	d.To = "+2"
	assert.ErrorContains(t, d.Validate(), "sms auth_token is required")

	d.AuthToken = "token"
	assert.ErrorContains(t, d.Validate(), "sms account_sid is required for twilio")

	d.AccountSID = "AC123"
	assert.NoError(t, d.Validate())

	d2 := &SMSDef{Provider: "vonage", From: "+1", To: "+2", AuthToken: "token"}
	assert.ErrorContains(t, d2.Validate(), "sms api_key is required for vonage")

	d2.APIKey = "apikey"
	assert.NoError(t, d2.Validate())

	d3 := &SMSDef{Provider: "other", From: "+1", To: "+2", AuthToken: "token"}
	assert.NoError(t, d3.Validate())
}

func TestSearchIndexDef_Validate(t *testing.T) {
	d := &SearchIndexDef{}
	assert.ErrorContains(t, d.Validate(), "search index provider is required")

	d.Provider = "algolia"
	assert.ErrorContains(t, d.Validate(), "search index is required")

	d.Index = "idx"
	assert.ErrorContains(t, d.Validate(), "search index api_key is required")

	d.APIKey = "key"
	assert.ErrorContains(t, d.Validate(), "search index app_id is required for algolia")

	d.AppID = "app123"
	assert.NoError(t, d.Validate())

	d2 := &SearchIndexDef{Provider: "meilisearch", Index: "idx", APIKey: "key"}
	assert.ErrorContains(t, d2.Validate(), "search index host is required for meilisearch")

	d2.Host = "http://localhost"
	assert.NoError(t, d2.Validate())

	d3 := &SearchIndexDef{Provider: "typesense", Index: "idx", APIKey: "key"}
	assert.ErrorContains(t, d3.Validate(), "search index host is required for typesense")

	d3.Host = "http://localhost"
	assert.NoError(t, d3.Validate())

	d4 := &SearchIndexDef{Provider: "other", Index: "idx", APIKey: "key"}
	assert.NoError(t, d4.Validate())
}

func TestRSSDef_Validate(t *testing.T) {
	d := &RSSDef{}
	assert.ErrorContains(t, d.Validate(), "rss output is required")

	d.Output = "out.xml"
	assert.ErrorContains(t, d.Validate(), "rss title is required")

	d.Title = "title"
	assert.ErrorContains(t, d.Validate(), "rss link is required")

	d.Link = "https://example.com"
	assert.NoError(t, d.Validate())
}

func TestSubscriberDef_Validate(t *testing.T) {
	d := &SubscriberDef{}
	require.ErrorContains(t, d.Validate(), "subscriber must have 'for' selector")

	d.For = &SubscriberFor{Events: []TriggerEventType{"invalid"}}
	require.ErrorContains(t, d.Validate(), "invalid 'for' selector")

	d.For = &SubscriberFor{}
	require.ErrorContains(t, d.Validate(), "subscriber must have at least one handler")

	d.Webhooks = []WebhookDef{{URL: "https://example.com"}}
	require.NoError(t, d.Validate())

	// Test inner handler validation
	d.Webhooks = []WebhookDef{{URL: ""}}
	require.ErrorContains(t, d.Validate(), "webhook url is required")

	d.Webhooks = nil
	d.Emails = []EmailDef{{}}
	require.ErrorContains(t, d.Validate(), "email to addresses are required")

	d.Emails = nil
	d.Telegrams = []TelegramDef{{}}
	require.ErrorContains(t, d.Validate(), "telegram token is required")

	d.Telegrams = nil
	d.WhatsApp = []WhatsAppDef{{}}
	require.ErrorContains(t, d.Validate(), "whatsapp from is required")

	d.WhatsApp = nil
	d.Slacks = []SlackDef{{}}
	require.ErrorContains(t, d.Validate(), "slack webhook_url is required")

	d.Slacks = nil
	d.Discords = []DiscordDef{{}}
	require.ErrorContains(t, d.Validate(), "discord webhook_url is required")

	d.Discords = nil
	d.GitHubActions = []GitHubActionDef{{}}
	require.ErrorContains(t, d.Validate(), "github owner is required")

	d.GitHubActions = nil
	d.GitLabCI = []GitLabCIDef{{}}
	require.ErrorContains(t, d.Validate(), "gitlab project_id is required")

	d.GitLabCI = nil
	d.Ntfy = []NtfyDef{{}}
	require.ErrorContains(t, d.Validate(), "ntfy topic is required")

	d.Ntfy = nil
	d.SMS = []SMSDef{{}}
	require.ErrorContains(t, d.Validate(), "sms provider is required")

	d.SMS = nil
	d.SearchIndexes = []SearchIndexDef{{}}
	require.ErrorContains(t, d.Validate(), "search index provider is required")

	d.SearchIndexes = nil
	d.RSS = []RSSDef{{}}
	require.ErrorContains(t, d.Validate(), "rss output is required")
}
