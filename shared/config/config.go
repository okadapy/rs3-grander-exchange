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

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
