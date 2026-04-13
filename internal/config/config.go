package config

import (
"fmt"
"os"
"path/filepath"

"gopkg.in/yaml.v3"
)

// Config mirrors the structure of ~/.zread/config.yaml as used by the original zread CLI.
//
// YAML layout:
//
//language: zh
//doc_language: zh
//llm:
//    provider: custom
//    model: deepseek-chat
//    api_key: sk-xxx
//    base_url: https://api.deepseek.com/v1
//concurrency:
//    max_concurrent: 1
//    max_retries: 1
type Config struct {
Language    string      `yaml:"language"`
DocLanguage string      `yaml:"doc_language"`
LLM         LLMConfig   `yaml:"llm"`
Concurrency ConcurrConfig `yaml:"concurrency"`
}

// LLMConfig holds the model provider settings.
type LLMConfig struct {
Provider string `yaml:"provider"`
Model    string `yaml:"model"`
APIKey   string `yaml:"api_key"`
BaseURL  string `yaml:"base_url"`
}

// ConcurrConfig holds concurrency settings.
type ConcurrConfig struct {
MaxConcurrent int `yaml:"max_concurrent"`
MaxRetries    int `yaml:"max_retries"`
}

// Flat returns a flat view of Config used by the rest of the codebase.
type Flat struct {
APIKey   string
BaseURL  string
Model    string
Language string
Workers  int
Retries  int
}

// ConfigPath returns the path of the config file (~/.zread/config.yaml).
func ConfigPath() string {
home, _ := os.UserHomeDir()
return filepath.Join(home, ".zread", "config.yaml")
}

// Load reads ~/.zread/config.yaml, applies env-var overrides, and returns a Flat view.
func Load() (*Flat, error) {
cfg := defaults()

data, err := os.ReadFile(ConfigPath())
if err == nil {
if err2 := yaml.Unmarshal(data, cfg); err2 != nil {
return nil, fmt.Errorf("解析配置文件失败: %w", err2)
}
}

// Ensure nested defaults if sections are empty
if cfg.LLM.BaseURL == "" {
cfg.LLM.BaseURL = "https://api.deepseek.com/v1"
}
if cfg.LLM.Model == "" {
cfg.LLM.Model = "deepseek-chat"
}
if cfg.Language == "" {
cfg.Language = "zh"
}
if cfg.DocLanguage == "" {
cfg.DocLanguage = cfg.Language
}
if cfg.Concurrency.MaxConcurrent <= 0 {
cfg.Concurrency.MaxConcurrent = 1
}
if cfg.Concurrency.MaxRetries <= 0 {
cfg.Concurrency.MaxRetries = 1
}

// Environment variable overrides (backwards compat)
if v := os.Getenv("ZREAD_API_KEY"); v != "" {
cfg.LLM.APIKey = v
}
if v := os.Getenv("ZREAD_BASE_URL"); v != "" {
cfg.LLM.BaseURL = v
}
if v := os.Getenv("ZREAD_MODEL"); v != "" {
cfg.LLM.Model = v
}

return cfg.Flat(), nil
}

// LoadRaw returns the full structured Config (used by the config subcommand).
func LoadRaw() *Config {
cfg := defaults()
data, err := os.ReadFile(ConfigPath())
if err == nil {
_ = yaml.Unmarshal(data, cfg)
}
return cfg
}

// Save writes the Config to ~/.zread/config.yaml.
func Save(cfg *Config) error {
path := ConfigPath()
if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
return err
}
data, err := yaml.Marshal(cfg)
if err != nil {
return err
}
return os.WriteFile(path, data, 0o600)
}

// SaveFlat converts a Flat config back to Config and saves it.
func SaveFlat(f *Flat) error {
raw := LoadRaw()
raw.LLM.APIKey = f.APIKey
raw.LLM.BaseURL = f.BaseURL
raw.LLM.Model = f.Model
raw.Language = f.Language
raw.DocLanguage = f.Language
raw.Concurrency.MaxConcurrent = f.Workers
raw.Concurrency.MaxRetries = f.Retries
return Save(raw)
}

func defaults() *Config {
return &Config{
Language:    "zh",
DocLanguage: "zh",
LLM: LLMConfig{
Provider: "custom",
Model:    "deepseek-chat",
BaseURL:  "https://api.deepseek.com/v1",
},
Concurrency: ConcurrConfig{
MaxConcurrent: 1,
MaxRetries:    1,
},
}
}

// Flat builds a Flat from the structured Config.
func (c *Config) Flat() *Flat {
return &Flat{
APIKey:   c.LLM.APIKey,
BaseURL:  c.LLM.BaseURL,
Model:    c.LLM.Model,
Language: docLang(c),
Workers:  c.Concurrency.MaxConcurrent,
Retries:  c.Concurrency.MaxRetries,
}
}

// docLang returns the effective documentation language string.
// Original zread uses "zh"/"en" locale codes; we map to display names for prompts.
func docLang(c *Config) string {
lang := c.DocLanguage
if lang == "" {
lang = c.Language
}
switch lang {
case "zh", "zh-CN", "zh-TW":
return "Chinese"
case "en", "en-US", "en-GB":
return "English"
default:
if lang == "" {
return "Chinese"
}
return lang
}
}
