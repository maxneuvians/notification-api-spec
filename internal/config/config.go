package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type jsonStringMap map[string]string

func (m *jsonStringMap) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*m = jsonStringMap{}
		return nil
	}

	var parsed map[string]string
	if err := json.Unmarshal(text, &parsed); err != nil {
		return err
	}

	*m = parsed
	return nil
}

type Config struct {
	DatabaseURI       string        `env:"DATABASE_URI"`
	DatabaseReaderURI string        `env:"DATABASE_READER_URI"`
	DBPoolSize        int           `env:"POOL_SIZE" envDefault:"5"`
	DBPoolRecycle     time.Duration `env:"-"`

	RedisURL     string `env:"REDIS_URL"`
	CacheOpsURL  string `env:"CACHE_OPS_URL"`
	RedisEnabled bool   `env:"REDIS_ENABLED" envDefault:"false"`

	AWSRegion                string   `env:"AWS_REGION" envDefault:"us-east-1"`
	AWSSESRegion             string   `env:"AWS_SES_REGION" envDefault:"us-east-1"`
	AWSSESAccessKey          string   `env:"AWS_SES_ACCESS_KEY"`
	AWSSESSecretKey          string   `env:"AWS_SES_SECRET_KEY"`
	AWSPinpointRegion        string   `env:"AWS_PINPOINT_REGION" envDefault:"us-west-2"`
	AWSPinpointSCPoolID      string   `env:"AWS_PINPOINT_SC_POOL_ID"`
	AWSPinpointDefaultPoolID string   `env:"AWS_PINPOINT_DEFAULT_POOL_ID"`
	AWSPinpointConfigSet     string   `env:"AWS_PINPOINT_CONFIGURATION_SET_NAME" envDefault:"pinpoint-configuration"`
	AWSPinpointSCTemplateIDs []string `env:"AWS_PINPOINT_SC_TEMPLATE_IDS" envSeparator:","`
	AWSUSTollFreeNumber      string   `env:"AWS_US_TOLL_FREE_NUMBER"`
	CSVUploadBucket          string   `env:"CSV_UPLOAD_BUCKET_NAME"`
	ReportsBucket            string   `env:"REPORTS_BUCKET_NAME"`
	GCOrganisationsBucket    string   `env:"GC_ORGANISATIONS_BUCKET_NAME"`
	GCOrganisationsFilename  string   `env:"GC_ORGANISATIONS_FILENAME" envDefault:"all.json"`

	NotificationQueuePrefix     string        `env:"NOTIFICATION_QUEUE_PREFIX"`
	CeleryDeliverSMSRateLimit   string        `env:"CELERY_DELIVER_SMS_RATE_LIMIT" envDefault:"1/s"`
	BatchInsertionChunkSize     int           `env:"BATCH_INSERTION_CHUNK_SIZE" envDefault:"500"`
	SMSWorkerConcurrency        int           `env:"CELERY_CONCURRENCY" envDefault:"4"`
	SendingNotificationsTimeout time.Duration `env:"-"`

	AdminBaseURL            string   `env:"ADMIN_BASE_URL" envDefault:"http://localhost:6012"`
	AdminClientSecret       string   `env:"ADMIN_CLIENT_SECRET"`
	AdminClientUserName     string   `env:"-"`
	SecretKey               []string `env:"SECRET_KEY" envSeparator:","`
	SecretKeys              []string `env:"-"`
	DangerousSalt           string   `env:"DANGEROUS_SALT"`
	SREClientSecret         string   `env:"SRE_CLIENT_SECRET"`
	SREUserName             string   `env:"SRE_USER_NAME"`
	CacheClearClientSecret  string   `env:"CACHE_CLEAR_CLIENT_SECRET"`
	CacheClearUserName      string   `env:"CACHE_CLEAR_USER_NAME"`
	CypressAuthClientSecret string   `env:"CYPRESS_AUTH_CLIENT_SECRET"`
	CypressAuthUserName     string   `env:"CYPRESS_AUTH_USER_NAME"`
	CypressUserPWSecret     string   `env:"CYPRESS_USER_PW_SECRET"`
	APIKeyPrefix            string   `env:"-"`

	FFUseBillableUnits        bool  `env:"FF_USE_BILLABLE_UNITS" envDefault:"false"`
	FFSalesforceContact       bool  `env:"FF_SALESFORCE_CONTACT" envDefault:"false"`
	FFUsePinpointForDedicated bool  `env:"FF_USE_PINPOINT_FOR_DEDICATED" envDefault:"false"`
	FFBounceRateSeedEpochMs   int64 `env:"FF_BOUNCE_RATE_SEED_EPOCH_MS" envDefault:"0"`
	FFPTServiceSkipFreshdesk  bool  `env:"FF_PT_SERVICE_SKIP_FRESHDESK" envDefault:"false"`
	FFEnableOtel              bool  `env:"FF_ENABLE_OTEL" envDefault:"false"`

	FreshDeskAPIURL               string `env:"FRESH_DESK_API_URL"`
	FreshDeskAPIKey               string `env:"FRESH_DESK_API_KEY"`
	FreshDeskProductID            string `env:"FRESH_DESK_PRODUCT_ID"`
	FreshDeskEnabled              bool   `env:"FRESH_DESK_ENABLED" envDefault:"false"`
	AirtableAPIKey                string `env:"AIRTABLE_API_KEY"`
	AirtableNewsletterBaseID      string `env:"AIRTABLE_NEWSLETTER_BASE_ID"`
	AirtableNewsletterTableName   string `env:"AIRTABLE_NEWSLETTER_TABLE_NAME"`
	AirtableCurrentTemplatesTable string `env:"AIRTABLE_CURRENT_NEWSLETTER_TEMPLATES_TABLE_NAME"`
	SalesforceDomain              string `env:"SALESFORCE_DOMAIN"`
	SalesforceClientID            string `env:"SALESFORCE_CLIENT_ID"`
	SalesforceUsername            string `env:"SALESFORCE_USERNAME"`
	SalesforcePassword            string `env:"SALESFORCE_PASSWORD"`
	SalesforceSecurityToken       string `env:"SALESFORCE_SECURITY_TOKEN"`

	NotifyEnvironment         string        `env:"NOTIFY_ENVIRONMENT" envDefault:"development"`
	APIHostName               string        `env:"API_HOST_NAME"`
	DocumentationDomain       string        `env:"DOCUMENTATION_DOMAIN" envDefault:"documentation.notification.canada.ca"`
	InvitationExpirationDays  int           `env:"-"`
	PageSize                  int           `env:"-"`
	APIPageSize               int           `env:"-"`
	MaxVerifyCodeCount        int           `env:"-"`
	JobsMaxScheduleHoursAhead int           `env:"-"`
	FailedLoginLimit          int           `env:"FAILED_LOGIN_LIMIT" envDefault:"10"`
	AttachmentNumLimit        int           `env:"ATTACHMENT_NUM_LIMIT" envDefault:"10"`
	AttachmentSizeLimit       int           `env:"ATTACHMENT_SIZE_LIMIT" envDefault:"10485760"`
	PersonalisationSizeLimit  int           `env:"PERSONALISATION_SIZE_LIMIT" envDefault:"51200"`
	AllowHTMLServiceIDs       []string      `env:"ALLOW_HTML_SERVICE_IDS" envSeparator:","`
	DaysBeforeReportsExpire   int           `env:"DAYS_BEFORE_REPORTS_EXPIRE" envDefault:"3"`
	StatsDHost                string        `env:"STATSD_HOST"`
	StatsDPort                int           `env:"-"`
	StatsDEnabled             bool          `env:"-"`
	OtelRequestMetricsEnabled bool          `env:"OTEL_REQUEST_METRICS_ENABLED" envDefault:"false"`
	CronitorEnabled           bool          `env:"-"`
	CronitorKeys              jsonStringMap `env:"CRONITOR_KEYS"`
	ScanForPII                bool          `env:"SCAN_FOR_PII" envDefault:"false"`
	CSVBulkRedirectThreshold  int           `env:"CSV_BULK_REDIRECT_THRESHOLD"`

	RateLimitPerSecond float64 `env:"RATE_LIMIT_PER_SECOND" envDefault:"10"`
	RateLimitBurst     int     `env:"RATE_LIMIT_BURST" envDefault:"20"`
	Port               string  `env:"PORT" envDefault:"8080"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	required := []string{
		"DATABASE_URI",
		"ADMIN_CLIENT_SECRET",
		"SECRET_KEY",
		"DANGEROUS_SALT",
	}

	missing := make([]string, 0)
	for _, key := range required {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	cfg.DBPoolRecycle = 5 * time.Minute
	cfg.SendingNotificationsTimeout = 72 * time.Hour
	cfg.AdminClientUserName = AdminClientUserName
	cfg.APIKeyPrefix = APIKeyPrefix
	cfg.InvitationExpirationDays = 2
	cfg.PageSize = 50
	cfg.APIPageSize = 250
	cfg.MaxVerifyCodeCount = 10
	cfg.JobsMaxScheduleHoursAhead = 96
	cfg.StatsDPort = 8125
	cfg.StatsDEnabled = strings.TrimSpace(cfg.StatsDHost) != ""
	cfg.CronitorEnabled = false
	cfg.SecretKeys = append([]string(nil), cfg.SecretKey...)

	if cfg.CacheOpsURL == "" {
		cfg.CacheOpsURL = cfg.RedisURL
	}

	return &cfg, nil
}
