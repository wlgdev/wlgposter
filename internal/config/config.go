package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Current holds the loaded configuration for global access after MustLoad.
var Current *Config

type Config struct {
	ENV                        string  `env:"ENV,required"`
	APIID                      int32   `env:"API_ID,required"`
	APIHash                    string  `env:"API_HASH,required"`
	TelegramBotToken           string  `env:"TELEGRAM_BOT_TOKEN,required"`
	TelegramTargetChannelID    []int64 `env:"-"`
	TelegramTargetChannelIDRaw string  `env:"TELEGRAM_TARGET_CHANNEL_ID,required"`
	TelegramAdmins             []int64 `env:"-"`
	TelegramAdminsRaw          string  `env:"TELEGRAM_ADMINS,required"`
	MaxBotToken                string  `env:"MAX_BOT_TOKEN"`
	MaxTargetChatID            int64   `env:"MAX_TARGET_CHAT_ID,required"`
	VkToken                    string  `env:"VK_TOKEN"`
	VkGroupID                  int     `env:"VK_GROUP_ID,required"`
	TmpDir                     string  `env:"TMP_DIR,required"`
	DBPath                     string  `env:"DB,required"`
	FileSizeLimit              int64   `env:"FILE_SIZE_LIMIT,required"`
	LogLevel                   string  `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat                  string  `env:"LOG_FORMAT" envDefault:"console"`
	OtlpEndpoint               string  `env:"OTLP_ENDPOINT,required"`
}

func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	Current = cfg
	return cfg
}

func Load() (*Config, error) {
	// priority order: dev, prod
	envFiles := []string{".env.dev", ".env.prod"}

	for _, file := range envFiles {
		if _, err := os.Stat(file); err == nil {
			if err := godotenv.Load(file); err != nil {
				return nil, err
			}
			break
		}
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	admins, err := parseListEntry(cfg.TelegramAdminsRaw)
	if err != nil {
		return nil, err
	}
	cfg.TelegramAdmins = admins

	channelIDs, err := parseListEntry(cfg.TelegramTargetChannelIDRaw)
	if err != nil {
		return nil, err
	}
	cfg.TelegramTargetChannelID = channelIDs

	if _, err := os.Stat(cfg.DBPath); os.IsNotExist(err) {
		if err := os.Mkdir(cfg.DBPath, 0777); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(cfg.TmpDir); os.IsNotExist(err) {
		if err := os.Mkdir(cfg.TmpDir, 0777); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

func parseListEntry(raw string) ([]int64, error) {
	parts := strings.Split(raw, ",")
	items := make([]int64, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		id, err := strconv.ParseInt(item, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid entry %q", item)
		}
		items = append(items, id)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("list is empty")
	}
	return items, nil
}
