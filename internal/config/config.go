package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Config struct {
	RoomsHostAddress string        `env:"REST_ROOMS_HOST_ADDRESS"`
	ServTimeout      time.Duration `env:"SERV_TIMEOUT"`
	DBHost           string        `env:"DB_HOST"`
	DBPort           string        `env:"DB_PORT"`
	DBUser           string        `env:"DB_USER"`
	DBPassword       string        `env:"DB_PASSWORD"`
	DBName           string        `env:"DB_NAME"`
	LogLevel         string        `env:"LOG_LEVEL"`

	AuthGrpcAddr string `env:"AUTH_GRPC_ADDR"`

	UserGrpcAddr string `env:"USER_GRPC_ADDR"`

	MessengerURL string `env:"MESSENGER_INTERNAL_URL"`
	MessengerKey string `env:"MESSENGER_INTERNAL_KEY"`
}

func InitConfig() (*Config, error) {
	_ = godotenv.Load("internal/config/config.env") // игнорируем ошибку — в проде переменные придут из окружения

	conf := Config{}
	if err := env.Parse(&conf); err != nil {
		return nil, err
	}

	if err := conf.validate(); err != nil {
		return nil, err
	}

	return &conf, nil
}

func (c *Config) validate() error {
	required := map[string]string{
		"REST_ROOMS_HOST_ADDRESS": c.RoomsHostAddress,
		"DB_HOST":                 c.DBHost,
		"DB_PORT":                 c.DBPort,
		"DB_USER":                 c.DBUser,
		"DB_PASSWORD":             c.DBPassword,
		"DB_NAME":                 c.DBName,
		"AUTH_GRPC_ADDR":          c.AuthGrpcAddr,
		"USER_GRPC_ADDR":          c.UserGrpcAddr,
	}

	for key, val := range required {
		if val == "" {
			return fmt.Errorf("config: %s is required", key)
		}
	}

	if c.ServTimeout == 0 {
		c.ServTimeout = 5 * time.Second
	}

	return nil
}
