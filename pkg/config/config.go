package config

import (
	"encoding/json"
	"os"
	"sync"
)

// Config 结构体定义了所有可配置的参数
type Config struct {
	URL                 string `json:"url"`
	PlanType            string `json:"plan_type"` // "daily" or "interval"
	IntervalMinutes     int    `json:"interval_minutes"`
	Hour                int    `json:"hour"`
	Minute              int    `json:"minute"`
	SpeedKB             int    `json:"speed_kb"`
	Dir                 string `json:"dir"`
	LimitMB             int    `json:"limit_mb"`
	TaskEnabled         bool   `json:"task_enabled"`         // 自动任务是否启用
	DailyLimitEnabled   bool   `json:"daily_limit_enabled"`  // 每日下载量限制是否启用
}

var (
	config     Config
	configLock sync.RWMutex
	configFile = "conf/config.json"
)

// LoadConfig 从 config.json 文件加载配置
// 如果文件不存在，会使用默认值创建一个
func LoadConfig() error {
	configLock.Lock()
	defer configLock.Unlock()

	file, err := os.Open(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在，使用默认值并创建
			config = DefaultConfig()
			return SaveConfigLocked()
		}
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		// 解析失败，使用默认值
		config = DefaultConfig()
	}
	return nil
}

// SaveConfig 将当前配置保存到 config.json 文件
func SaveConfig() error {
	configLock.Lock()
	defer configLock.Unlock()
	return SaveConfigLocked()
}

// SaveConfigLocked 是一个无锁的保存配置的内部函数
func SaveConfigLocked() error {
	file, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // 美化输出
	return encoder.Encode(config)
}

// GetConfig 返回当前配置的一个安全副本
func GetConfig() Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return config
}

// UpdateConfig 使用一个函数来安全地更新配置
func UpdateConfig(updateFunc func(c *Config)) {
	configLock.Lock()
	defer configLock.Unlock()
	updateFunc(&config)
}

// GetTaskEnabled 获取自动任务启用状态
func GetTaskEnabled() bool {
	configLock.RLock()
	defer configLock.RUnlock()
	return config.TaskEnabled
}

// SetTaskEnabled 设置自动任务启用状态
func SetTaskEnabled(enabled bool) error {
	configLock.Lock()
	defer configLock.Unlock()
	config.TaskEnabled = enabled
	return SaveConfigLocked()
}

// GetDailyLimitEnabled 获取每日下载量限制启用状态
func GetDailyLimitEnabled() bool {
	configLock.RLock()
	defer configLock.RUnlock()
	return config.DailyLimitEnabled
}

// SetDailyLimitEnabled 设置每日下载量限制启用状态
func SetDailyLimitEnabled(enabled bool) error {
	configLock.Lock()
	defer configLock.Unlock()
	config.DailyLimitEnabled = enabled
	return SaveConfigLocked()
}

// ToggleTaskEnabled 切换自动任务启用状态
func ToggleTaskEnabled() (bool, error) {
	configLock.Lock()
	defer configLock.Unlock()
	config.TaskEnabled = !config.TaskEnabled
	err := SaveConfigLocked()
	return config.TaskEnabled, err
}

// ToggleDailyLimitEnabled 切换每日下载量限制启用状态
func ToggleDailyLimitEnabled() (bool, error) {
	configLock.Lock()
	defer configLock.Unlock()
	config.DailyLimitEnabled = !config.DailyLimitEnabled
	err := SaveConfigLocked()
	return config.DailyLimitEnabled, err
}

// DefaultConfig 返回一个默认的配置实例
func DefaultConfig() Config {
	return Config{
		URL:                 "",
		PlanType:            "interval",
		IntervalMinutes:     30,
		Hour:                3,
		Minute:              0,
		SpeedKB:             0,    // 0 表示不限速
		Dir:                 "tmp",
		LimitMB:             1024,
		TaskEnabled:         true,  
		DailyLimitEnabled:   false, 
	}
}
