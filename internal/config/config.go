package config

import (
	"os"
	"path/filepath"

	"quill/internal/logc"

	"github.com/joho/godotenv"
)

type Config struct {
	BaseURL   string
	WSURL     string
	Token     string
	WSMode    string
	WSListen  string
	WSPath    string
	FileUnsafe bool
}

func Load() *Config {
	loadDotEnv()

	cfg := &Config{
		BaseURL:    getEnv("ONEBOT_BASE_URL", "http://localhost:3007"),
		Token:      getEnv("ONEBOT_TOKEN", ""),
		WSMode:     getEnv("ONEBOT_WS_MODE", "forward"),
		WSListen:   getEnv("ONEBOT_WS_LISTEN", ":3001"),
		WSPath:     getEnv("ONEBOT_WS_PATH", "/"),
		FileUnsafe: getEnvBool("ONEBOT_FILE_UNSAFE"),
	}
	cfg.WSURL = getEnv("ONEBOT_WS_URL", cfg.BaseURL)

	if cfg.FileUnsafe {
		logc.Fmt("\u26A0", logc.Red, "\u9AD8\u5371! ONEBOT_FILE_UNSAFE=true \u2014 bot.file \u53EF\u8BFB\u5199\u4EFB\u610F\u8DEF\u5F84")
	}

	return cfg
}

func loadDotEnv() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	paths := []string{
		filepath.Join(dir, ".env"),
		".env",
	}
	for _, p := range paths {
		if err := godotenv.Load(p); err == nil {
			logc.Fmt("\u00b7", logc.Gray, "加载配置 %s", p)
			return
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1" || v == "yes"
}
