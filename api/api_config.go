package api

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"novel-api/config"
	"strings"

	"gopkg.in/yaml.v2"
)

// ConfigResponse 配置查询响应
type ConfigResponse struct {
	Success bool   `json:"success"`
	Data    ConfigData `json:"data"`
	Message string `json:"message,omitempty"`
}

type ConfigData struct {
	NovelAIProviders    []config.Provider `json:"novel_ai_providers"`
	NovelAIBaseURL      string            `json:"novel_ai_base_url"`
	NovelAIKey          string            `json:"novel_ai_key"`
	NovelAIA1111Path    string            `json:"novel_ai_a1111_path"`
	NovelAIA1111NoSave  bool              `json:"novel_ai_a1111_no_save"`
	ComfyUIBaseURL      string            `json:"comfyui_base_url"`
	ComfyUIAutoRoute    bool              `json:"comfyui_auto_route"`
	ComfyUIDefaultRoute string            `json:"comfyui_default_route"`
}

// ConfigUpdateRequest 配置更新请求
type ConfigUpdateRequest struct {
	NovelAIProviders    []config.Provider `json:"novel_ai_providers"`
	NovelAIBaseURL      string            `json:"novel_ai_base_url"`
	NovelAIKey          string            `json:"novel_ai_key"`
	NovelAIA1111Path    string            `json:"novel_ai_a1111_path"`
	NovelAIA1111NoSave  bool              `json:"novel_ai_a1111_no_save"`
	ComfyUIBaseURL      string            `json:"comfyui_base_url"`
	ComfyUIAutoRoute    bool              `json:"comfyui_auto_route"`
	ComfyUIDefaultRoute string            `json:"comfyui_default_route"`
}

// GetConfig 获取配置API
func GetConfig(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 验证token
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if !isValidToken(token) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "未授权访问",
		})
		return
	}

	response := ConfigResponse{
		Success: true,
		Data: ConfigData{
			NovelAIProviders:    cfg.NovelAI.Providers,
			NovelAIBaseURL:      cfg.NovelAI.BaseURL,
			NovelAIKey:          cfg.NovelAI.Key,
			NovelAIA1111Path:    cfg.NovelAI.A1111Path,
			NovelAIA1111NoSave:  cfg.NovelAI.A1111NoSave,
			ComfyUIBaseURL:      cfg.ComfyUI.BaseURL,
			ComfyUIAutoRoute:    cfg.ComfyUI.AutoRoute,
			ComfyUIDefaultRoute: cfg.ComfyUI.DefaultRoute,
		},
	}
	json.NewEncoder(w).Encode(response)
}

// UpdateConfig 更新配置API (内存更新)
func UpdateConfig(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 验证token
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if !isValidToken(token) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "未授权访问",
		})
		return
	}

	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "无效的请求格式",
		})
		return
	}

	// 更新内存中的配置
	cfg.NovelAI.Providers = req.NovelAIProviders
	cfg.NovelAI.BaseURL = req.NovelAIBaseURL
	cfg.NovelAI.Key = req.NovelAIKey
	cfg.NovelAI.A1111Path = strings.Trim(req.NovelAIA1111Path, "/")
	cfg.NovelAI.A1111NoSave = req.NovelAIA1111NoSave
	cfg.ComfyUI.BaseURL = req.ComfyUIBaseURL
	cfg.ComfyUI.AutoRoute = req.ComfyUIAutoRoute
	if req.ComfyUIDefaultRoute == "" {
		req.ComfyUIDefaultRoute = "novelai"
	}
	cfg.ComfyUI.DefaultRoute = req.ComfyUIDefaultRoute

	// 将配置持久化到 .env (YAML格式) 文件
	data, err := yaml.Marshal(cfg)
	if err == nil {
		err = ioutil.WriteFile(".env", data, 0644)
		if err != nil {
			log.Printf("Failed to save config to .env: %v", err)
		} else {
			log.Printf("Config successfully saved to .env")
		}
	} else {
		log.Printf("Failed to marshal config: %v", err)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "配置更新成功",
	})
}