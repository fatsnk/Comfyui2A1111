package upload

import (
	"fmt"
	"log"
	"novel-api/config"
	"os"
	"path/filepath"
	"strings"
)

// LocalUploader 本地存储上传器
type LocalUploader struct {
	cfg *config.Config
}

// NewLocalUploader 创建本地存储上传器
func NewLocalUploader(cfg *config.Config) (*LocalUploader, error) {
	if cfg.Local.Path == "" {
		cfg.Local.Path = "./images" // 默认路径
	}
	// 确保目录存在
	if err := os.MkdirAll(cfg.Local.Path, os.ModePerm); err != nil {
		return nil, fmt.Errorf("创建本地存储目录失败: %v", err)
	}
	return &LocalUploader{cfg: cfg}, nil
}

// UploadFromBytes 实现本地保存
func (u *LocalUploader) UploadFromBytes(data []byte, fileName, folder string) (*UploadResponse, error) {
	// 保存路径为 cfg.Local.Path / fileName
	filePath := filepath.Join(u.cfg.Local.Path, fileName)
	
	err := os.WriteFile(filePath, data, 0644)
	if err != nil {
		log.Printf("本地文件写入失败: %v", err)
		return &UploadResponse{
			Success: false,
			Message: fmt.Sprintf("本地文件写入失败: %v", err),
		}, err
	}
	
	baseURL := strings.TrimSuffix(u.cfg.Local.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:" + u.cfg.Server.Addr + "/images"
	}
	
	fileURL := fmt.Sprintf("%s/%s", baseURL, fileName)

	return &UploadResponse{
		Success: true,
		Message: "Upload success",
		Data: struct {
			URL      string `json:"url"`
			Key      string `json:"key"`
			Size     int64  `json:"size"`
			FileName string `json:"filename"`
		}{
			URL:      fileURL,
			Key:      fileName,
			Size:     int64(len(data)),
			FileName: fileName,
		},
	}, nil
}