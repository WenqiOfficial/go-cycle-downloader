package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"docker-cycler/pkg/config"
)

// DownloadProgress 保存当前下载的状态
type DownloadProgress struct {
	Percent int    `json:"percent"`
	Speed   int    `json:"speed"` // KB/s
	Size    int    `json:"size"`  // KB
	Status  string `json:"status"`
}

// Stats 保存下载统计信息
type Stats struct {
	LastDownload        string     `json:"last_download"`
	LastFile            string     `json:"last_file"`
	Message             string     `json:"message"`
	DailyDownloadedMB   int        `json:"daily_downloaded_mb"`
	MonthlyDownloadedMB int        `json:"monthly_downloaded_mb"`
	LastStatDay         int        `json:"-"` // 在JSON中忽略
	LastStatMonth       time.Month `json:"-"` // 在JSON中忽略
}

// AppStatus 代表发送到前端的应用程序的整体状态
type AppStatus struct {
	Config      config.Config `json:"config"`
	Stats       Stats         `json:"stats"`
	TaskEnabled bool          `json:"task_enabled"`
	TaskStatus  string        `json:"task_status"`
}

var (
	// 全局状态变量
	appStats        Stats
	currentProgress DownloadProgress
	taskStatus      = "空闲" // "空闲", "下载中", "失败", "已停止"
	downloadDir     string

	// 用于线程安全访问的互斥锁
	stateLock    sync.RWMutex
	progressLock sync.RWMutex

	// 用于通知下载停止的上下文和取消函数
	downloadCtx    context.Context
	downloadCancel context.CancelFunc

	statsFile = "conf/stats.json"
)

// InitState 从文件加载初始状态
func InitState() {
	if err := os.MkdirAll("conf", 0755); err != nil {
		log.Printf("警告: 创建配置目录 'conf' 失败: %v", err)
	}

	if err := config.LoadConfig(); err != nil {
		log.Printf("警告: 加载配置文件失败: %v。将使用默认配置。", err)
	} else {
		log.Println("配置文件加载成功。")
	}

	downloadDir = config.GetConfig().Dir
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		log.Printf("警告: 创建下载目录 '%s' 失败: %v", downloadDir, err)
	}

	if err := LoadStats(); err != nil {
		log.Printf("警告: 加载统计文件失败: %v。将使用初始统计。", err)
		ResetStats(true, true) // 如果加载失败，重置统计
	} else {
		log.Println("统计文件加载成功。")
	}

	// 初始化第一个下载上下文
	NewDownloadContext()
}

// NewDownloadContext 创建一个新的可取消的上下文用于下载
func NewDownloadContext() {
	stateLock.Lock()
	defer stateLock.Unlock()
	if downloadCancel != nil {
		downloadCancel() // 以防万一，取消之前的上下文
	}
	downloadCtx, downloadCancel = context.WithCancel(context.Background())
}

// GetDownloadContext 返回当前的下载上下文
func GetDownloadContext() context.Context {
	stateLock.RLock()
	defer stateLock.RUnlock()
	return downloadCtx
}

// --- 进度管理 ---

func SetProgress(percent, speed, size int, status string) {
	progressLock.Lock()
	defer progressLock.Unlock()
	currentProgress = DownloadProgress{
		Percent: percent,
		Speed:   speed,
		Size:    size,
		Status:  status,
	}
}

func GetProgress() DownloadProgress {
	progressLock.RLock()
	defer progressLock.RUnlock()
	return currentProgress
}

// --- 状态和统计管理 ---

func SetTaskStatus(status string) {
	stateLock.Lock()
	defer stateLock.Unlock()
	taskStatus = status
}

func GetTaskStatus() string {
	stateLock.RLock()
	defer stateLock.RUnlock()
	return taskStatus
}

func GetAppStatus() AppStatus {
	stateLock.RLock()
	defer stateLock.RUnlock()
	cfg := config.GetConfig()
	// 使用配置文件中的设置，而不是内部变量
	return AppStatus{
		Config:      cfg,
		Stats:       appStats,
		TaskEnabled: cfg.TaskEnabled,
		TaskStatus:  taskStatus,
	}
}

func UpdateMessage(format string, args ...interface{}) {
	stateLock.Lock()
	defer stateLock.Unlock()
	appStats.Message = fmt.Sprintf(format, args...)
	log.Println(appStats.Message)
}

func AddDownloadStats(bytesDownloaded int) {
	stateLock.Lock()
	defer stateLock.Unlock()
	mbDownloaded := bytesDownloaded / 1024 / 1024
	if mbDownloaded > 0 {
		appStats.DailyDownloadedMB += mbDownloaded
		appStats.MonthlyDownloadedMB += mbDownloaded
		saveStats()
	}
}

func UpdateLastDownloadInfo(filename string, success bool) {
	stateLock.Lock()
	defer stateLock.Unlock()
	if success {
		appStats.LastDownload = time.Now().Format("2006-01-02 15:04:05")
		appStats.LastFile = filename
	}
	saveStats()
}

// checkAndResetStats 检查是否需要重置每日或每月统计
func CheckAndResetStats() {
	stateLock.Lock()
	defer stateLock.Unlock()
	now := time.Now()
	resetDaily := now.Day() != appStats.LastStatDay
	resetMonthly := now.Month() != appStats.LastStatMonth
	ResetStats(resetDaily, resetMonthly)
}

func ResetStats(daily, monthly bool) {
	now := time.Now()
	if daily {
		appStats.DailyDownloadedMB = 0
		appStats.LastStatDay = now.Day()
	}
	if monthly {
		appStats.MonthlyDownloadedMB = 0
		appStats.LastStatMonth = now.Month()
	}
	saveStats()
}

// --- 持久化 ---

func LoadStats() error {
	stateLock.Lock()
	defer stateLock.Unlock()
	file, err := os.Open(statsFile)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&appStats)
	if err == nil {
		// 成功加载后，设置当前日期
		now := time.Now()
		appStats.LastStatDay = now.Day()
		appStats.LastStatMonth = now.Month()
	}
	return err
}

func saveStats() error {
	file, err := os.Create(statsFile)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(appStats)
}
