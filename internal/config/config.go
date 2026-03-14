package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
	"github.com/meindokuse/transaction-module/pkg/connect/postgres"
	"github.com/meindokuse/transaction-module/pkg/connect/redis"
)

// Config - корневая структура настроек
type Config struct {
	Env    string          `yaml:"env"`
	Server HTTPServer      `yaml:"server"`
	Kafka  KafkaConfig     `yaml:"kafka"`
	DB     postgres.Config `yaml:"db"`
	Redis  redis.Config    `yaml:"redis"`
}

// HTTPServer - настройки REST API
type HTTPServer struct {
	Address      string        `yaml:"address" env:"HTTP_ADDRESS" env-default:":8080"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env-default:"5s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env-default:"10s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

// KafkaConfig содержит настройки и консьюмера, и DLQ-продюсера
type KafkaConfig struct {
	Consumer ConsumerConfig `yaml:"consumer"`
	Producer ProducerConfig `yaml:"producer"`
}

type ConsumerConfig struct {
	Brokers        []string      `env:"KAFKA_BROKERS" env-required:"true"`
	Topic          string        `yaml:"topic" env-default:"casino.transactions.v1"`
	GroupID        string        `yaml:"group_id" env-default:"casino-tx-consumer-group"`
	WorkersCount   int           `yaml:"workers_count" env-default:"32"`
	MinBytes       int           `yaml:"min_bytes" env-default:"10000"`
	MaxBytes       int           `yaml:"max_bytes" env-default:"10000000"`
	MaxWait        time.Duration `yaml:"max_wait" env-default:"1s"`
	ReadBackoffMin time.Duration `yaml:"read_backoff_min" env-default:"100ms"`
	ReadBackoffMax time.Duration `yaml:"read_backoff_max" env-default:"1s"`
	RetryMaxDelay  time.Duration `yaml:"retry_max_delay" env-default:"5s"`
	MaxRetries     int           `yaml:"max_retries" env-default:"3"`
	BatchSize      int           `yaml:"batch_size" env-default:"100"`
	BatchTimeout   time.Duration `yaml:"batch_timeout" env-default:"50ms"`
	MaxDlqRetries int			 `yaml:"max_dlq_retries" env-default:"3"`
}

type ProducerConfig struct {
	Brokers      []string      `env:"KAFKA_BROKERS" env-required:"true"`
	DLQTopic     string        `yaml:"dlq_topic" env-default:"casino.transactions.dlq.v1"`
	BatchTimeout time.Duration `yaml:"batch_timeout" env-default:"10ms"`
	BatchSize    int           `yaml:"batch_size" env-default:"100"`
	MaxAttempts  int           `yaml:"max_attempts" env-default:"3"`
	Compression  string        `yaml:"compression" env-default:"snappy"`
}


func Load() *Config {
    _ = godotenv.Load()

    var cfg Config

    configPath := os.Getenv("CONFIG_PATH")
    if configPath == "" {
        configPath = "config/config.yaml" 
    }

    if _, err := os.Stat(configPath); err == nil {
        if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
            help, _ := cleanenv.GetDescription(&cfg, nil)
            slog.Error("cannot read config file", "err", err, "help", help)
            os.Exit(1)
        }
        slog.Info("config loaded from file and env", "path", configPath)
    } else {
        // prod
        if err := cleanenv.ReadEnv(&cfg); err != nil {
            help, _ := cleanenv.GetDescription(&cfg, nil)
            slog.Error("cannot read env variables", "err", err, "help", help)
            os.Exit(1)
        }
        slog.Info("config loaded from env variables only")
    }

    return &cfg
}
