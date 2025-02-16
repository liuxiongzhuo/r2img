package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	ApiSite       string  `json:"api_site"`
	ApiKey        string  `json:"api_key"`
	AuthKey       string  `json:"auth_key"`
	Quality       float32 `json:"quality"`
	MaxFileSize   int64   `json:"max_file_size"`
	Port          int     `json:"port"`
	MaxCacheSize  int64   `json:"max_cache_size"`
	FreeCacheSize int64   `json:"free_cache_size"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败: %v", err)
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("解码配置文件失败: %v", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("配置文件验证失败: %v", err)
	}

	return &cfg, nil
}

func validateConfig(cfg *Config) error {
	if cfg.ApiSite == "" || cfg.ApiKey == "" || cfg.AuthKey == "" || cfg.Quality == 0 || cfg.MaxFileSize == 0 || cfg.Port == 0 || cfg.MaxCacheSize == 0 || cfg.FreeCacheSize == 0 {
		return fmt.Errorf("配置文件缺少必要字段")
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("无效的端口号: %d", cfg.Port)
	}

	if cfg.MaxFileSize <= 0 || cfg.MaxFileSize > 1024*1024 {
		return fmt.Errorf("最大文件大小过大或过小: %d", cfg.MaxFileSize)
	}

	return nil
}
