package api

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"novel-api/config"
	"novel-api/models"
	"os"
	"strings"
	"time"
)

// A1111Txt2ImgRequest 兼容 A1111 sdapi/v1/txt2img 的请求结构
type A1111Txt2ImgRequest struct {
	Prompt           string                 `json:"prompt"`
	NegativePrompt   string                 `json:"negative_prompt,omitempty"`
	Steps            int                    `json:"steps,omitempty"`
	Width            int                    `json:"width,omitempty"`
	Height           int                    `json:"height,omitempty"`
	SamplerName      string                 `json:"sampler_name,omitempty"`
	CfgScale         float64                `json:"cfg_scale,omitempty"`
	Seed             int                    `json:"seed,omitempty"`
	BatchSize        int                    `json:"batch_size,omitempty"`
	OverrideSettings map[string]interface{} `json:"override_settings,omitempty"`
	// 可根据需要添加其他 A1111 支持的参数，当前仅提取影响NovelAI生成的关键参数
}

// A1111Txt2Img 处理 A1111 兼容的文生图请求 (无验证)
func A1111Txt2Img(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// 如果是 OPTIONS 请求，直接返回 200 OK
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. 解析请求体
	var req A1111Txt2ImgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode A1111 request body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("A1111 Txt2Img request: Prompt=%s", req.Prompt)

	// 2. 处理参数默认值，如果没有提供，则使用配置文件的默认值
	width := cfg.Parameters.Width
	if req.Width > 0 {
		width = req.Width
	}

	height := cfg.Parameters.Height
	if req.Height > 0 {
		height = req.Height
	}

	steps := cfg.Parameters.Steps
	if req.Steps > 0 {
		steps = req.Steps
	}

	cfgScale := cfg.Parameters.Scale
	if req.CfgScale > 0 {
		cfgScale = req.CfgScale
	}

	seed := req.Seed
	if seed <= -1 {
		rand.Seed(time.Now().UnixNano())
		seed = rand.Intn(1000000)
	}

	// 3. 构造给 NovelAI 模型的临时配置对象（为了不覆盖全局配置，这里可以临时修改）
	// 注意：因为我们目前使用的模型生成函数直接接收 cfg 参数，所以我们先深拷贝一个 cfg 副本用于本次请求
	tempCfg := *cfg
	tempCfg.Parameters.Width = width
	tempCfg.Parameters.Height = height
	tempCfg.Parameters.Steps = steps
	tempCfg.Parameters.Scale = cfgScale
	
	// 如果提供了负面提示词并且不为空，覆盖默认的
	if strings.TrimSpace(req.NegativePrompt) != "" {
		tempCfg.Parameters.CustomAntiWords = req.NegativePrompt
	}

	// 4. 获取用户输入的提示词
	userInput := req.Prompt

	// 5. 提取图片链接(可选，A1111的txt2img一般不带，如果有按原逻辑处理)
	imageURL := extractLinksFromPrompt(userInput)
	var base64String string
	if len(imageURL) > 0 {
		imageURLS := imageURL[0]
		base64String, _ = ImageURLToBase64(imageURLS)
	}

	// 6. 确定使用的模型
	modelName := ""
	if override, ok := req.OverrideSettings["sd_model_checkpoint"]; ok {
		if modelStr, ok := override.(string); ok && modelStr != "" {
			modelName = modelStr
		}
	}

	// 如果未指定模型，优先使用第一个 ComfyUI 工作流，若无则使用 NAI
	if modelName == "" {
		workflowsDir := cfg.ComfyUI.WorkflowsDir
		if workflowsDir == "" {
			workflowsDir = "comfyui-workflows"
		}
		if entries, err := os.ReadDir(workflowsDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
					name := entry.Name()
					modelName = name[:len(name)-5] // 移除 .json 后缀
					break
				}
			}
		}
		
		if modelName == "" {
			modelName = "nai-diffusion-3"
		}
	}

	// 构建兼容的 ChatRequest 结构
	compatibleReq := config.ChatRequest{
		Authorization: tempCfg.NovelAI.Key, // A1111无验证，使用全局配置的默认Key
		Model:         modelName,
		Messages: []config.Message{
			{
				Role:    "user",
				Content: userInput,
			},
		},
	}

	// 7. 标识这是 DALL-E 格式请求 (用于一些早期的兼容逻辑，现在我们在模型内部通过 r.URL.Path 判断A1111)
	isDallRequest := true

	// 8. 路由到对应的模型生成函数 (V3 或 V4/V4.5)
	// 根据您的要求：如果模型名称包含 "3"，走 V3 逻辑；否则（其他所有模型名，如带 4 或更新的名称）默认走 V4(最新) 逻辑。

	// 8. 路由到对应的模型生成函数 (ComfyUI / V3 / V4)
	if cfg.ComfyUI.BaseURL != "" {
		shouldRouteToComfyUI := false
		
		// 移除可能存在的 provider 前缀进行匹配检查
		checkModelName := modelName
		if dotIdx := strings.Index(checkModelName, "."); dotIdx != -1 {
			checkModelName = checkModelName[dotIdx+1:]
		}

		if cfg.ComfyUI.AutoRoute {
			// 开启自动路由：非 nai-diffusion- 开头的尝试走 ComfyUI
			if !strings.HasPrefix(checkModelName, "nai-diffusion-") {
				shouldRouteToComfyUI = true
			}
		} else {
			// 关闭自动路由：根据默认去向决定
			if cfg.ComfyUI.DefaultRoute == "comfyui" {
				shouldRouteToComfyUI = true
			}
		}

		if shouldRouteToComfyUI {
			log.Printf("A1111: Routing request to ComfyUI (URL: %s, Model: %s)", cfg.ComfyUI.BaseURL, modelName)
			images, err := models.GenerateWithComfyUI(modelName, &tempCfg, userInput, base64String, width, height, seed)
			if err == nil && len(images) > 0 {
				log.Printf("A1111: ComfyUI generation successful, returned %d images", len(images))
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"images": images,
					"parameters": req,
					"info": "",
				})
				return // 成功处理则返回
			}
			// 只在尝试调用 ComfyUI 但由于某些原因（例如工作流不存在或生成报错）失败时，才打印 fall back 日志
			if err != nil {
				log.Printf("A1111 ComfyUI generation failed for model %s, falling back to NAI: %v", modelName, err)
			}
		}
	}

	// 走 NAI 逻辑
	baseURL, key, realModelName := models.RouteModel(&tempCfg, modelName)
	
	// 临时修改 tempCfg 中的 BaseURL 和 Key 以便底层函数使用
	tempCfg.NovelAI.BaseURL = baseURL
	tempCfg.NovelAI.Key = key
	
	// 更新 compatibleReq 中的模型名称为去掉 provider 前缀的真实模型名
	compatibleReq.Model = realModelName

	if strings.Contains(realModelName, "-3") {
		log.Printf("A1111: Routing model '%s' to V3 logic", realModelName)
		models.Nai3WithFormatAndSize(w, r, compatibleReq, seed, base64String, key, &tempCfg, userInput, width, height, isDallRequest)
	} else {
		log.Printf("A1111: Routing model '%s' to V4/Latest logic", realModelName)
		models.Nai4WithFormatAndSize(w, r, compatibleReq, seed, base64String, key, &tempCfg, userInput, nil, width, height, isDallRequest)
	}
}

// A1111Models 处理 A1111 兼容的模型列表请求
func A1111Models(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	workflowsDir := "comfyui-workflows" // 默认，如果未配置
	// 为了简化，这里我们只扫描目录里的 json 文件
	entries, _ := os.ReadDir(workflowsDir)

	var modelsList []map[string]interface{}
	
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			name := entry.Name()
			nameWithoutExt := name[:len(name)-5] // remove .json
			modelsList = append(modelsList, map[string]interface{}{
				"title":      nameWithoutExt,
				"model_name": nameWithoutExt,
				"hash":       "comfy",
				"sha256":     "comfyui_workflow",
				"filename":   name,
			})
		}
	}

	// 补充内置的模型列表
	builtInModels := []map[string]interface{}{
		{
			"title":      "nai-diffusion-3",
			"model_name": "nai-diffusion-3",
			"hash":       "nai3",
			"sha256":     "nai3",
			"filename":   "nai-diffusion-3",
		},
		{
			"title":      "nai-diffusion-4-full",
			"model_name": "nai-diffusion-4-full",
			"hash":       "nai4",
			"sha256":     "nai4",
			"filename":   "nai-diffusion-4-full",
		},
		{
			"title":      "nai-diffusion-furry-3",
			"model_name": "nai-diffusion-furry-3",
			"hash":       "furry3",
			"sha256":     "furry3",
			"filename":   "nai-diffusion-furry-3",
		},
		{
			"title":      "nai-diffusion-4-curated-preview",
			"model_name": "nai-diffusion-4-curated-preview",
			"hash":       "nai4cp",
			"sha256":     "nai4cp",
			"filename":   "nai-diffusion-4-curated-preview",
		},
		{
			"title":      "nai-diffusion-4-5-curated",
			"model_name": "nai-diffusion-4-5-curated",
			"hash":       "nai45c",
			"sha256":     "nai45c",
			"filename":   "nai-diffusion-4-5-curated",
		},
		{
			"title":      "nai-diffusion-4-5-full",
			"model_name": "nai-diffusion-4-5-full",
			"hash":       "nai45f",
			"sha256":     "nai45f",
			"filename":   "nai-diffusion-4-5-full",
		},
	}

	modelsList = append(modelsList, builtInModels...)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsList)
}

// A1111Progress 处理 A1111 兼容的进度查询请求
func A1111Progress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 因为此代理使用的是同步阻塞请求，无法获取 NovelAI 的实时生成进度，返回空闲状态的默认数据
	progress := map[string]interface{}{
		"progress":     0.0,
		"eta_relative": 0.0,
		"state": map[string]interface{}{
			"skipped":        false,
			"interrupted":    false,
			"job":            "",
			"job_count":      0,
			"job_timestamp":  "19700101000000",
			"job_no":         0,
			"sampling_step":  0,
			"sampling_steps": 0,
		},
		"current_image": nil,
		"textinfo":      nil,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}