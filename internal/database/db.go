package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alertowls/backend-go/internal/models"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
	mu   sync.RWMutex
}

var GlobalDB *DB

func HashPassword(password string) string {
	hasher := sha256.New()
	hasher.Write([]byte("alertowls_salt_" + password))
	return hex.EncodeToString(hasher.Sum(nil))
}

func InitDB(dbPath string) (*DB, error) {
	if dbPath == "" {
		dbPath = "alertowls.db"
	}

	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	conn, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	if err := db.seedDemoData(); err != nil {
		log.Printf("Warning seeding demo data: %v", err)
	}

	GlobalDB = db
	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			company_name TEXT DEFAULT '',
			avatar_url TEXT DEFAULT '',
			google_id TEXT DEFAULT '',
			password_hash TEXT DEFAULT '',
			plan_tier TEXT DEFAULT 'GROWTH',
			telegram_chat_id TEXT DEFAULT '',
			telegram_bot_token TEXT DEFAULT '',
			slack_webhook_url TEXT DEFAULT '',
			discord_webhook_url TEXT DEFAULT '',
			custom_webhook_url TEXT DEFAULT '',
			min_intent_threshold TEXT DEFAULT 'ALL',
			notify_on_negative INTEGER DEFAULT 1,
			onboarding_complete INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS keywords (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			keyword TEXT NOT NULL,
			category TEXT DEFAULT 'BUYER_LEADS',
			negative_keywords TEXT DEFAULT '[]',
			platforms TEXT DEFAULT '["reddit","hackernews"]',
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS leads (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			keyword_id TEXT,
			platform TEXT NOT NULL,
			subreddit TEXT DEFAULT '',
			external_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			content TEXT DEFAULT '',
			author TEXT DEFAULT '',
			url TEXT NOT NULL,
			intent_score TEXT DEFAULT 'GENERAL_DISCUSSION',
			score_value INTEGER DEFAULT 75,
			sentiment TEXT DEFAULT 'NEUTRAL',
			urgency_score INTEGER DEFAULT 5,
			ai_summary TEXT DEFAULT '',
			suggested_reply_angle TEXT DEFAULT '',
			key_takeaways TEXT DEFAULT '[]',
			matched_keywords TEXT DEFAULT '[]',
			status TEXT DEFAULT 'NEW',
			notes TEXT DEFAULT '',
			is_notified INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (keyword_id) REFERENCES keywords(id) ON DELETE SET NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_leads_user_id ON leads(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_leads_created_at ON leads(created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_leads_external_id ON leads(external_id);`,
		`CREATE INDEX IF NOT EXISTS idx_keywords_user_id ON keywords(user_id);`,
	}

	for _, q := range queries {
		if _, err := db.conn.Exec(q); err != nil {
			return fmt.Errorf("failed executing query (%s): %w", q, err)
		}
	}

	// Schema upgrades / column migrations for existing SQLite DBs
	alterQueries := []string{
		`ALTER TABLE users ADD COLUMN company_name TEXT DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN avatar_url TEXT DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN google_id TEXT DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN onboarding_complete INTEGER DEFAULT 1;`,
		`ALTER TABLE users ADD COLUMN plan_tier TEXT DEFAULT 'GROWTH';`,
		`ALTER TABLE users ADD COLUMN telegram_chat_id TEXT DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN telegram_bot_token TEXT DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN slack_webhook_url TEXT DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN discord_webhook_url TEXT DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN custom_webhook_url TEXT DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN min_intent_threshold TEXT DEFAULT 'ALL';`,
		`ALTER TABLE users ADD COLUMN notify_on_negative INTEGER DEFAULT 1;`,
	}
	for _, aq := range alterQueries {
		_, _ = db.conn.Exec(aq)
	}

	return nil
}

func (db *DB) seedDemoData() error {
	defaultUserID := "00000000-0000-0000-0000-000000000001"
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", defaultUserID).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		hashedPw := HashPassword("password123")
		_, err = db.conn.Exec(`INSERT INTO users (
			id, email, name, company_name, avatar_url, google_id, password_hash, plan_tier,
			telegram_chat_id, telegram_bot_token, slack_webhook_url, discord_webhook_url, custom_webhook_url,
			min_intent_threshold, notify_on_negative, onboarding_complete, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			defaultUserID, "demo@alertowls.com", "Alex Founder", "AlertOwls Inc",
			"https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=100&auto=format&fit=crop&q=80",
			"", hashedPw, "GROWTH", "", "", "", "", "", "ALL", 1, 1, time.Now())
		if err != nil {
			return err
		}

		// Initial seed keywords
		initialKeywords := []struct {
			kw     string
			cat    string
			negKws []string
			plats  []string
		}{
			{
				kw:     "looking for CRM alternative",
				cat:    "BUYER_LEADS",
				negKws: []string{"crack", "torrent"},
				plats:  []string{"reddit", "hackernews"},
			},
			{
				kw:     "social listening tool",
				cat:    "BUYER_LEADS",
				negKws: []string{"pdf"},
				plats:  []string{"reddit", "hackernews"},
			},
			{
				kw:     "AlertOwls",
				cat:    "BRAND_MONITORING",
				negKws: []string{},
				plats:  []string{"reddit", "hackernews"},
			},
			{
				kw:     "Notion vs Obsidian",
				cat:    "COMPETITOR_WATCH",
				negKws: []string{},
				plats:  []string{"reddit", "hackernews"},
			},
			{
				kw:     "hate slow Jira",
				cat:    "PAIN_POINTS",
				negKws: []string{},
				plats:  []string{"reddit", "hackernews"},
			},
		}

		for _, k := range initialKeywords {
			negJSON, _ := json.Marshal(k.negKws)
			platJSON, _ := json.Marshal(k.plats)
			kwID := uuid.New().String()
			_, _ = db.conn.Exec(`INSERT INTO keywords (id, user_id, keyword, category, negative_keywords, platforms, is_active, created_at)
				VALUES (?, ?, ?, ?, ?, ?, 1, ?)`,
				kwID, defaultUserID, k.kw, k.cat, string(negJSON), string(platJSON), time.Now())
		}

		// Seed initial leads with scores 0-100 and pipeline stages
		seedLeads := []struct {
			platform string
			sub      string
			extID    string
			title    string
			content  string
			author   string
			url      string
			intent   models.IntentType
			score    int
			sent     models.SentimentType
			urgency  int
			summary  string
			angle    string
			status   models.LeadStatus
			kwName   string
		}{
			{
				platform: "reddit",
				sub:      "r/SaaS",
				extID:    "rd_seed_201",
				title:    "Need recommendations: Any modern social listening & lead alert tool for Reddit & HN?",
				content:  "We are launching our B2B SaaS next month and want to monitor keywords like competitor mentions and buy signals across Reddit and HackerNews. Zapier is too clunky and Brand24 is overkill. Looking for something fast with instant Telegram alerts. Budget is around $30-100/mo.",
				author:   "growth_dan",
				url:      "https://reddit.com/r/SaaS/comments/sample1",
				intent:   models.IntentHighBuying,
				score:    96,
				sent:     models.SentimentPositive,
				urgency:  9,
				summary:  "Founder is actively evaluating social listening tools with Reddit/HN support and Telegram alerts, with an active budget of $30-100/mo.",
				angle:    "Mention AlertOwls sub-second stream scraping and instant Telegram bot integration. Highlight low noise AI filter.",
				status:   models.LeadStatusNew,
				kwName:   "social listening tool",
			},
			{
				platform: "hackernews",
				sub:      "Ask HN",
				extID:    "hn_seed_202",
				title:    "Ask HN: What is your stack for tracking customer pain points across forums?",
				content:  "I spend 2 hours a day manually reading r/webdev and HN new to see what developers are complaining about. Is there an automated tool that flags negative sentiment and pain points on specific tech stacks?",
				author:   "solotech99",
				url:      "https://news.ycombinator.com/item?id=sample2",
				intent:   models.IntentSeekingRecommendation,
				score:    88,
				sent:     models.SentimentNeutral,
				urgency:  7,
				summary:  "Engineer is seeking automated tooling to replace 2 hours of daily manual social listening for customer pain points.",
				angle:    "Position AlertOwls as an automated intelligence radar that categorizes pain points and filters out noise automatically.",
				status:   models.LeadStatusReviewing,
				kwName:   "social listening tool",
			},
			{
				platform: "reddit",
				sub:      "r/Entrepreneur",
				extID:    "rd_seed_203",
				title:    "Finally ditching HubSpot CRM — too bloated and expensive for a 5 person team",
				content:  "HubSpot just increased our renewal quote by 40%. Looking for a fast CRM alternative that connects directly to inbound social signals and Slack notifications.",
				author:   "mike_bootstrapped",
				url:      "https://reddit.com/r/Entrepreneur/comments/sample3",
				intent:   models.IntentCompetitorMention,
				score:    91,
				sent:     models.SentimentNegative,
				urgency:  8,
				summary:  "Bootstrapped business owner is abandoning HubSpot due to high price and looking for a streamlined CRM alternative with Slack alerts.",
				angle:    "Acknowledge the pricing frustration and suggest lean alternatives or explain how social inbound workflows bridge into lightweight CRMs.",
				status:   models.LeadStatusContacted,
				kwName:   "looking for CRM alternative",
			},
			{
				platform: "reddit",
				sub:      "r/startups",
				extID:    "rd_seed_204",
				title:    "Closed our first 3 enterprise contracts from Reddit inbound leads this week",
				content:  "Replying within 5 minutes of a buyer asking for recommendations converted 3 out of 4 calls. Social listening is undefeated.",
				author:   "sarah_sales",
				url:      "https://reddit.com/r/startups/comments/sample4",
				intent:   models.IntentBrandMention,
				score:    84,
				sent:     models.SentimentPositive,
				urgency:  6,
				summary:  "Sales leader shares success story on fast social inbound response rates converting into enterprise deals.",
				angle:    "Celebrate their win and offer to showcase their workflow in our community spotlight.",
				status:   models.LeadStatusWon,
				kwName:   "AlertOwls",
			},
		}

		for _, sl := range seedLeads {
			takeawaysJSON, _ := json.Marshal([]string{"High intent", "Active buyer", "Decision maker"})
			matchedJSON, _ := json.Marshal([]string{sl.kwName})
			_, _ = db.conn.Exec(`INSERT INTO leads (
				id, user_id, keyword_id, platform, subreddit, external_id, title, content, author, url,
				intent_score, score_value, sentiment, urgency_score, ai_summary, suggested_reply_angle,
				key_takeaways, matched_keywords, status, notes, is_notified, created_at
			) VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', 1, ?)`,
				uuid.New().String(), defaultUserID, sl.platform, sl.sub, sl.extID, sl.title, sl.content, sl.author, sl.url,
				sl.intent, sl.score, sl.sent, sl.urgency, sl.summary, sl.angle,
				string(takeawaysJSON), string(matchedJSON), sl.status, time.Now().Add(-15*time.Minute))
		}
	}
	return nil
}

// User CRUD & Auth
func (db *DB) GetUser(userID string) (*models.User, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var u models.User
	var notifyNeg, onbComp int
	err := db.conn.QueryRow(`SELECT id, email, name, company_name, avatar_url, google_id, password_hash, plan_tier,
		telegram_chat_id, telegram_bot_token, slack_webhook_url, discord_webhook_url, custom_webhook_url,
		min_intent_threshold, notify_on_negative, onboarding_complete, created_at
		FROM users WHERE id = ?`, userID).Scan(
		&u.ID, &u.Email, &u.Name, &u.CompanyName, &u.AvatarURL, &u.GoogleID, &u.PasswordHash, &u.PlanTier,
		&u.TelegramChatID, &u.TelegramBotToken, &u.SlackWebhookURL, &u.DiscordWebhookURL, &u.CustomWebhookURL,
		&u.MinIntentThreshold, &notifyNeg, &onbComp, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.NotifyOnNegative = notifyNeg == 1
	u.OnboardingComplete = onbComp == 1
	return &u, nil
}

func (db *DB) GetUserByEmail(email string) (*models.User, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var u models.User
	var notifyNeg, onbComp int
	err := db.conn.QueryRow(`SELECT id, email, name, company_name, avatar_url, google_id, password_hash, plan_tier,
		telegram_chat_id, telegram_bot_token, slack_webhook_url, discord_webhook_url, custom_webhook_url,
		min_intent_threshold, notify_on_negative, onboarding_complete, created_at
		FROM users WHERE LOWER(email) = LOWER(?)`, email).Scan(
		&u.ID, &u.Email, &u.Name, &u.CompanyName, &u.AvatarURL, &u.GoogleID, &u.PasswordHash, &u.PlanTier,
		&u.TelegramChatID, &u.TelegramBotToken, &u.SlackWebhookURL, &u.DiscordWebhookURL, &u.CustomWebhookURL,
		&u.MinIntentThreshold, &notifyNeg, &onbComp, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.NotifyOnNegative = notifyNeg == 1
	u.OnboardingComplete = onbComp == 1
	return &u, nil
}

func (db *DB) CreateUser(email, name, password, companyName, avatarURL, googleID string) (*models.User, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	id := uuid.New().String()
	hashedPw := ""
	if password != "" {
		hashedPw = HashPassword(password)
	}

	now := time.Now()
	_, err := db.conn.Exec(`INSERT INTO users (
		id, email, name, company_name, avatar_url, google_id, password_hash, plan_tier,
		telegram_chat_id, telegram_bot_token, slack_webhook_url, discord_webhook_url, custom_webhook_url,
		min_intent_threshold, notify_on_negative, onboarding_complete, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, 'GROWTH', '', '', '', '', '', 'ALL', 1, 0, ?)`,
		id, email, name, companyName, avatarURL, googleID, hashedPw, now,
	)
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:                 id,
		Email:              email,
		Name:               name,
		CompanyName:        companyName,
		AvatarURL:          avatarURL,
		GoogleID:           googleID,
		PlanTier:           models.PlanGrowth,
		OnboardingComplete: false,
		CreatedAt:          now,
	}, nil
}

func (db *DB) CompleteOnboarding(userID string, companyName string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.conn.Exec(`UPDATE users SET onboarding_complete = 1, company_name = COALESCE(NULLIF(?, ''), company_name) WHERE id = ?`, companyName, userID)
	return err
}

func (db *DB) GetDefaultUser() (*models.User, error) {
	return db.GetUser("00000000-0000-0000-0000-000000000001")
}

func (db *DB) UpdateNotificationSettings(userID string, settings models.NotificationSettings) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	notifyNeg := 0
	if settings.NotifyOnNegative {
		notifyNeg = 1
	}

	_, err := db.conn.Exec(`UPDATE users SET 
		telegram_chat_id = ?, 
		telegram_bot_token = ?, 
		slack_webhook_url = ?, 
		discord_webhook_url = ?, 
		custom_webhook_url = ?, 
		min_intent_threshold = ?,
		notify_on_negative = ?
		WHERE id = ?`,
		settings.TelegramChatID, settings.TelegramBotToken,
		settings.SlackWebhookURL, settings.DiscordWebhookURL, settings.CustomWebhookURL,
		settings.MinIntentThreshold, notifyNeg, userID,
	)
	return err
}

func (db *DB) UpdateUserPlanTier(userID string, tier models.PlanTier) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.conn.Exec(`UPDATE users SET plan_tier = ? WHERE id = ?`, tier, userID)
	return err
}

// Keywords
func (db *DB) GetKeywords(userID string) ([]models.Keyword, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.conn.Query(`
		SELECT k.id, k.user_id, k.keyword, k.category, k.negative_keywords, k.platforms, k.is_active, k.created_at,
		       (SELECT COUNT(*) FROM leads l WHERE l.keyword_id = k.id OR l.matched_keywords LIKE '%' || k.keyword || '%') as leads_count
		FROM keywords k 
		WHERE k.user_id = ? 
		ORDER BY k.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Keyword
	for rows.Next() {
		var k models.Keyword
		var negJSON, platJSON string
		var isActiveInt int
		if err := rows.Scan(&k.ID, &k.UserID, &k.Keyword, &k.Category, &negJSON, &platJSON, &isActiveInt, &k.CreatedAt, &k.LeadsCount); err != nil {
			return nil, err
		}
		k.IsActive = isActiveInt == 1
		_ = json.Unmarshal([]byte(negJSON), &k.NegativeKeywords)
		_ = json.Unmarshal([]byte(platJSON), &k.Platforms)
		list = append(list, k)
	}
	return list, nil
}

func (db *DB) GetAllActiveKeywords() ([]models.Keyword, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.conn.Query(`SELECT id, user_id, keyword, category, negative_keywords, platforms, is_active, created_at FROM keywords WHERE is_active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Keyword
	for rows.Next() {
		var k models.Keyword
		var negJSON, platJSON string
		var isActiveInt int
		if err := rows.Scan(&k.ID, &k.UserID, &k.Keyword, &k.Category, &negJSON, &platJSON, &isActiveInt, &k.CreatedAt); err != nil {
			return nil, err
		}
		k.IsActive = isActiveInt == 1
		_ = json.Unmarshal([]byte(negJSON), &k.NegativeKeywords)
		_ = json.Unmarshal([]byte(platJSON), &k.Platforms)
		list = append(list, k)
	}
	return list, nil
}

func (db *DB) CreateKeyword(userID string, keyword string, category string, negativeKeywords []string, platforms []string) (*models.Keyword, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	id := uuid.New().String()
	negJSON, _ := json.Marshal(negativeKeywords)
	platJSON, _ := json.Marshal(platforms)
	now := time.Now()

	_, err := db.conn.Exec(`INSERT INTO keywords (id, user_id, keyword, category, negative_keywords, platforms, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?)`,
		id, userID, keyword, category, string(negJSON), string(platJSON), now)
	if err != nil {
		return nil, err
	}

	return &models.Keyword{
		ID:               id,
		UserID:           userID,
		Keyword:          keyword,
		Category:         category,
		NegativeKeywords: negativeKeywords,
		Platforms:        platforms,
		IsActive:         true,
		CreatedAt:        now,
	}, nil
}

func (db *DB) ToggleKeyword(id, userID string, isActive bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	activeInt := 0
	if isActive {
		activeInt = 1
	}
	_, err := db.conn.Exec(`UPDATE keywords SET is_active = ? WHERE id = ? AND user_id = ?`, activeInt, id, userID)
	return err
}

func (db *DB) DeleteKeyword(id, userID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.conn.Exec(`DELETE FROM keywords WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// Leads & Pipeline
func (db *DB) LeadExists(externalID string) (bool, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM leads WHERE external_id = ?`, externalID).Scan(&count)
	return count > 0, err
}

func (db *DB) SaveLead(lead *models.AIProcessedLead) (*models.Lead, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	leadID := uuid.New().String()
	takeawaysJSON, _ := json.Marshal(lead.KeyTakeaways)
	matchedJSON, _ := json.Marshal(lead.MatchedKeywords)
	now := time.Now()

	scoreVal := lead.ScoreValue
	if scoreVal <= 0 {
		scoreVal = lead.UrgencyScore * 10
	}

	_, err := db.conn.Exec(`INSERT INTO leads (
		id, user_id, keyword_id, platform, subreddit, external_id, title, content, author, url,
		intent_score, score_value, sentiment, urgency_score, ai_summary, suggested_reply_angle, key_takeaways, matched_keywords,
		status, notes, is_notified, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'NEW', '', 1, ?)`,
		leadID, lead.UserID, sqlNullString(lead.KeywordID), lead.Platform, lead.Subreddit,
		lead.ExternalID, lead.Title, lead.Content, lead.Author, lead.URL,
		lead.Intent, scoreVal, lead.Sentiment, lead.UrgencyScore, lead.AISummary, lead.SuggestedReplyAngle,
		string(takeawaysJSON), string(matchedJSON), now,
	)
	if err != nil {
		return nil, err
	}

	return &models.Lead{
		ID:                  leadID,
		UserID:              lead.UserID,
		KeywordID:           lead.KeywordID,
		Platform:            lead.Platform,
		Subreddit:           lead.Subreddit,
		ExternalID:          lead.ExternalID,
		Title:               lead.Title,
		Content:             lead.Content,
		Author:              lead.Author,
		URL:                 lead.URL,
		IntentScore:         lead.Intent,
		ScoreValue:          scoreVal,
		Sentiment:           lead.Sentiment,
		UrgencyScore:        lead.UrgencyScore,
		AISummary:           lead.AISummary,
		SuggestedReplyAngle: lead.SuggestedReplyAngle,
		KeyTakeaways:        lead.KeyTakeaways,
		MatchedKeywords:     lead.MatchedKeywords,
		Status:              models.LeadStatusNew,
		IsNotified:          true,
		CreatedAt:           now,
	}, nil
}

func (db *DB) GetLeads(userID string, intent string, sentiment string, platform string, status string, search string, limit, offset int) ([]models.Lead, int, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	whereClauses := []string{"l.user_id = ?"}
	args := []interface{}{userID}

	if intent != "" && intent != "ALL" {
		whereClauses = append(whereClauses, "l.intent_score = ?")
		args = append(args, intent)
	}
	if sentiment != "" && sentiment != "ALL" {
		whereClauses = append(whereClauses, "l.sentiment = ?")
		args = append(args, sentiment)
	}
	if platform != "" && platform != "ALL" {
		whereClauses = append(whereClauses, "l.platform = ?")
		args = append(args, platform)
	}
	if status != "" && status != "ALL" {
		whereClauses = append(whereClauses, "l.status = ?")
		args = append(args, status)
	}
	if search != "" {
		whereClauses = append(whereClauses, "(l.title LIKE ? OR l.content LIKE ? OR l.ai_summary LIKE ?)")
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	whereSql := strings.Join(whereClauses, " AND ")

	countSql := fmt.Sprintf("SELECT COUNT(*) FROM leads l WHERE %s", whereSql)
	var total int
	err := db.conn.QueryRow(countSql, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	querySql := fmt.Sprintf(`SELECT l.id, l.user_id, COALESCE(l.keyword_id, ''), l.platform, l.subreddit, l.external_id, l.title, l.content, l.author, l.url,
		l.intent_score, l.score_value, l.sentiment, l.urgency_score, l.ai_summary, l.suggested_reply_angle, l.key_takeaways, l.matched_keywords,
		l.status, COALESCE(l.notes, ''), l.is_notified, l.created_at, COALESCE(k.keyword, '') as keyword_name
		FROM leads l
		LEFT JOIN keywords k ON l.keyword_id = k.id
		WHERE %s
		ORDER BY l.created_at DESC
		LIMIT ? OFFSET ?`, whereSql)

	pageArgs := append(args, limit, offset)
	rows, err := db.conn.Query(querySql, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var leads []models.Lead
	for rows.Next() {
		var l models.Lead
		var takeJSON, matchJSON string
		var isNotifiedInt int
		if err := rows.Scan(
			&l.ID, &l.UserID, &l.KeywordID, &l.Platform, &l.Subreddit, &l.ExternalID, &l.Title, &l.Content, &l.Author, &l.URL,
			&l.IntentScore, &l.ScoreValue, &l.Sentiment, &l.UrgencyScore, &l.AISummary, &l.SuggestedReplyAngle, &takeJSON, &matchJSON,
			&l.Status, &l.Notes, &isNotifiedInt, &l.CreatedAt, &l.KeywordName,
		); err != nil {
			return nil, 0, err
		}
		l.IsNotified = isNotifiedInt == 1
		_ = json.Unmarshal([]byte(takeJSON), &l.KeyTakeaways)
		_ = json.Unmarshal([]byte(matchJSON), &l.MatchedKeywords)
		leads = append(leads, l)
	}

	return leads, total, nil
}

func (db *DB) UpdateLeadStage(leadID, userID string, status models.LeadStatus, notes string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.conn.Exec(`UPDATE leads SET status = ?, notes = COALESCE(NULLIF(?, ''), notes) WHERE id = ? AND user_id = ?`, status, notes, leadID, userID)
	return err
}

func (db *DB) DeleteLead(leadID, userID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.conn.Exec(`DELETE FROM leads WHERE id = ? AND user_id = ?`, leadID, userID)
	return err
}

// Analytics
func (db *DB) GetAnalytics(userID string) (*models.AnalyticsStats, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := &models.AnalyticsStats{
		PlatformBreakdown:  make(map[string]int),
		IntentBreakdown:    make(map[string]int),
		SentimentBreakdown: make(map[string]int),
		PipelineBreakdown:  make(map[string]int),
		DailyVelocity:      make([]models.TimeSeries, 0),
		TopKeywords:        make([]models.KeywordStat, 0),
	}

	_ = db.conn.QueryRow(`SELECT 
		COUNT(*),
		SUM(CASE WHEN intent_score IN ('HIGH_BUYING', 'SEEKING_RECOMMENDATION') THEN 1 ELSE 0 END),
		SUM(CASE WHEN sentiment = 'POSITIVE' THEN 1 ELSE 0 END),
		SUM(CASE WHEN sentiment = 'NEUTRAL' THEN 1 ELSE 0 END),
		SUM(CASE WHEN sentiment = 'NEGATIVE' THEN 1 ELSE 0 END),
		SUM(CASE WHEN is_notified = 1 THEN 1 ELSE 0 END),
		SUM(CASE WHEN status = 'WON' THEN 1 ELSE 0 END)
		FROM leads WHERE user_id = ?`, userID).Scan(
		&stats.TotalLeads,
		&stats.HighIntentCount,
		&stats.PositiveCount,
		&stats.NeutralCount,
		&stats.NegativeCount,
		&stats.NotifiedCount,
		&stats.WonLeadsCount,
	)

	// Platform breakdown
	pRows, err := db.conn.Query(`SELECT platform, COUNT(*) FROM leads WHERE user_id = ? GROUP BY platform`, userID)
	if err == nil {
		for pRows.Next() {
			var p string
			var c int
			if err := pRows.Scan(&p, &c); err == nil {
				stats.PlatformBreakdown[p] = c
			}
		}
		pRows.Close()
	}

	// Intent breakdown
	iRows, err := db.conn.Query(`SELECT intent_score, COUNT(*) FROM leads WHERE user_id = ? GROUP BY intent_score`, userID)
	if err == nil {
		for iRows.Next() {
			var i string
			var c int
			if err := iRows.Scan(&i, &c); err == nil {
				stats.IntentBreakdown[i] = c
			}
		}
		iRows.Close()
	}

	// Pipeline breakdown
	plRows, err := db.conn.Query(`SELECT status, COUNT(*) FROM leads WHERE user_id = ? GROUP BY status`, userID)
	if err == nil {
		for plRows.Next() {
			var s string
			var c int
			if err := plRows.Scan(&s, &c); err == nil {
				stats.PipelineBreakdown[s] = c
			}
		}
		plRows.Close()
	}

	// Sentiment breakdown
	sRows, err := db.conn.Query(`SELECT sentiment, COUNT(*) FROM leads WHERE user_id = ? GROUP BY sentiment`, userID)
	if err == nil {
		for sRows.Next() {
			var s string
			var c int
			if err := sRows.Scan(&s, &c); err == nil {
				stats.SentimentBreakdown[s] = c
			}
		}
		sRows.Close()
	}

	// Daily velocity (last 7 days)
	vRows, err := db.conn.Query(`SELECT DATE(created_at) as day, COUNT(*) 
		FROM leads 
		WHERE user_id = ? AND created_at >= DATETIME('now', '-7 days')
		GROUP BY DATE(created_at) 
		ORDER BY day ASC`, userID)
	if err == nil {
		for vRows.Next() {
			var ts models.TimeSeries
			if err := vRows.Scan(&ts.Date, &ts.Count); err == nil {
				stats.DailyVelocity = append(stats.DailyVelocity, ts)
			}
		}
		vRows.Close()
	}

	// Top matched keywords
	kwRows, err := db.conn.Query(`SELECT k.keyword, COUNT(l.id) as cnt
		FROM keywords k
		LEFT JOIN leads l ON (l.keyword_id = k.id OR l.matched_keywords LIKE '%' || k.keyword || '%')
		WHERE k.user_id = ?
		GROUP BY k.id
		ORDER BY cnt DESC
		LIMIT 5`, userID)
	if err == nil {
		for kwRows.Next() {
			var ks models.KeywordStat
			if err := kwRows.Scan(&ks.Keyword, &ks.Count); err == nil {
				stats.TopKeywords = append(stats.TopKeywords, ks)
			}
		}
		kwRows.Close()
	}

	return stats, nil
}

func sqlNullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
