package models

import (
	"time"
)

type PlanTier string

const (
	PlanStarter PlanTier = "STARTER" // max 5 keywords
	PlanGrowth  PlanTier = "GROWTH"  // max 15 keywords
	PlanScale   PlanTier = "SCALE"   // max 50 keywords
)

type User struct {
	ID                 string    `json:"id"`
	Email              string    `json:"email"`
	Name               string    `json:"name"`
	CompanyName        string    `json:"company_name,omitempty"`
	AvatarURL          string    `json:"avatar_url,omitempty"`
	GoogleID           string    `json:"google_id,omitempty"`
	PasswordHash       string    `json:"-"`
	PlanTier           PlanTier  `json:"plan_tier"`
	TelegramChatID     string    `json:"telegram_chat_id"`
	TelegramBotToken   string    `json:"telegram_bot_token"`
	SlackWebhookURL    string    `json:"slack_webhook_url"`
	DiscordWebhookURL  string    `json:"discord_webhook_url"`
	CustomWebhookURL   string    `json:"custom_webhook_url"`
	MinIntentThreshold string    `json:"min_intent_threshold"`
	NotifyOnNegative   bool      `json:"notify_on_negative"`
	OnboardingComplete bool      `json:"onboarding_complete"`
	CreatedAt          time.Time `json:"created_at"`
}

type Keyword struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Keyword          string    `json:"keyword"`
	Category         string    `json:"category"` // "BUYER_LEADS", "BRAND_MONITORING", "COMPETITOR_WATCH", "PAIN_POINTS", "GENERAL"
	NegativeKeywords []string  `json:"negative_keywords"`
	Platforms        []string  `json:"platforms"`
	IsActive         bool      `json:"is_active"`
	LeadsCount       int       `json:"leads_count,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type IntentType string

const (
	IntentHighBuying            IntentType = "HIGH_BUYING"
	IntentSeekingRecommendation IntentType = "SEEKING_RECOMMENDATION"
	IntentCompetitorMention     IntentType = "COMPETITOR_MENTION"
	IntentBrandMention          IntentType = "BRAND_MENTION"
	IntentCustomerPainPoint     IntentType = "CUSTOMER_PAIN_POINT"
	IntentQuestionHelp          IntentType = "QUESTION_HELP"
	IntentGeneralDiscussion     IntentType = "GENERAL_DISCUSSION"
	IntentNoise                 IntentType = "NOISE"
)

type SentimentType string

const (
	SentimentPositive SentimentType = "POSITIVE"
	SentimentNeutral  SentimentType = "NEUTRAL"
	SentimentNegative SentimentType = "NEGATIVE"
)

type LeadStatus string

const (
	LeadStatusNew       LeadStatus = "NEW"
	LeadStatusReviewing LeadStatus = "REVIEWING"
	LeadStatusContacted LeadStatus = "CONTACTED"
	LeadStatusWon       LeadStatus = "WON"
	LeadStatusArchived  LeadStatus = "ARCHIVED"
)

type Lead struct {
	ID                  string        `json:"id"`
	UserID              string        `json:"user_id"`
	KeywordID           string        `json:"keyword_id"`
	KeywordName         string        `json:"keyword_name,omitempty"`
	Platform            string        `json:"platform"`
	Subreddit           string        `json:"subreddit,omitempty"`
	ExternalID          string        `json:"external_id"`
	Title               string        `json:"title"`
	Content             string        `json:"content"`
	Author              string        `json:"author"`
	URL                 string        `json:"url"`
	IntentScore         IntentType    `json:"intent_score"`
	ScoreValue          int           `json:"score_value"` // 0 to 100 Buska-style score
	Sentiment           SentimentType `json:"sentiment"`
	UrgencyScore        int           `json:"urgency_score"` // 1 - 10
	AISummary           string        `json:"ai_summary"`
	SuggestedReplyAngle string        `json:"suggested_reply_angle"`
	KeyTakeaways        []string      `json:"key_takeaways"`
	MatchedKeywords     []string      `json:"matched_keywords"`
	Status              LeadStatus    `json:"status"` // Pipeline stage
	Notes               string        `json:"notes,omitempty"`
	IsNotified          bool          `json:"is_notified"`
	CreatedAt           time.Time     `json:"created_at"`
}

type RawLeadPayload struct {
	Platform   string    `json:"platform"`
	Subreddit  string    `json:"subreddit,omitempty"`
	ExternalID string    `json:"external_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Author     string    `json:"author"`
	URL        string    `json:"url"`
	KeywordID  string    `json:"keyword_id"`
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type AIProcessedLead struct {
	RawLeadPayload
	IsNoise             bool          `json:"is_noise"`
	Intent              IntentType    `json:"intent"`
	ScoreValue          int           `json:"score_value"`
	Sentiment           SentimentType `json:"sentiment"`
	UrgencyScore        int           `json:"urgency_score"`
	AISummary           string        `json:"ai_summary"`
	SuggestedReplyAngle string        `json:"suggested_reply_angle"`
	KeyTakeaways        []string      `json:"key_takeaways"`
	MatchedKeywords     []string      `json:"matched_keywords"`
}

type NotificationSettings struct {
	TelegramChatID     string `json:"telegram_chat_id"`
	TelegramBotToken   string `json:"telegram_bot_token"`
	SlackWebhookURL    string `json:"slack_webhook_url"`
	DiscordWebhookURL  string `json:"discord_webhook_url"`
	CustomWebhookURL   string `json:"custom_webhook_url"`
	MinIntentThreshold string `json:"min_intent_threshold"`
	NotifyOnNegative   bool   `json:"notify_on_negative"`
}

type AnalyticsStats struct {
	TotalLeads         int            `json:"total_leads"`
	HighIntentCount    int            `json:"high_intent_count"`
	PositiveCount      int            `json:"positive_count"`
	NeutralCount       int            `json:"neutral_count"`
	NegativeCount      int            `json:"negative_count"`
	NotifiedCount      int            `json:"notified_count"`
	WonLeadsCount      int            `json:"won_leads_count"`
	PlatformBreakdown  map[string]int `json:"platform_breakdown"`
	IntentBreakdown    map[string]int `json:"intent_breakdown"`
	SentimentBreakdown map[string]int `json:"sentiment_breakdown"`
	PipelineBreakdown  map[string]int `json:"pipeline_breakdown"`
	DailyVelocity      []TimeSeries   `json:"daily_velocity"`
	TopKeywords        []KeywordStat  `json:"top_keywords"`
}

type TimeSeries struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type KeywordStat struct {
	Keyword string `json:"keyword"`
	Count   int    `json:"count"`
}

// Auth DTOs
type RegisterRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	CompanyName string `json:"company_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type GoogleAuthRequest struct {
	Credential string `json:"credential"` // Google ID Token
	Email      string `json:"email,omitempty"`
	Name       string `json:"name,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
	GoogleID   string `json:"google_id,omitempty"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}
