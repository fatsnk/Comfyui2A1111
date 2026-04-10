package api

import (
	"encoding/json"
	"io"
	"net/http"
	"novel-api/comfyui"
	"novel-api/config"
	"os"
	"path/filepath"
	"strings"
)

// ListWorkflows 列出所有工作流
func ListWorkflows(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 Token
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !isValidToken(token) && (cfg.LogsAdmin.Password != "" && token != cfg.LogsAdmin.Password) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	workflowsDir := cfg.ComfyUI.WorkflowsDir
	if workflowsDir == "" {
		workflowsDir = "comfyui-workflows"
	}

	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var workflows []map[string]interface{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			filePath := filepath.Join(workflowsDir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Try to load params if exists
			params, _ := comfyui.LoadWorkflowParams(workflowsDir, entry.Name())
			
			// Load workflow just to detect kind
			data, err := os.ReadFile(filePath)
			kind := "unknown"
			if err == nil {
				var obj map[string]interface{}
				if err := json.Unmarshal(data, &obj); err == nil {
					if prompt, err := comfyui.ExtractPromptAndExtra(obj); err == nil {
						kind = comfyui.DetectKind(prompt)
					}
				}
			}

			workflow := map[string]interface{}{
				"name":     entry.Name(),
				"size":     info.Size(),
				"mod_time": info.ModTime(),
				"kind":     kind,
			}
			
			if params != nil {
				workflow["has_params"] = true
				workflow["params"] = params
			} else {
				workflow["has_params"] = false
			}

			workflows = append(workflows, workflow)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    workflows,
	})
}

// UploadWorkflow 上传工作流
func UploadWorkflow(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 Token (这里复用后端的鉴权逻辑，假设在路由层已经处理，或者在这里简单校验)
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !isValidToken(token) && (cfg.LogsAdmin.Password != "" && token != cfg.LogsAdmin.Password) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".json") {
		http.Error(w, "Only JSON files are allowed", http.StatusBadRequest)
		return
	}

	workflowsDir := cfg.ComfyUI.WorkflowsDir
	if workflowsDir == "" {
		workflowsDir = "comfyui-workflows"
	}

	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filePath := filepath.Join(workflowsDir, header.Filename)
	
	// Read into memory first to validate
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}
	
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}
	
	_, err = comfyui.ExtractPromptAndExtra(obj)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Workflow uploaded successfully",
		"name":    header.Filename,
	})
}

// GetWorkflowTargets 获取候选节点
func GetWorkflowTargets(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 Token
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !isValidToken(token) && (cfg.LogsAdmin.Password != "" && token != cfg.LogsAdmin.Password) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing workflow name", http.StatusBadRequest)
		return
	}

	workflowsDir := cfg.ComfyUI.WorkflowsDir
	if workflowsDir == "" {
		workflowsDir = "comfyui-workflows"
	}

	filePath := filepath.Join(workflowsDir, name)
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		http.Error(w, "Invalid workflow JSON", http.StatusInternalServerError)
		return
	}

	targets, err := comfyui.GetWorkflowTargets(obj)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    targets,
	})
}

// SaveWorkflowParams 保存参数映射配置
func SaveWorkflowParams(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 Token
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !isValidToken(token) && (cfg.LogsAdmin.Password != "" && token != cfg.LogsAdmin.Password) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Name   string                 `json:"name"`
		Params comfyui.WorkflowParams `json:"params"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Missing workflow name", http.StatusBadRequest)
		return
	}

	workflowsDir := cfg.ComfyUI.WorkflowsDir
	if workflowsDir == "" {
		workflowsDir = "comfyui-workflows"
	}

	req.Params.Version = 1
	if err := comfyui.SaveWorkflowParams(workflowsDir, req.Name, &req.Params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Params saved successfully",
	})
}

// DeleteWorkflow 删除工作流
func DeleteWorkflow(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 Token
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !isValidToken(token) && (cfg.LogsAdmin.Password != "" && token != cfg.LogsAdmin.Password) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing workflow name", http.StatusBadRequest)
		return
	}

	workflowsDir := cfg.ComfyUI.WorkflowsDir
	if workflowsDir == "" {
		workflowsDir = "comfyui-workflows"
	}

	// 删除 JSON 文件
	filePath := filepath.Join(workflowsDir, name)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		http.Error(w, "Failed to delete workflow", http.StatusInternalServerError)
		return
	}

	// 删除参数配置文件
	ext := filepath.Ext(name)
	nameWithoutExt := name[:len(name)-len(ext)]
	sidecarPath := filepath.Join(workflowsDir, ".comfyui2api", nameWithoutExt+".params.json")
	_ = os.Remove(sidecarPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Workflow deleted successfully",
	})
}