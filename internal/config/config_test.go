package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLoadFromFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`
env: "test"
server:
  address: ":9000"
db:
  dsn: "postgres://file"
redis:
  addr: "redis-file:6379"
  password: "file-pass"
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)
	t.Setenv("HTTP_ADDRESS", ":9100")
	t.Setenv("KAFKA_BROKERS", "kafka:9092")
	t.Setenv("DB_DSN", "postgres://env")
	t.Setenv("REDIS_ADDR", "redis-env:6379")
	t.Setenv("REDIS_PASSWORD", "env-pass")

	cfg := Load()

	if cfg.Server.Address != ":9100" {
		t.Fatalf("Server.Address = %q, want %q", cfg.Server.Address, ":9100")
	}
	if cfg.DB.DSN != "postgres://env" {
		t.Fatalf("DB.DSN = %q, want env override", cfg.DB.DSN)
	}
	if cfg.Redis.Addr != "redis-env:6379" {
		t.Fatalf("Redis.Addr = %q, want env override", cfg.Redis.Addr)
	}
}

func TestLoadFromEnvOnly(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("HTTP_ADDRESS", ":9200")
	t.Setenv("KAFKA_BROKERS", "kafka:9092")
	t.Setenv("DB_DSN", "postgres://env-only")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("REDIS_PASSWORD", "secret")

	cfg := Load()

	if cfg.Server.Address != ":9200" {
		t.Fatalf("Server.Address = %q, want %q", cfg.Server.Address, ":9200")
	}
	if cfg.DB.DSN != "postgres://env-only" {
		t.Fatalf("DB.DSN = %q, want env-only value", cfg.DB.DSN)
	}
	if len(cfg.Kafka.Consumer.Brokers) != 1 || cfg.Kafka.Consumer.Brokers[0] != "kafka:9092" {
		t.Fatalf("KAFKA_BROKERS = %#v, want one broker", cfg.Kafka.Consumer.Brokers)
	}
}

func TestLoadExitsOnInvalidConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "broken.yaml")
	if err := os.WriteFile(configPath, []byte("server: ["), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestLoadHelperProcess")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=invalid-config",
		"CONFIG_PATH="+configPath,
	)

	err := cmd.Run()
	if err == nil {
		t.Fatal("helper process error = nil, want non-zero exit")
	}
}

func TestLoadExitsOnMissingEnv(t *testing.T) {
	dir := t.TempDir()

	cmd := exec.Command(os.Args[0], "-test.run=TestLoadHelperProcess")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=missing-env",
		"CONFIG_PATH="+filepath.Join(dir, "missing.yaml"),
	)

	err := cmd.Run()
	if err == nil {
		t.Fatal("helper process error = nil, want non-zero exit")
	}
}

func TestLoadHelperProcess(t *testing.T) {
	switch os.Getenv("GO_WANT_HELPER_PROCESS") {
	case "invalid-config", "missing-env":
		Load()
		t.Fatal("Load() returned, want process exit")
	default:
		return
	}
}
