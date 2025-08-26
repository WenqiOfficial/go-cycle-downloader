package server

import (
	"embed"
	"encoding/json"
	"net/http"
	"strconv"
	"os"
	"log"
	"io/fs"

	"docker-cycler/pkg/config"
	"docker-cycler/pkg/docker"
	dockerPkg "docker-cycler/pkg/docker"
	"docker-cycler/pkg/downloader"
)

var embeddedFS embed.FS

// SetEmbeddedFS 设置嵌入的文件系统
func SetEmbeddedFS(fs embed.FS) {
	embeddedFS = fs
}

// RegisterHandlers 设置所有HTTP路由
func RegisterHandlers() {
	// 用于提供静态文件（CSS, JS）
	staticFS, _ := fs.Sub(embeddedFS, "templates/static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// 主页面
	http.HandleFunc("/", indexHandler)

	// API端点
	http.HandleFunc("/api/status", statusHandler)
	http.HandleFunc("/api/progress", progressHandler)
	http.HandleFunc("/api/set", setHandler)
	http.HandleFunc("/api/download", downloadHandler)
	http.HandleFunc("/api/stop", stopHandler)
	http.HandleFunc("/api/toggle_task", toggleTaskHandler)
	http.HandleFunc("/api/toggle_limit", toggleLimitHandler)
	http.HandleFunc("/api/clean", cleanHandler)
}

// --- 页面处理器 ---

func indexHandler(w http.ResponseWriter, r *http.Request) {
	data, err := embeddedFS.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// --- API 处理器 ---

func statusHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, docker.GetAppStatus())
}

func progressHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, docker.GetProgress())
}

func setHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "只允许POST方法")
		return
	}

	config.UpdateConfig(func(c *config.Config) {
		c.URL = r.FormValue("url")
		c.PlanType = r.FormValue("plan_type")
		c.Dir = r.FormValue("dir")
		if val, err := strconv.Atoi(r.FormValue("interval_minutes")); err == nil {
			c.IntervalMinutes = val
		}
		if val, err := strconv.Atoi(r.FormValue("hour")); err == nil {
			c.Hour = val
		}
		if val, err := strconv.Atoi(r.FormValue("minute")); err == nil {
			c.Minute = val
		}
		if val, err := strconv.Atoi(r.FormValue("speed")); err == nil {
			c.SpeedKB = val
		}
		if val, err := strconv.Atoi(r.FormValue("limit_mb")); err == nil {
			c.LimitMB = val
		}
	})

	if err := config.SaveConfig(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	// 更新下载目录并确保它存在
	cfg := config.GetConfig()
	os.MkdirAll(cfg.Dir, 0755)

	docker.UpdateMessage("配置已保存")
	respondWithJSON(w, http.StatusOK, docker.GetAppStatus())
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "只允许POST方法")
		return
	}

	go func() {
		cfg := config.GetConfig()
		if cfg.URL == "" {
			docker.UpdateMessage("下载失败: 未设置下载地址")
			return
		}

		// 检查下载限制 - 使用配置文件中的设置
		status := docker.GetAppStatus()
		if cfg.DailyLimitEnabled && status.Stats.DailyDownloadedMB >= cfg.LimitMB {
			docker.UpdateMessage("今日下载量已达上限")
			return
		}

		docker.SetTaskStatus("下载中")
		docker.NewDownloadContext() // 为这次手动下载创建一个新的上下文

		file, size, err := downloader.DownloadFileWithProgress(docker.GetDownloadContext(), cfg.URL, cfg.SpeedKB, cfg.Dir)
		if err != nil {
			docker.SetTaskStatus("失败")
			docker.UpdateMessage("下载失败: %v", err)
		} else {
			docker.SetTaskStatus("空闲")
			docker.UpdateMessage("下载成功: %s", file)
			docker.UpdateLastDownloadInfo(file, true)
			docker.AddDownloadStats(size)
		}
	}()

	respondWithJSON(w, http.StatusAccepted, docker.GetAppStatus())
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "只允许POST方法")
		return
	}
	docker.NewDownloadContext() // 取消当前下载上下文
	docker.UpdateMessage("已发送停止信号")
	respondWithJSON(w, http.StatusOK, docker.GetAppStatus())
}

func toggleTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "只允许POST方法")
		return
	}
	
	enabled, err := config.ToggleTaskEnabled()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "切换任务状态失败: "+err.Error())
		return
	}
	
	msg := "禁用"
	if enabled {
		msg = "启用"
	}
	docker.UpdateMessage("定时任务已%s", msg)
	respondWithJSON(w, http.StatusOK, docker.GetAppStatus())
}

func toggleLimitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "只允许POST方法")
		return
	}
	
	enabled, err := config.ToggleDailyLimitEnabled()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "切换限制状态失败: "+err.Error())
		return
	}
	
	msg := "关闭"
	if enabled {
		msg = "启用"
	}
	docker.UpdateMessage("每日下载量限制已%s", msg)
	respondWithJSON(w, http.StatusOK, docker.GetAppStatus())
}

func cleanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "只允许POST方法")
		return
	}
	go func() {
		count := dockerPkg.CleanCache()
		docker.UpdateMessage("清理了 %d 个缓存文件", count)
	}()
	respondWithJSON(w, http.StatusAccepted, docker.GetAppStatus())
}

// --- 辅助函数 ---

func respondWithError(w http.ResponseWriter, code int, message string) {
	log.Printf("API Error: %s", message)
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "无法序列化响应: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
