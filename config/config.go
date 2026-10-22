package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// 把整个小配置打包成一个整体，方便统一管理
/*
不打包 → 恶心写法
func ConnectDB(server ServerConfig, mysql MysqlConfig, redis RedisConfig)
打包 → 清爽写法
func ConnectDB(cfg *config.App)
*/
type App struct {
	Server      ServerConfigItem
	Mysql       MysqlConfig
	Redis       RedisConfig
	RabbitMQ    RabbitMQConfig    `mapstructure:"rabbitmq"`
	Seckill     SeckillConfig     `mapstructure:"seckill"`
	Trace       TraceConfig       `mapstructure:"trace"`
	Path        PathConfig
	Jwt         JwtConfig         `mapstructure:"jwt"`
	Meilisearch MeilisearchConfig `mapstructure:"meilisearch"`
	Security    SecurityConfig    `mapstructure:"security"`
}

// 解耦 + 方便测试
// 测试代码直接造一个假配置实现接口
type ConfigProvider interface {
	GetServerConfig() ServerConfigItem
	GetMysqlConfig() MysqlConfig
	GetRedisConfig() RedisConfig
	GetRabbitMQConfig() RabbitMQConfig
	GetSeckillConfig() SeckillConfig
	GetTraceConfig() TraceConfig
	GetPathConfig() PathConfig
	GetJwtConfig() JwtConfig
	GetMeilisearchConfig() MeilisearchConfig
}
type ServerConfigItem struct {
	Model string `mapstructure:"model"`
	Port  string `mapstructure:"port"`
}
type MysqlConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Dbname   string `mapstructure:"dbname"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}
type RabbitMQConfig struct {
	URL string `mapstructure:"url"`
}
type MeilisearchConfig struct {
	Host         string `mapstructure:"host"`
	APIKey       string `mapstructure:"api_key"`
	ProductIndex string `mapstructure:"product_index"`
}
type SeckillConfig struct {
	RateLimitRate     int `mapstructure:"rate_limit_rate"`
	RateLimitCapacity int `mapstructure:"rate_limit_capacity"`
}
type TraceConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	ServiceName    string `mapstructure:"service_name"`
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"`
}
type PathConfig struct {
	Product string `mapstructure:"product"`
	Avatar  string `mapstructure:"avatar"`
}
type JwtConfig struct {
	Secret string `mapstructure:"secret"`
}

// SecurityConfig 安全相关配置。
// 生产通过环境变量覆盖：CORS_ORIGINS / DEMO_MODE / COOKIE_SECURE / TRUSTED_PROXIES。
type SecurityConfig struct {
	// CORSOrigins 允许跨域的来源白名单；为空表示同源部署，不返回任何 CORS 头。
	CORSOrigins []string `mapstructure:"cors_origins"`
	// DemoMode 开启演示模式：公开演示账号提示、模拟支付接口可用。
	DemoMode bool `mapstructure:"demo_mode"`
	// CookieSecure 强制 refresh token cookie 加 Secure（HTTPS 部署建议开启）。
	CookieSecure bool `mapstructure:"cookie_secure"`
	// TrustedProxies 允许解析 X-Forwarded-For 的代理地址，默认仅本机。
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

func (c *App) GetServerConfig() ServerConfigItem {
	return c.Server
}
func (c *App) GetMysqlConfig() MysqlConfig {
	return c.Mysql
}
func (c *App) GetRedisConfig() RedisConfig {
	return c.Redis
}
func (c *App) GetRabbitMQConfig() RabbitMQConfig {
	return c.RabbitMQ
}
func (c *App) GetSeckillConfig() SeckillConfig {
	return c.Seckill
}
func (c *App) GetTraceConfig() TraceConfig {
	return c.Trace
}
func (c *App) GetPathConfig() PathConfig {
	return c.Path
}
func (c *App) GetJwtConfig() JwtConfig {
	return c.Jwt
}
func (a *App) GetMeilisearchConfig() MeilisearchConfig {
	return a.Meilisearch
}

// GetSecurityConfig 返回安全配置（不放进 ConfigProvider，避免测试里的假配置被动改接口）。
func (a *App) GetSecurityConfig() SecurityConfig {
	return a.Security
}

func Init(filepath string) (*App, error) {
	v := viper.New()
	v.SetConfigFile(filepath)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg App
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	applyEnvOverrides(&cfg)
	return &cfg, nil
}

// applyEnvOverrides 用环境变量覆盖敏感配置。
//
// 注意：viper 的 AutomaticEnv() 对 Unmarshal 不生效（它查的是 "jwt.secret" 这种键），
// 所以这里显式读取环境变量，保证容器 / systemd 注入的密钥真的生效。
func applyEnvOverrides(cfg *App) {
	if v := os.Getenv("SERVER_PORT"); v != "" {
		cfg.Server.Port = v
	}
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		cfg.Mysql.Host = v
	}
	if v := os.Getenv("MYSQL_PORT"); v != "" {
		cfg.Mysql.Port = v
	}
	if v := os.Getenv("MYSQL_DATABASE"); v != "" {
		cfg.Mysql.Dbname = v
	}
	if v := os.Getenv("MYSQL_USERNAME"); v != "" {
		cfg.Mysql.Username = v
	}
	if v := os.Getenv("MYSQL_PASSWORD"); v != "" {
		cfg.Mysql.Password = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		cfg.Redis.Port = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("RABBITMQ_URL"); v != "" {
		cfg.RabbitMQ.URL = v
	}
	if v := os.Getenv("MEILI_HOST"); v != "" {
		cfg.Meilisearch.Host = v
	}
	if v := os.Getenv("MEILI_MASTER_KEY"); v != "" {
		cfg.Meilisearch.APIKey = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Jwt.Secret = v
	}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		cfg.Security.CORSOrigins = splitCSV(v)
	}
	if v := os.Getenv("DEMO_MODE"); v != "" {
		cfg.Security.DemoMode = parseBool(v, cfg.Security.DemoMode)
	}
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		cfg.Security.CookieSecure = parseBool(v, cfg.Security.CookieSecure)
	}
	if v := os.Getenv("TRUSTED_PROXIES"); v != "" {
		cfg.Security.TrustedProxies = splitCSV(v)
	}
	if len(cfg.Security.TrustedProxies) == 0 {
		// 默认只信任本机反向代理（nginx -> Go），避免客户端伪造 X-Forwarded-For 绕过限流
		cfg.Security.TrustedProxies = []string{"127.0.0.1", "::1"}
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseBool(value string, fallback bool) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}
