package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/time/rate"
	"docker-cycler/pkg/docker"
)

// rateLimitedReader 实现了限速的io.Reader
type rateLimitedReader struct {
	reader  io.Reader
	limiter *rate.Limiter
	ctx     context.Context
}

func (r *rateLimitedReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.limiter != nil {
		// 等待令牌
		if err := r.limiter.WaitN(r.ctx, n); err != nil {
			return n, err
		}
	}
	return n, err
}

// progressWriter 是一个自定义的 io.Writer，用于跟踪下载进度和速度
type progressWriter struct {
	total      int64
	size       int64
	lastUpdate time.Time
	lastBytes  int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.total += int64(n)

	now := time.Now()
	if now.Sub(pw.lastUpdate) > time.Second || pw.total == pw.size {
		elapsed := now.Sub(pw.lastUpdate).Seconds()
		if elapsed == 0 {
			elapsed = 1 // 避免除以零
		}
		speed := float64(pw.total-pw.lastBytes) / 1024 / elapsed

		percent := 0
		totalKB := int(pw.total / 1024)
		if pw.size > 0 {
			percent = int(float64(pw.total) * 100 / float64(pw.size))
			docker.SetProgress(percent, int(speed), int(pw.size/1024), "下载中")
		} else {
			// 未知文件大小时，显示已下载的大小
			docker.SetProgress(0, int(speed), totalKB, "下载中")
		}

		pw.lastUpdate = now
		pw.lastBytes = pw.total
	}

	return n, nil
}

// downloadFileWithProgress 使用令牌桶算法进行限速，并提供精确的进度回调
func DownloadFileWithProgress(ctx context.Context, urlStr string, speedKB int, downloadDir string) (string, int, error) {

	// 清除缓存
	docker.CleanCache()

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		docker.SetProgress(0, 0, 0, "下载失败: "+err.Error())
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
		docker.SetProgress(0, 0, 0, err.Error())
		return "", 0, err
	}

	size := resp.ContentLength
	if size < 0 {
		size = 0 // 未知文件大小
	}
	filename := filepath.Join(downloadDir, fmt.Sprintf("file_%d", time.Now().Unix()))
	out, err := os.Create(filename)
	if err != nil {
		docker.SetProgress(0, 0, 0, "创建文件失败")
		return "", 0, err
	}
	defer out.Close()

	// 初始化进度写入器
	pw := &progressWriter{size: size, lastUpdate: time.Now()}

	// 设置读取器，根据是否限速进行包装
	var reader io.Reader = resp.Body
	if speedKB > 0 {
		// 令牌桶：每秒产生 speedKB * 1024 个令牌，桶容量为 2 倍的每秒速率
		limiter := rate.NewLimiter(rate.Limit(speedKB*1024), speedKB*1024*2)
		reader = &rateLimitedReader{
			reader:  resp.Body,
			limiter: limiter,
			ctx:     ctx,
		}
	}

	// 使用 MultiWriter 将数据同时写入文件和进度跟踪器
	mw := io.MultiWriter(out, pw)

	// 开始下载
	sizeKB := int(size / 1024)
	if size == 0 {
		sizeKB = 0 // 未知大小
	}
	docker.SetProgress(0, 0, sizeKB, "下载中")
	_, err = io.Copy(mw, reader)
	if err != nil {
		// 检查是否是 context cancel 导致的错误
		currentKB := int(pw.total / 1024)
		if size > 0 {
			currentKB = int(size / 1024)
		}
		if err == context.Canceled {
			docker.SetProgress(pw.Percent(), 0, currentKB, "已手动停止")
			return filename, int(pw.total), fmt.Errorf("下载被手动停止")
		}
		docker.SetProgress(pw.Percent(), 0, currentKB, "下载失败: "+err.Error())
		return filename, int(pw.total), err
	}

	finalKB := int(pw.total / 1024)
	if size > 0 {
		finalKB = int(size / 1024)
	}
	docker.SetProgress(100, 0, finalKB, "下载完成")
	return filename, int(pw.total), nil
}

// Percent 计算当前下载百分比
func (pw *progressWriter) Percent() int {
	if pw.size <= 0 {
		return 0
	}
	return int(float64(pw.total) * 100 / float64(pw.size))
}
