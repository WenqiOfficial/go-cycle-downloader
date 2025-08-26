package docker

import (
	"log"
	"os"
	"path/filepath"
	// "time"

	"docker-cycler/pkg/config"
)

// const cacheDuration = 24 * time.Hour // 超过这个时间的文件会被清理

// CleanCache 遍历下载目录并删除过期的文件
func CleanCache() int {
	cfg := config.GetConfig()
	files, err := os.ReadDir(cfg.Dir)
	if err != nil {
		log.Printf("清理缓存失败，无法读取目录 '%s': %v", cfg.Dir, err)
		return 0
	}

	count := 0
	// now := time.Now()
	for _, f := range files {
		if f.IsDir() {
			continue // 跳过子目录
		}
		info, err := f.Info()
		if err != nil {
			continue
		}

		_ = info

		// 删除过期文件
		// if now.Sub(info.ModTime()) > cacheDuration {
		// 	err := os.Remove(filepath.Join(cfg.Dir, f.Name()))
		// 	if err != nil {
		// 		log.Printf("无法删除文件 '%s': %v", f.Name(), err)
		// 	} else {
		// 		count++
		// 	}
		// }

		// 删除所有文件
		err = os.Remove(filepath.Join(cfg.Dir, f.Name()))
		if err != nil {
			log.Printf("无法删除文件 '%s': %v", f.Name(), err)
		} else {
			count++
		}
	}
	return count
}
