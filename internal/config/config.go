package config

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	RedisAddr   string        `mapstructure:"REDIS_ADDR"`
	RedisPass   string        `mapstructure:"REDIS_PASSWORD"`
	RedisDB     int           `mapstructure:"REDIS_DB"`
	UpstreamURL string        `mapstructure:"UPSTREAM_URL"`
	HTTPPort    string        `mapstructure:"HTTP_PORT"`
	JWTSecret   string        `mapstructure:"JWT_SECRET"`
	JWTExpiry   time.Duration `mapstructure:"JWT_EXPIRY"`
	LogLevel    string        `mapstructure:"LOG_LEVEL"`
	MaxSymbols  int           `mapstructure:"MAX_SYMBOLS_MEMORY"`
}

func Load() *Config {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln("No .env file found:", err)
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		log.Fatalln("Config unmarshal:", err)
	}

	if cfg.JWTExpiry == 0 {
		cfg.JWTExpiry = 24 * time.Hour
	}
	if cfg.MaxSymbols == 0 {
		cfg.MaxSymbols = 1000
	}
	if cfg.HTTPPort == "" {
		cfg.HTTPPort = ":8080"
	}

	return cfg
}
