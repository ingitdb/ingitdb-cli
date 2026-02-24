package ingitdb

import (
	"errors"
	"fmt"
)

// SubscriberFor represents the selector for when a subscriber group should be triggered
type SubscriberFor struct {
	Paths  []string           `yaml:"paths,omitempty"`
	Events []TriggerEventType `yaml:"events,omitempty"`
}

func (s *SubscriberFor) Validate() error {
	for _, e := range s.Events {
		if e != TriggerEventCreated && e != TriggerEventUpdated && e != TriggerEventDeleted {
			return fmt.Errorf("invalid event type: %s", e)
		}
	}
	return nil
}

// HandlerBase contains common fields for all subscriber handlers
type HandlerBase struct {
	Name string `yaml:"name,omitempty"`
}

// WebhookDef represents a webhook subscriber handler
type WebhookDef struct {
	HandlerBase `yaml:",inline"`
	URL         string            `yaml:"url"`
	Method      string            `yaml:"method,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
}

func (d *WebhookDef) Validate() error {
	if d.URL == "" {
		return errors.New("webhook url is required")
	}
	return nil
}

// EmailDef represents an email subscriber handler via SMTP
type EmailDef struct {
	HandlerBase `yaml:",inline"`
	From        string   `yaml:"from,omitempty"`
	To          []string `yaml:"to"`
	SMTP        string   `yaml:"smtp"`
	Port        int      `yaml:"port,omitempty"`
	User        string   `yaml:"user,omitempty"`
	Pass        string   `yaml:"pass,omitempty"`
	Subject     string   `yaml:"subject,omitempty"`
}

func (d *EmailDef) Validate() error {
	if len(d.To) == 0 {
		return errors.New("email to addresses are required")
	}
	if d.SMTP == "" {
		return errors.New("email smtp is required")
	}
	return nil
}

// TelegramDef represents a Telegram subscriber handler
type TelegramDef struct {
	HandlerBase `yaml:",inline"`
	Token       string `yaml:"token"`
	ChatID      string `yaml:"chat_id"`
}

func (d *TelegramDef) Validate() error {
	if d.Token == "" {
		return errors.New("telegram token is required")
	}
	if d.ChatID == "" {
		return errors.New("telegram chat_id is required")
	}
	return nil
}

// WhatsAppDef represents a WhatsApp Business API subscriber handler
type WhatsAppDef struct {
	HandlerBase `yaml:",inline"`
	From        string `yaml:"from"`
	To          string `yaml:"to"`
	AccountSID  string `yaml:"account_sid"`
	AuthToken   string `yaml:"auth_token"`
}

func (d *WhatsAppDef) Validate() error {
	if d.From == "" {
		return errors.New("whatsapp from is required")
	}
	if d.To == "" {
		return errors.New("whatsapp to is required")
	}
	if d.AccountSID == "" {
		return errors.New("whatsapp account_sid is required")
	}
	if d.AuthToken == "" {
		return errors.New("whatsapp auth_token is required")
	}
	return nil
}

// SlackDef represents a Slack incoming webhook subscriber handler
type SlackDef struct {
	HandlerBase `yaml:",inline"`
	WebhookURL  string `yaml:"webhook_url"`
}

func (d *SlackDef) Validate() error {
	if d.WebhookURL == "" {
		return errors.New("slack webhook_url is required")
	}
	return nil
}

// DiscordDef represents a Discord webhook subscriber handler
type DiscordDef struct {
	HandlerBase `yaml:",inline"`
	WebhookURL  string `yaml:"webhook_url"`
}

func (d *DiscordDef) Validate() error {
	if d.WebhookURL == "" {
		return errors.New("discord webhook_url is required")
	}
	return nil
}

// GitHubActionDef represents a GitHub Actions workflow_dispatch subscriber handler
type GitHubActionDef struct {
	HandlerBase `yaml:",inline"`
	Owner       string `yaml:"owner"`
	Repo        string `yaml:"repo"`
	Workflow    string `yaml:"workflow"`
	Ref         string `yaml:"ref"`
	Token       string `yaml:"token"`
}

func (d *GitHubActionDef) Validate() error {
	if d.Owner == "" {
		return errors.New("github owner is required")
	}
	if d.Repo == "" {
		return errors.New("github repo is required")
	}
	if d.Workflow == "" {
		return errors.New("github workflow is required")
	}
	if d.Ref == "" {
		return errors.New("github ref is required")
	}
	if d.Token == "" {
		return errors.New("github token is required")
	}
	return nil
}

// GitLabCIDef represents a GitLab CI pipeline trigger subscriber handler
type GitLabCIDef struct {
	HandlerBase `yaml:",inline"`
	ProjectID   string `yaml:"project_id"`
	Ref         string `yaml:"ref"`
	Token       string `yaml:"token"`
	Host        string `yaml:"host,omitempty"`
}

func (d *GitLabCIDef) Validate() error {
	if d.ProjectID == "" {
		return errors.New("gitlab project_id is required")
	}
	if d.Ref == "" {
		return errors.New("gitlab ref is required")
	}
	if d.Token == "" {
		return errors.New("gitlab token is required")
	}
	return nil
}

// NtfyDef represents a ntfy.sh push notification subscriber handler
type NtfyDef struct {
	HandlerBase `yaml:",inline"`
	Topic       string `yaml:"topic"`
	Server      string `yaml:"server,omitempty"`
}

func (d *NtfyDef) Validate() error {
	if d.Topic == "" {
		return errors.New("ntfy topic is required")
	}
	return nil
}

// SMSDef represents an SMS subscriber handler via Twilio or Vonage
type SMSDef struct {
	HandlerBase `yaml:",inline"`
	Provider    string `yaml:"provider"`
	From        string `yaml:"from"`
	To          string `yaml:"to"`
	AccountSID  string `yaml:"account_sid,omitempty"`
	AuthToken   string `yaml:"auth_token"`
	APIKey      string `yaml:"api_key,omitempty"`
}

func (d *SMSDef) Validate() error {
	if d.Provider == "" {
		return errors.New("sms provider is required")
	}
	if d.From == "" {
		return errors.New("sms from is required")
	}
	if d.To == "" {
		return errors.New("sms to is required")
	}
	if d.AuthToken == "" {
		// Need AuthToken for Twilio and AuthToken(Api Secret) for Vonage
		return errors.New("sms auth_token is required")
	}
	if d.Provider == "twilio" && d.AccountSID == "" {
		return errors.New("sms account_sid is required for twilio")
	}
	if d.Provider == "vonage" && d.APIKey == "" {
		return errors.New("sms api_key is required for vonage")
	}
	return nil
}

// SearchIndexDef represents a search index sync subscriber handler
type SearchIndexDef struct {
	HandlerBase `yaml:",inline"`
	Provider    string `yaml:"provider"`
	Index       string `yaml:"index"`
	AppID       string `yaml:"app_id,omitempty"`
	APIKey      string `yaml:"api_key"`
	Host        string `yaml:"host,omitempty"`
}

func (d *SearchIndexDef) Validate() error {
	if d.Provider == "" {
		return errors.New("search index provider is required")
	}
	if d.Index == "" {
		return errors.New("search index is required")
	}
	if d.APIKey == "" {
		return errors.New("search index api_key is required")
	}
	if d.Provider == "algolia" && d.AppID == "" {
		return errors.New("search index app_id is required for algolia")
	}
	if (d.Provider == "meilisearch" || d.Provider == "typesense") && d.Host == "" {
		return fmt.Errorf("search index host is required for %s", d.Provider)
	}
	return nil
}

// RSSDef represents an RSS or Atom feed generator subscriber handler
type RSSDef struct {
	HandlerBase `yaml:",inline"`
	Output      string `yaml:"output"`
	Title       string `yaml:"title"`
	Link        string `yaml:"link"`
	Format      string `yaml:"format,omitempty"`
}

func (d *RSSDef) Validate() error {
	if d.Output == "" {
		return errors.New("rss output is required")
	}
	if d.Title == "" {
		return errors.New("rss title is required")
	}
	if d.Link == "" {
		return errors.New("rss link is required")
	}
	return nil
}

// SubscriberDef represents a single subscriber configuration block
type SubscriberDef struct {
	HandlerBase   `yaml:",inline"`
	For           *SubscriberFor    `yaml:"for"`
	Webhooks      []WebhookDef      `yaml:"webhooks,omitempty"`
	Emails        []EmailDef        `yaml:"emails,omitempty"`
	Telegrams     []TelegramDef     `yaml:"telegrams,omitempty"`
	WhatsApp      []WhatsAppDef     `yaml:"whatsapp,omitempty"`
	Slacks        []SlackDef        `yaml:"slacks,omitempty"`
	Discords      []DiscordDef      `yaml:"discords,omitempty"`
	GitHubActions []GitHubActionDef `yaml:"github_actions,omitempty"`
	GitLabCI      []GitLabCIDef     `yaml:"gitlab_ci,omitempty"`
	Ntfy          []NtfyDef         `yaml:"ntfy,omitempty"`
	SMS           []SMSDef          `yaml:"sms,omitempty"`
	SearchIndexes []SearchIndexDef  `yaml:"search_indexes,omitempty"`
	RSS           []RSSDef          `yaml:"rss,omitempty"`
}

func (s *SubscriberDef) Validate() error {
	if s.For == nil {
		return errors.New("subscriber must have 'for' selector")
	}
	if err := s.For.Validate(); err != nil {
		return fmt.Errorf("invalid 'for' selector: %w", err)
	}

	hasHandler := false

	// Validate webhooks
	for i, h := range s.Webhooks {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("webhook at index %d is invalid: %w", i, err)
		}
	}

	// Validate emails
	for i, h := range s.Emails {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("email at index %d is invalid: %w", i, err)
		}
	}

	// Validate telegrams
	for i, h := range s.Telegrams {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("telegram at index %d is invalid: %w", i, err)
		}
	}

	// Validate whatsapp
	for i, h := range s.WhatsApp {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("whatsapp at index %d is invalid: %w", i, err)
		}
	}

	// Validate slacks
	for i, h := range s.Slacks {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("slack at index %d is invalid: %w", i, err)
		}
	}

	// Validate discords
	for i, h := range s.Discords {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("discord at index %d is invalid: %w", i, err)
		}
	}

	// Validate github actions
	for i, h := range s.GitHubActions {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("github action at index %d is invalid: %w", i, err)
		}
	}

	// Validate gitlab ci
	for i, h := range s.GitLabCI {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("gitlab ci at index %d is invalid: %w", i, err)
		}
	}

	// Validate ntfy
	for i, h := range s.Ntfy {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("ntfy at index %d is invalid: %w", i, err)
		}
	}

	// Validate sms
	for i, h := range s.SMS {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("sms at index %d is invalid: %w", i, err)
		}
	}

	// Validate search indexes
	for i, h := range s.SearchIndexes {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("search index at index %d is invalid: %w", i, err)
		}
	}

	// Validate rss
	for i, h := range s.RSS {
		hasHandler = true
		if err := h.Validate(); err != nil {
			return fmt.Errorf("rss at index %d is invalid: %w", i, err)
		}
	}

	if !hasHandler {
		return errors.New("subscriber must have at least one handler (e.g. webhooks, emails)")
	}

	return nil
}
