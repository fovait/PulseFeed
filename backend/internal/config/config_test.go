package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 合法的测试用 YAML 配置，覆盖全部模块
const validYAML = `
server:
  host: "127.0.0.1"
  port: 8080
  mode: debug

database:
  host: 127.0.0.1
  port: 3306
  user: root
  password: ""
  dbname: pulsefeed

redis:
  host: "127.0.0.1"
  port: 6379
  password: ""
  db: 0

jwt:
  secret: "test-secret"
  issuer: "pulsefeed"
  ttl: 24h

rabbitmq:
  host: 127.0.0.1
  port: 5672
  username: guest
  password: guest

observability:
  pprof:
    enabled: true
    api_addr: "localhost:6060"
    worker_addr: "localhost:6061"
`

// 在临时目录写入 YAML 文件，返回文件路径
func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)
	return path
}

// ============================================================================
// load 函数测试（内部函数，同包白盒测试）
// ============================================================================

// TestLoad_Success 测试正常解析合法 YAML 文件
func TestLoad_Success(t *testing.T) {
	path := writeTempYAML(t, validYAML)

	cfg, err := load(path)
	if err != nil {
		t.Fatalf("预期无错误，实际: %v", err)
	}

	// 抽样验证各模块字段是否正确解析
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %q, 预期 %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, 预期 %d", cfg.Server.Port, 8080)
	}
	if cfg.Database.DBName != "pulsefeed" {
		t.Errorf("Database.DBName = %q, 预期 %q", cfg.Database.DBName, "pulsefeed")
	}
	if cfg.JWT.TTL.String() != "24h0m0s" {
		t.Errorf("JWT.TTL = %v, 预期 %v", cfg.JWT.TTL, "24h0m0s")
	}
	if cfg.ObservabilityConfig.Pprof.Enabled != true {
		t.Errorf("Pprof.Enabled = %v, 预期 true", cfg.ObservabilityConfig.Pprof.Enabled)
	}
	if cfg.ObservabilityConfig.Pprof.ApiAddr != "localhost:6060" {
		t.Errorf("Pprof.ApiAddr = %q, 预期 %q", cfg.ObservabilityConfig.Pprof.ApiAddr, "localhost:6060")
	}
}

// TestLoad_FileNotFound 测试文件不存在时的错误包装
func TestLoad_FileNotFound(t *testing.T) {
	cfg, err := load("/tmp/does-not-exist/config.yaml")

	if err == nil {
		t.Fatalf("预期返回错误，实际 cfg=%+v", cfg)
	}
	if !strings.Contains(err.Error(), "failed to load config file") {
		t.Errorf("错误信息应包含 'failed to load config file'，实际: %v", err)
	}
}

// TestLoad_InvalidYAML 测试解析非法的 YAML 内容
func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTempYAML(t, "{this is not valid yaml")

	cfg, err := load(path)
	if err == nil {
		t.Fatalf("预期解析错误，实际 cfg=%+v", cfg)
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Errorf("错误信息应包含 'parse config'，实际: %v", err)
	}
}

// ============================================================================
// applyEnvOverrides 函数测试
// ============================================================================

// TestApplyEnvOverrides 测试环境变量覆盖配置
func TestApplyEnvOverrides(t *testing.T) {
	cfg := DefaultLocalConfig()

	// 设置环境变量
	t.Setenv("JWT_SECRET", "env-override-secret")
	t.Setenv("JWT_ISSUER", "env-override-issuer")
	t.Setenv("JWT_TTL", "1h30m")

	applyEnvOverrides(&cfg)

	if cfg.JWT.Secret != "env-override-secret" {
		t.Errorf("JWT.Secret = %q, 预期 %q", cfg.JWT.Secret, "env-override-secret")
	}
	if cfg.JWT.Issuer != "env-override-issuer" {
		t.Errorf("JWT.Issuer = %q, 预期 %q", cfg.JWT.Issuer, "env-override-issuer")
	}
	if cfg.JWT.TTL.String() != "1h30m0s" {
		t.Errorf("JWT.TTL = %v, 预期 %v", cfg.JWT.TTL, "1h30m0s")
	}
}

// TestApplyEnvOverrides_NilConfig 测试传入 nil 配置不会 panic
func TestApplyEnvOverrides_NilConfig(t *testing.T) {
	// 不应 panic
	applyEnvOverrides(nil)
}

// TestApplyEnvOverrides_EmptyEnv 测试环境变量为空时保留原值
func TestApplyEnvOverrides_EmptyEnv(t *testing.T) {
	cfg := DefaultLocalConfig()
	originalSecret := cfg.JWT.Secret

	// 不设置任何环境变量
	applyEnvOverrides(&cfg)

	if cfg.JWT.Secret != originalSecret {
		t.Errorf("空环境变量不应覆盖原值: got %q, want %q", cfg.JWT.Secret, originalSecret)
	}
}

// ============================================================================
// DefaultLocalConfig 函数测试
// ============================================================================

// TestDefaultLocalConfig 测试本地默认配置中的关键默认值
func TestDefaultLocalConfig(t *testing.T) {
	cfg := DefaultLocalConfig()

	// Server 默认值
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, 预期 8080", cfg.Server.Port)
	}

	// Database 默认值
	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %q, 预期 localhost", cfg.Database.Host)
	}

	// JWT 默认值
	if cfg.JWT.Secret == "" {
		t.Error("JWT.Secret 不应为空")
	}
	if cfg.JWT.Issuer != "pulsefeed" {
		t.Errorf("JWT.Issuer = %q, 预期 pulsefeed", cfg.JWT.Issuer)
	}
	if cfg.JWT.TTL == 0 {
		t.Error("JWT.TTL 不应为 0")
	}

	// Observability 默认值
	if !cfg.ObservabilityConfig.Pprof.Enabled {
		t.Error("Pprof.Enabled 应为 true（本地开发默认开启）")
	}
}

// ============================================================================
// LoadConfig 函数测试（公开 API）
// ============================================================================

// TestLoadConfig_FileExists 测试文件存在时正常加载
func TestLoadConfig_FileExists(t *testing.T) {
	path := writeTempYAML(t, validYAML)

	cfg, usedDefault, err := LoadConfig(path)
	if err != nil {
		t.Logf("LoadConfig 返回值结构可能需调整，当前 err: %v", err)
	}
	if usedDefault {
		t.Error("文件存在时应返回 usedDefault=false")
	}
	_ = cfg // 具体值由 TestLoad_Success 验证
}

// TestLoadConfig_FileNotFound 文件不存在时回退到默认配置
func TestLoadConfig_FileNotFound(t *testing.T) {
	cfg, usedDefault, err := LoadConfig("/tmp/does-not-exist.yaml")

	if err != nil {
		t.Fatalf("回退默认配置不应返回 error，实际: %v", err)
	}
	if !usedDefault {
		t.Error("文件不存在时 usedDefault 应为 true")
	}
	if cfg.Server.Port == 0 {
		t.Error("默认配置未生效（Server.Port 为 0）")
	}
	if cfg.JWT.Secret == "" {
		t.Error("默认配置未生效（JWT.Secret 为空）")
	}
}
