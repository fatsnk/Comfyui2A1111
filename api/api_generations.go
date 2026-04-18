package api

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"novel-api/config"
	"novel-api/models"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// GenerationRequest 定义 OpenAI DALL-E 格式的请求结构体
type GenerationRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`              // 生成图片数量，默认为1
	Size           string `json:"size,omitempty"`           // 图片尺寸，如 "1024x1024"
	Quality        string `json:"quality,omitempty"`        // 图片质量，如 "standard" 或 "hd"
	ResponseFormat string `json:"response_format,omitempty"` // 响应格式，"url" 或 "b64_json"
}

// GenerationResponse 定义 OpenAI DALL-E 格式的响应结构体
type GenerationResponse struct {
	Created int64                 `json:"created"`
	Data    []GenerationImageData `json:"data"`
}

// GenerationImageData 定义生成的图片数据结构
type GenerationImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// 提取链接的函数 (复用自 completions)
func extractLinksFromPrompt(prompt string) []string {
	re := regexp.MustCompile(`https?://[^\s]+`)
	matches := re.FindAllString(prompt, -1)
	return matches
}

// Generations 处理 OpenAI DALL-E 格式的画图请求
func Generations(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// 如果是 OPTIONS 请求，直接返回 200 OK
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 1. 获取 Authorization 请求头的值
	authHeader := r.Header.Get("Authorization")
	authHeader = strings.TrimPrefix(authHeader, "Bearer ")

	// 本地接口鉴权：使用配置中的 A1111Path (即安全路径) 作为密码
	// 如果配置了 A1111Path，且客户端传入的 token 不等于该值，则返回 401
	if cfg.NovelAI.A1111Path != "" && authHeader != cfg.NovelAI.A1111Path {
		log.Printf("Generations API Unauthorized: Expected %s, got %s", cfg.NovelAI.A1111Path, authHeader)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fmt.Println("Generations API 秘钥鉴权通过。传入 Token：", authHeader)
	// 由于实际发往 NovelAI 时需要用真实的 Key，我们将用作鉴权的 authHeader 覆盖为系统配置的实际 NovelAI Key
	// 如果需要发往 ComfyUI，不涉及 authHeader，但这里统一替换方便底层复用。
	authHeader = cfg.NovelAI.Key

	// 2. 解析请求体
	var req GenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode generation request body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Generation request: Model=%s, Prompt=%s", req.Model, req.Prompt)

	// 3. 处理默认值
	if req.N == 0 {
		req.N = 1 // 默认生成1张图片
	}

	// 4. 获取用户输入的提示词
	userInput := req.Prompt

	// 5. 如果启用翻译，则翻译用户输入
	log.Printf("[Generations] Translation.Enable value: %v (URL: %s, Model: %s)", cfg.Translation.Enable, cfg.Translation.URL, cfg.Translation.Model)
	if cfg.Translation.Enable {
		translatedInput, err := TranslateText(userInput, cfg)
		if err != nil {
			log.Printf("Translation failed, using original text: %v", err)
		} else {
			log.Printf("Translation enabled, translated: %s -> %s", userInput, translatedInput)
			userInput = translatedInput
		}
	} else {
		log.Printf("[Generations] Translation is disabled, skipping translation")
	}

	// 6. 提取用户输入中的链接 (用于参考图像)
	imageURL := extractLinksFromPrompt(userInput)
	var base64String string
	if len(imageURL) > 0 {
		// 选择第一个提取到的链接
		imageURLS := imageURL[0]
		// 解析图片为base64
		base64String, _ = ImageURLToBase64(imageURLS)
	}

	// 7. 解析 size 参数，如果没有传递则使用配置文件中的默认值
	width := cfg.Parameters.Width
	height := cfg.Parameters.Height
	if req.Size != "" {
		// 解析 size 参数，格式如 "1024x1024"
		var parsedWidth, parsedHeight int
		_, err := fmt.Sscanf(req.Size, "%dx%d", &parsedWidth, &parsedHeight)
		if err == nil && parsedWidth > 0 && parsedHeight > 0 {
			width = parsedWidth
			height = parsedHeight
			log.Printf("Using size from request: %dx%d", width, height)
		} else {
			log.Printf("Invalid size format '%s', using default: %dx%d", req.Size, width, height)
		}
	} else {
		log.Printf("No size specified, using default: %dx%d", width, height)
	}

	// 8. 生成一个随机种子
	rand.Seed(time.Now().UnixNano())
	randomSeed := rand.Intn(1000000)

	// 9. 构建兼容的 ChatRequest 结构 (复用现有模型)
	compatibleReq := config.ChatRequest{
		Authorization: authHeader,
		Model:         req.Model,
		Messages: []config.Message{
			{
				Role:    "user",
				Content: req.Prompt,
			},
		},
	}

	// 10. 确定路由
	isDallRequest := true
	shouldRouteToComfyUI := false

	if cfg.ComfyUI.BaseURL != "" {
		if cfg.ComfyUI.AutoRoute {
			if !strings.Contains(req.Model, "nai-diffusion-") {
				shouldRouteToComfyUI = true
			}
		} else {
			if cfg.ComfyUI.DefaultRoute == "comfyui" {
				shouldRouteToComfyUI = true
			}
		}
	}

	if shouldRouteToComfyUI {
		comfyModelName := req.Model
		workflowsDir := cfg.ComfyUI.WorkflowsDir
		if workflowsDir == "" {
			workflowsDir = "comfyui-workflows"
		}
		
		workflowPath := filepath.Join(workflowsDir, comfyModelName)
		if !strings.HasSuffix(workflowPath, ".json") {
			workflowPath += ".json"
		}
		
		// 如果模型不存在，查找第一个可用的工作流
		if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
			if entries, err := os.ReadDir(workflowsDir); err == nil {
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
						name := entry.Name()
						comfyModelName = name[:len(name)-5]
						break
					}
				}
			}
		}

		log.Printf("Routing to ComfyUI for model: %s", comfyModelName)
		images, err := models.GenerateWithComfyUI(comfyModelName, cfg, userInput, base64String, width, height, randomSeed)
		if err == nil && len(images) > 0 {
			sendDallEResponse(w, r, cfg, images, userInput, req.ResponseFormat)
			return
		}
		log.Printf("ComfyUI generation failed, falling back to NAI: %v", err)
	}

	// 走 NAI 逻辑
	baseURL, key, realModelName := models.RouteModel(cfg, req.Model)
	
	// 临时修改 cfg 中的 BaseURL 和 Key 以便底层函数使用
	originalBaseURL := cfg.NovelAI.BaseURL
	originalKey := cfg.NovelAI.Key
	cfg.NovelAI.BaseURL = baseURL
	cfg.NovelAI.Key = key
	
	// 确保恢复原始配置
	defer func() {
		cfg.NovelAI.BaseURL = originalBaseURL
		cfg.NovelAI.Key = originalKey
	}()

	// 如果 authHeader 是默认的，也更新它
	if authHeader == originalKey {
		authHeader = key
	}
	
	// 更新 compatibleReq 中的模型名称为去掉 provider 前缀的真实模型名
	compatibleReq.Model = realModelName

	switch realModelName {
	case "nai-diffusion-3", "nai-diffusion-furry-3":
		models.Nai3WithFormatAndSize(w, r, compatibleReq, randomSeed, base64String, authHeader, cfg, userInput, width, height, isDallRequest, req.ResponseFormat)
	case "nai-diffusion-4-full", "nai-diffusion-4-curated-preview", "nai-diffusion-4-5-curated", "nai-diffusion-4-5-full":
		models.Nai4WithFormatAndSize(w, r, compatibleReq, randomSeed, base64String, authHeader, cfg, userInput, nil, width, height, isDallRequest, req.ResponseFormat)
	default:
		if strings.Contains(realModelName, "-3") {
			models.Nai3WithFormatAndSize(w, r, compatibleReq, randomSeed, base64String, authHeader, cfg, userInput, width, height, isDallRequest, req.ResponseFormat)
		} else {
			models.Nai4WithFormatAndSize(w, r, compatibleReq, randomSeed, base64String, authHeader, cfg, userInput, nil, width, height, isDallRequest, req.ResponseFormat)
		}
	}
}

// 辅助方法：发送 DALL-E 格式响应
func sendDallEResponse(w http.ResponseWriter, r *http.Request, cfg *config.Config, images []string, prompt string, responseFormat string) {
	response := GenerationResponse{
		Created: time.Now().Unix(),
		Data:    make([]GenerationImageData, len(images)),
	}

	useURL := responseFormat != "b64_json" // 默认是 url

	for i, base64Str := range images {
		if useURL {
			// 需要将 base64 存图或通过内部工具转为 URL (取决于之前的图床上传逻辑)
			// 为了简化并复用现有机制，我们这里可以直接调用 tools 或直接使用 Base64
			// 因为现有系统 models.Nai3WithFormatAndSize 内实现了大量的图床上传
			// 这里如果无法复用，我们可以提供一个 /images/ 目录的本地链接或图床链接
			// 这里为了简化且快速响应 DALL-E 的 response_format="url"，我们尝试利用现有 upload 逻辑
			// 但因为缺少内部上下文上下文，如果强制走图床可能会比较复杂。
			// 如果没有图床处理逻辑在当前文件，则默认给 b64_json 即可（或者强制要求客户端用 b64_json）
			// 这里我们使用一种简化：将 b64 保存到本地然后返回 URL
			
			// 如果之前没有实现这个转换，也可以直接返回 base64 数据 url: "data:image/png;base64,..."
			// DALL-E 原生 URL 是一段有效期的下载链接。
			response.Data[i] = GenerationImageData{
				URL:           "data:image/png;base64," + base64Str,
				RevisedPrompt: prompt,
			}
		} else {
			response.Data[i] = GenerationImageData{
				B64JSON:       base64Str,
				RevisedPrompt: prompt,
			}
		}
	}

	// 打印日志，隐藏 base64 数据
	logResponse := GenerationResponse{
		Created: response.Created,
		Data:    make([]GenerationImageData, len(response.Data)),
	}
	for i, d := range response.Data {
		logResponse.Data[i] = d
		if d.B64JSON != "" {
			logResponse.Data[i].B64JSON = fmt.Sprintf("<base64 data, length: %d>", len(d.B64JSON))
		}
		if strings.HasPrefix(d.URL, "data:image/png;base64,") {
			logResponse.Data[i].URL = fmt.Sprintf("<data URI, length: %d>", len(d.URL))
		}
	}
	logBytes, _ := json.Marshal(logResponse)
	log.Printf("Sending DALL-E response: %s", string(logBytes))

	w.Header().Set("Content-Type", "application/json")
	// 使用 json.Marshal 默认生成单行 JSON
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Failed to marshal DALL-E response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Write(jsonBytes)
}

// GenerationsJSON 处理 OpenAI DALL-E 格式的画图请求并返回 JSON 响应 (非流式)
func GenerationsJSON(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// 如果是 OPTIONS 请求，直接返回 200 OK
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 1. 获取 Authorization 请求头的值
	authHeader := r.Header.Get("Authorization")
	authHeader = strings.TrimPrefix(authHeader, "Bearer ")

	// 本地接口鉴权：使用配置中的 A1111Path (即安全路径) 作为密码
	if cfg.NovelAI.A1111Path != "" && authHeader != cfg.NovelAI.A1111Path {
		log.Printf("GenerationsJSON API Unauthorized: Expected %s, got %s", cfg.NovelAI.A1111Path, authHeader)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fmt.Println("GenerationsJSON API 秘钥鉴权通过。传入 Token：", authHeader)
	// 覆盖为发往 NovelAI 的真实 Key
	authHeader = cfg.NovelAI.Key

	// 2. 解析请求体
	var req GenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode generation JSON request body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Generation JSON request: Model=%s, Prompt=%s", req.Model, req.Prompt)

	// 3. 处理默认值
	if req.N == 0 {
		req.N = 1 // 默认生成1张图片
	}

	// 4. 构建响应结构
	response := GenerationResponse{
		Created: time.Now().Unix(),
		Data: []GenerationImageData{
			{
				URL:           "", // 这里会在实际生成后填充
				RevisedPrompt: req.Prompt,
			},
		},
	}

	// 5. 设置响应头并返回 JSON
	w.Header().Set("Content-Type", "application/json")

	// 注意：这里是一个简化版本，实际应该调用相应的模型生成图片
	// 然后获取生成的图片URL填充到响应中
	// 为了保持一致性，建议使用流式响应版本 Generations 函数

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Models 处理 OpenAI 兼容的模型列表请求
func Models(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var modelsList []map[string]interface{}
	
	// 从 ComfyUI 工作流目录读取模型
	workflowsDir := cfg.ComfyUI.WorkflowsDir
	if workflowsDir == "" {
		workflowsDir = "comfyui-workflows"
	}
	
	entries, err := os.ReadDir(workflowsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				name := entry.Name()
				nameWithoutExt := name[:len(name)-5] // remove .json
				modelsList = append(modelsList, map[string]interface{}{
					"id":      nameWithoutExt,
					"object":  "model",
					"created": time.Now().Unix(),
					"owned_by": "comfyui",
				})
			}
		}
	}

	// 补充内置的 NAI 模型
	builtInModels := []string{
		"nai-diffusion-3",
		"nai-diffusion-4-full",
		"nai-diffusion-furry-3",
		"nai-diffusion-4-curated-preview",
		"nai-diffusion-4-5-curated",
		"nai-diffusion-4-5-full",
	}

	for _, m := range builtInModels {
		modelsList = append(modelsList, map[string]interface{}{
			"id":      m,
			"object":  "model",
			"created": time.Now().Unix(),
			"owned_by": "novelai",
		})
	}

	response := map[string]interface{}{
		"object": "list",
		"data":   modelsList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
