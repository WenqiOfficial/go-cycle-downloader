package server

import (
	"log"
	"time"

	"docker-cycler/pkg/config"
	"docker-cycler/pkg/docker"
	"docker-cycler/pkg/downloader"
)

// StartScheduler 启动一个goroutine来处理定时下载任务
func StartScheduler() {
	log.Println("任务调度器已启动")
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次是否需要执行任务

	go func() {
		for range ticker.C {
			status := docker.GetAppStatus()
			if !status.TaskEnabled {
				continue
			}

			// 检查并重置每日/每月统计数据
			docker.CheckAndResetStats()

			cfg := config.GetConfig()
			if cfg.URL == "" {
				continue // 没有URL，跳过
			}

			if shouldDownload(cfg) {
				// 避免在已有下载时启动新下载
				if docker.GetTaskStatus() == "下载中" {
					log.Println("调度器：检测到已有下载任务，本次跳过")
					continue
				}

				// 检查下载限制 - 使用配置文件中的设置
				if cfg.DailyLimitEnabled && status.Stats.DailyDownloadedMB >= cfg.LimitMB {
					docker.UpdateMessage("调度器：今日下载量已达上限，任务跳过")
					continue
				}

				// 启动下载
				go func() {
					docker.SetTaskStatus("下载中")
					docker.NewDownloadContext() // 为定时任务创建一个新的上下文

					file, size, err := downloader.DownloadFileWithProgress(docker.GetDownloadContext(), cfg.URL, cfg.SpeedKB, cfg.Dir)
					if err != nil {
						docker.SetTaskStatus("失败")
						docker.UpdateMessage("定时下载失败: %v", err)
						docker.UpdateLastDownloadInfo(file, false)
					} else {
						docker.SetTaskStatus("空闲")
						docker.UpdateMessage("定时下载成功: %s", file)
						docker.UpdateLastDownloadInfo(file, true)
						docker.AddDownloadStats(size)
					}
				}()
			}
		}
	}()
}

// lastTriggered 用于跟踪周期性任务的最后触发时间
var lastTriggered time.Time

// shouldDownload 判断当前时间是否满足下载条件
func shouldDownload(cfg config.Config) bool {
	now := time.Now()
	switch cfg.PlanType {
	case "daily":
		// 检查是否在设定的分钟内
		if now.Hour() == cfg.Hour && now.Minute() == cfg.Minute {
			// 为了避免在同一分钟内重复触发，我们检查自上次触发以来是否已超过一分钟
			if time.Since(lastTriggered) > time.Minute {
				lastTriggered = now
				return true
			}
		}
	case "interval":
		if cfg.IntervalMinutes <= 0 {
			return false
		}
		// 检查自上次触发以来是否已超过设定的间隔
		if time.Since(lastTriggered) > time.Duration(cfg.IntervalMinutes)*time.Minute {
			lastTriggered = now
			return true
		}
	}
	return false
}
