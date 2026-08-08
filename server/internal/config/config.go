package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Security SecurityConfig `mapstructure:"security"`
	Backup   BackupConfig   `mapstructure:"backup"`
	Log      LogConfig      `mapstructure:"log"`
}

type ServerConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	Mode        string `mapstructure:"mode"`
	ExternalURL string `mapstructure:"external_url"`
	// TrustedProxies 限定可提供 X-Forwarded-For 等头部的反向代理地址。
	// 默认仅信任本机代理；空列表表示不信任任何代理头。
	TrustedProxies []string `mapstructure:"trusted_proxies"`
	// WebRoot 指向前端构建产物目录。留空时后端会按部署惯例自动探测
	// （./web、./web/dist、/opt/backupx/web 等）。探测命中后后端直接托管
	// 前端 SPA，无需额外的 nginx 反向代理即可访问 Web 控制台。
	WebRoot string `mapstructure:"web_root"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type SecurityConfig struct {
	JWTSecret     string `mapstructure:"jwt_secret"`
	JWTExpire     string `mapstructure:"jwt_expire"`
	EncryptionKey string `mapstructure:"encryption_key"`
}

type BackupConfig struct {
	TempDir        string `mapstructure:"temp_dir"`
	MaxConcurrent  int    `mapstructure:"max_concurrent"`
	Retries        int    `mapstructure:"retries"`         // 底层 HTTP 请求重试次数，默认 10
	BandwidthLimit string `mapstructure:"bandwidth_limit"` // 带宽限制，如 "10M"，空不限
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

func Load(configPath string) (Config, error) {
	v := viper.New()
	applyDefaults(v)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("BACKUPX")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("./server")
		v.AddConfigPath("/etc/backupx")
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return Config{}, fmt.Errorf("read config: %w", err)
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8340
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "release"
	}
	cfg.Server.ExternalURL = strings.TrimRight(strings.TrimSpace(cfg.Server.ExternalURL), "/")
	if cfg.Server.ExternalURL != "" {
		externalURL, parseErr := url.Parse(cfg.Server.ExternalURL)
		if parseErr != nil || (externalURL.Scheme != "http" && externalURL.Scheme != "https") || externalURL.Host == "" || externalURL.User != nil || externalURL.RawQuery != "" || externalURL.Fragment != "" {
			return Config{}, fmt.Errorf("server.external_url must be an absolute http(s) URL without credentials, query or fragment")
		}
	}
	if len(cfg.Server.TrustedProxies) == 1 && strings.Contains(cfg.Server.TrustedProxies[0], ",") {
		cfg.Server.TrustedProxies = strings.Split(cfg.Server.TrustedProxies[0], ",")
	}
	for index := range cfg.Server.TrustedProxies {
		proxy := strings.TrimSpace(cfg.Server.TrustedProxies[index])
		cfg.Server.TrustedProxies[index] = proxy
		if net.ParseIP(proxy) == nil {
			if _, _, parseErr := net.ParseCIDR(proxy); parseErr != nil {
				return Config{}, fmt.Errorf("server.trusted_proxies contains invalid IP or CIDR %q", proxy)
			}
		}
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./data/backupx.db"
	}
	if cfg.Security.JWTExpire == "" {
		cfg.Security.JWTExpire = "24h"
	}
	if cfg.Backup.TempDir == "" {
		cfg.Backup.TempDir = "/tmp/backupx"
	}
	if cfg.Backup.MaxConcurrent <= 0 {
		cfg.Backup.MaxConcurrent = 2
	}
	if cfg.Backup.Retries <= 0 {
		cfg.Backup.Retries = 10
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.File == "" {
		cfg.Log.File = "./data/backupx.log"
	}
	if cfg.Log.MaxSize <= 0 {
		cfg.Log.MaxSize = 100
	}
	if cfg.Log.MaxBackups <= 0 {
		cfg.Log.MaxBackups = 3
	}
	if cfg.Log.MaxAge <= 0 {
		cfg.Log.MaxAge = 30
	}

	return cfg, nil
}

func MustJWTDuration(cfg SecurityConfig) time.Duration {
	duration, err := time.ParseDuration(cfg.JWTExpire)
	if err != nil {
		return 24 * time.Hour
	}
	return duration
}

func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func applyDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8340)
	v.SetDefault("server.mode", "release")
	v.SetDefault("server.external_url", "")
	v.SetDefault("server.trusted_proxies", []string{"127.0.0.1", "::1"})
	v.SetDefault("server.web_root", "")
	v.SetDefault("database.path", "./data/backupx.db")
	v.SetDefault("security.jwt_expire", "24h")
	v.SetDefault("backup.temp_dir", "/tmp/backupx")
	v.SetDefault("backup.max_concurrent", 2)
	v.SetDefault("backup.retries", 10)
	v.SetDefault("backup.bandwidth_limit", "")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.file", "./data/backupx.log")
	v.SetDefault("log.max_size", 100)
	v.SetDefault("log.max_backups", 3)
	v.SetDefault("log.max_age", 30)
}
