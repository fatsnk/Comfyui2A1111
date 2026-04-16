package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"novel-api/config"
	"novel-api/logs"
	"novel-api/models"
	"novel-api/upload"
	"regexp"
	"strings"
	"time"
)

// ChatRequest 定义请求结构体
type ChatRequest struct {
	Authorization string    `json:"Authorization"`
	Messages      []Message `json:"messages"`
	Model         string    `json:"model"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// 提取链接的函数
func extractLinks(userInput string) []string {
	re := regexp.MustCompile(`https?://[^\s]+`)
	matches := re.FindAllString(userInput, -1)
	return matches
}

func Completions(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	// 如果是 OPTIONS 请求,直接返回 200 OK
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	// 1. 获取 Authorization 请求头的值
	authHeader := r.Header.Get("Authorization")
	authHeader = strings.TrimPrefix(authHeader, "Bearer ")

	// 本地接口鉴权：使用配置中的 A1111Path (即安全路径) 作为密码
	if cfg.NovelAI.A1111Path != "" && authHeader != cfg.NovelAI.A1111Path {
		log.Printf("Completions API Unauthorized: Expected %s, got %s", cfg.NovelAI.A1111Path, authHeader)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fmt.Println("Completions API 秘钥鉴权通过。传入 Token：", authHeader)
	// 覆盖为发往 NovelAI 的真实 Key
	authHeader = cfg.NovelAI.Key
	// 解析请求体
	var req config.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode request body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	// 获取最后一条用户输入
	var userInput string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userInput = req.Messages[i].Content
			log.Printf("User input found: %s", userInput)
			break
		}
	}

	// 如果启用翻译，则翻译用户输入
	log.Printf("[Completions] Translation.Enable value: %v (URL: %s, Model: %s)", cfg.Translation.Enable, cfg.Translation.URL, cfg.Translation.Model)
	if cfg.Translation.Enable {
		translatedInput, err := TranslateText(userInput, cfg)
		if err != nil {
			log.Printf("Translation failed, using original text: %v", err)
		} else {
			log.Printf("Translation enabled, translated: %s -> %s", userInput, translatedInput)
			userInput = translatedInput
		}
	} else {
		log.Printf("[Completions] Translation is disabled, skipping translation")
	}

	// 提取用户输入中的链接
	imageURL := extractLinks(userInput)
	var base64String string
	if len(imageURL) > 0 {
		// 选择第一个提取到的链接
		imageURLS := imageURL[0]
		// 解析图片为bash
		base64String, _ = ImageURLToBase64(imageURLS)
		//if err != nil {
		//	log.Fatalf("Error: %v", err)
		//}
	}

	// 调用翻译,翻译为专用提示词

	// 生成一个随机种子
	rand.Seed(time.Now().UnixNano()) // 使用当前时间的纳秒数作为随机数生成器的种子
	randomSeed := rand.Intn(1000000) // 生成一个0到999999之间的随机数

	// 确定路由
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
		log.Printf("[Completions] Routing to ComfyUI for model: %s", req.Model)
		width := cfg.Parameters.Width
		height := cfg.Parameters.Height
		
		images, err := models.GenerateWithComfyUI(req.Model, cfg, userInput, base64String, width, height, randomSeed)
		if err == nil && len(images) > 0 {
			timestamp := time.Now().Unix()
			
			// 解码 base64
			imgData, decodeErr := base64.StdEncoding.DecodeString(images[0])
			var publicLink string
			
			if decodeErr == nil {
				imageName := fmt.Sprintf("comfy_%d.png", timestamp)
				
				// 调用通用上传函数
				response, uploadErr := upload.UploadFile(imgData, imageName, cfg)
				if uploadErr != nil {
					log.Printf("图片上传失败: %v", uploadErr)
					publicLink = fmt.Sprintf("error: 上传失败 - %s", imageName)
					
					// 记录失败日志
					logs.LogImage(logs.ImageLog{
						Model:    req.Model,
						Prompt:   userInput,
						ImageURL: "",
						UserIP:   r.RemoteAddr,
						Status:   "failed",
						Error:    fmt.Sprintf("上传失败: %v", uploadErr),
					})
				} else {
					log.Printf("图片上传成功: %s", response.Data.URL)
					publicLink = fmt.Sprintf("![%s](%s)", imageName, response.Data.URL)
					
					// 记录成功日志
					logs.LogImage(logs.ImageLog{
						Model:    req.Model,
						Prompt:   userInput,
						ImageURL: response.Data.URL,
						UserIP:   r.RemoteAddr,
						Status:   "success",
					})
				}
			} else {
				// 如果解码失败，回退到 data URI
				contentStr := fmt.Sprintf("![generated image](data:image/png;base64,%s)", images[0])
				publicLink = contentStr
			}
			
			// 修正 JSON 格式，确保 content 字段的值被正确转义
			contentBytes, _ := json.Marshal(publicLink)
			
			sseResponse := fmt.Sprintf(
				"data: {\"id\":\"%s\",\"object\":\"chat.completion.chunk\",\"created\":%d,\"model\":\"%s\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%s},\"logprobs\":null,\"finish_reason\":null}]}\n\n",
				"chatcmpl-"+fmt.Sprintf("%d", timestamp),
				timestamp,
				req.Model,
				string(contentBytes),
			)

			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte(sseResponse))
			w.Write([]byte("event: end\n\n"))
			w.(http.Flusher).Flush()
			return
		}
		log.Printf("[Completions] ComfyUI generation failed, falling back to NAI: %v", err)
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
	
	// 更新 req 中的模型名称为去掉 provider 前缀的真实模型名
	req.Model = realModelName

	switch realModelName {
	case "nai-diffusion-3", "nai-diffusion-furry-3":
		models.Nai3(w, r, req, randomSeed, base64String, authHeader, cfg, userInput)
	case "nai-diffusion-4-full", "nai-diffusion-4-curated-preview", "nai-diffusion-4-5-curated", "nai-diffusion-4-5-full":
		models.Nai4(w, r, req, randomSeed, base64String, authHeader, cfg, userInput, nil)
	default:
		if strings.Contains(realModelName, "-3") {
			models.Nai3(w, r, req, randomSeed, base64String, authHeader, cfg, userInput)
		} else {
			models.Nai4(w, r, req, randomSeed, base64String, authHeader, cfg, userInput, nil)
		}
	}
}
