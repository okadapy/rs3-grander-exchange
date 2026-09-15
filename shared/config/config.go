package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	ServiceName string         `mapstructure:"service_name"`
	Port        int            `mapstructure:"port"`
	LogLevel    string         `mapstructure:"log_level"`
	Env         string         `mapstructure:"env"`
	MySQL       MySQLConfig    `mapstructure:"mysql"`
	Redis       RedisConfig    `mapstructure:"redis"`
	WeirdGloop  WeirdGloopConf `mapstructure:"weirdgloop"`
	Hiscore     HiscoreConf    `mapstructure:"hiscore"`
	Poller      PollerConf     `mapstructure:"poller"`
	Services    ServicesConf   `mapstructure:"services"`
	JWT         JWTConf        `mapstructure:"jwt"`
	Chat        ChatConf       `mapstructure:"chat"`
	Market      MarketConf     `mapstructure:"market"`

	// Gateway-only fields
	Routes    []RouteConfig   `mapstructure:"routes"`
	CORS      CORSConfig      `mapstructure:"cors"`
	RateLimit RateLimitConfig `mapstructure:"ratelimit"`
}

type MySQLConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"db_name"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type WeirdGloopConf struct {
	BaseURL     string        `mapstructure:"base_url"`
	UserAgent   string        `mapstructure:"user_agent"`
	BatchSize   int           `mapstructure:"batch_size"`
	MinGap      time.Duration `mapstructure:"min_gap"`
	HTTPTimeout time.Duration `mapstructure:"http_timeout"`
}

type HiscoreConf struct {
	BaseURL  string        `mapstructure:"base_url"`
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
}

type PollerConf struct {
	PollInterval time.Duration `mapstructure:"poll_interval"`
}

// MarketConf holds the trading assumptions that turn raw Grand Exchange
// guide prices into numbers someone can act on. Every one of these is a
// modelling choice rather than a fact we scrape, so they live in config
// and are echoed back in API responses — a GP/h figure is only
// meaningful next to the assumptions that produced it.
type MarketConf struct {
	// SpreadPct is the assumed round-trip gap between what you actually
	// pay to buy and what you actually receive to sell, as a percentage
	// of the guide price. RS3 publishes no bid/ask, so this is an
	// explicit assumption: buys are modelled at price*(1+spread/2),
	// sells at price*(1-spread/2). 0 reproduces the old behaviour of
	// pretending you can transact at the guide price.
	SpreadPct float64 `mapstructure:"spread_pct"`

	// TaxPct is the Grand Exchange sales tax applied to the whole sale
	// value, TaxCapPerItem caps it per item sold, and TaxExemptBelow
	// skips the tax for items cheaper than the threshold.
	TaxPct         float64 `mapstructure:"tax_pct"`
	TaxCapPerItem  int64   `mapstructure:"tax_cap_per_item"`
	TaxExemptBelow int64   `mapstructure:"tax_exempt_below"`

	// DefaultActionsPerHour is the last-resort throughput used when a
	// recipe has neither a wiki value nor a per-skill default.
	DefaultActionsPerHour int `mapstructure:"default_actions_per_hour"`
}

type ServicesConf struct {
	RecipeURL  string `mapstructure:"recipe_url"`
	PriceURL   string `mapstructure:"price_url"`
	HiscoreURL string `mapstructure:"hiscore_url"`
}

type JWTConf struct {
	Secret string        `mapstructure:"secret"`
	Expiry time.Duration `mapstructure:"expiry"`
}

type ChatConf struct {
	RateLimitWindow time.Duration `mapstructure:"rate_limit_window"`
	HistoryLimit    int           `mapstructure:"history_limit"`
}

// ---------------- gateway-only ----------------

type RouteConfig struct {
	Prefix      string        `mapstructure:"prefix"`
	Target      string        `mapstructure:"target"`
	WebSocket   bool          `mapstructure:"websocket"`
	Timeout     time.Duration `mapstructure:"timeout"`
	StripPrefix bool          `mapstructure:"strip_prefix"`
}

type CORSConfig struct {
	AllowedOrigins []string      `mapstructure:"allowed_origins"`
	AllowedMethods []string      `mapstructure:"allowed_methods"`
	AllowedHeaders []string      `mapstructure:"allowed_headers"`
	ExposeHeaders  []string      `mapstructure:"expose_headers"`
	MaxAge         time.Duration `mapstructure:"max_age"`
	AllowCreds     bool          `mapstructure:"allow_credentials"`
}

type RateLimitConfig struct {
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
	Burst             int     `mapstructure:"burst"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("RS3")
	v.AutomaticEnv()

	_ = v.BindEnv("env", "RS3_ENV")
	_ = v.BindEnv("mysql.host", "RS3_DB_HOST")
	_ = v.BindEnv("mysql.password", "RS3_DB_PASSWORD")
	_ = v.BindEnv("redis.addr", "RS3_REDIS_ADDR")
	_ = v.BindEnv("jwt.secret", "RS3_JWT_SECRET")

	v.SetDefault("market.spread_pct", 2.0)
	v.SetDefault("market.tax_pct", 2.0)
	v.SetDefault("market.tax_cap_per_item", 5_000_000)
	v.SetDefault("market.tax_exempt_below", 0)
	v.SetDefault("market.default_actions_per_hour", 600)

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
