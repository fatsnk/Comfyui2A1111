package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	//"net/http"
	"novel-api/comfyui"
	"novel-api/config"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func GenerateWithComfyUI(
	modelName string,
	cfg *config.Config,
	userInput string,
	base64String string,
	width int,
	height int,
	randomSeed int,
) ([]string, error) {
	workflowsDir := cfg.ComfyUI.WorkflowsDir
	if workflowsDir == "" {
		workflowsDir = "comfyui-workflows"
	}

	workflowPath := filepath.Join(workflowsDir, modelName)
	if !strings.HasSuffix(workflowPath, ".json") {
		workflowPath += ".json"
	}

	// 如果指定的工作流不存在，则回退到目录下的第一个工作流
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		found := false
		if entries, readErr := os.ReadDir(workflowsDir); readErr == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
					modelName = entry.Name()[:len(entry.Name())-5]
					workflowPath = filepath.Join(workflowsDir, entry.Name())
					found = true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("workflow not found: %s (and no fallback workflow available)", modelName)
		}
	}

	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow: %v", err)
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("invalid workflow JSON: %v", err)
	}

	prompt, err := comfyui.ExtractPromptAndExtra(obj)
	if err != nil {
		return nil, err
	}

	params, err := comfyui.LoadWorkflowParams(workflowsDir, modelName)
	if err != nil {
		log.Printf("Warning: failed to load params for %s: %v", modelName, err)
	}
	
	if params == nil {
		// Use auto-detection if no params defined
		targets, err := comfyui.GetWorkflowTargets(obj)
		if err == nil {
			params = &comfyui.WorkflowParams{
				Version: 1,
				Kind:    "auto",
			}
			if targets.PositivePrompt.Autodetect != nil {
				params.PromptNode = fmt.Sprintf("%s.%s", targets.PositivePrompt.Autodetect.NodeID, targets.PositivePrompt.Autodetect.InputKey)
			}
			if targets.Image.Autodetect != nil {
				params.ImageNode = fmt.Sprintf("%s.%s", targets.Image.Autodetect.NodeID, targets.Image.Autodetect.InputKey)
			}
			if targets.Width.Autodetect != nil {
				params.WidthNode = fmt.Sprintf("%s.%s", targets.Width.Autodetect.NodeID, targets.Width.Autodetect.InputKey)
			}
			if targets.Height.Autodetect != nil {
				params.HeightNode = fmt.Sprintf("%s.%s", targets.Height.Autodetect.NodeID, targets.Height.Autodetect.InputKey)
			}
			if targets.Steps.Autodetect != nil {
				params.StepsNode = fmt.Sprintf("%s.%s", targets.Steps.Autodetect.NodeID, targets.Steps.Autodetect.InputKey)
			}
			if targets.Seed.Autodetect != nil {
				params.SeedNode = fmt.Sprintf("%s.%s", targets.Seed.Autodetect.NodeID, targets.Seed.Autodetect.InputKey)
			}
		}
	}

	client := comfyui.NewClient(cfg.ComfyUI.BaseURL)
	
	// Upload image if provided
	imageFilename := ""
	if base64String != "" && params != nil && params.ImageNode != "" {
		imgData, err := base64.StdEncoding.DecodeString(base64String)
		if err == nil {
			uploadRes, err := client.UploadImage(imgData, fmt.Sprintf("upload_%s.png", uuid.New().String()))
			if err == nil {
				if name, ok := uploadRes["name"].(string); ok {
					subfolder, _ := uploadRes["subfolder"].(string)
					if subfolder != "" {
						imageFilename = subfolder + "/" + name
					} else {
						imageFilename = name
					}
				}
			} else {
				log.Printf("ComfyUI image upload failed: %v", err)
			}
		}
	}

	overrideReq := map[string]interface{}{
		"prompt": userInput,
		"width":  width,
		"height": height,
		"seed":   randomSeed,
	}
	
	if imageFilename != "" {
		overrideReq["image"] = imageFilename
	}

	if params != nil {
		if err := comfyui.ApplyOverrides(prompt, params, overrideReq); err != nil {
			log.Printf("Failed to apply overrides: %v", err)
		}
	}

	clientID := uuid.New().String()
	res, err := client.QueuePrompt(prompt, clientID)
	if err != nil {
		return nil, fmt.Errorf("ComfyUI queue failed: %v", err)
	}

	history, err := client.WaitForCompletion(res.PromptID, clientID, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("ComfyUI wait failed: %v", err)
	}

	// Extract output images
	var images []string
	if outputs, ok := history["outputs"].(map[string]interface{}); ok {
		for _, nodeOutput := range outputs {
			if noMap, ok := nodeOutput.(map[string]interface{}); ok {
				if imgs, ok := noMap["images"].([]interface{}); ok {
					for _, img := range imgs {
						if imgMap, ok := img.(map[string]interface{}); ok {
							filename, _ := imgMap["filename"].(string)
							subfolder, _ := imgMap["subfolder"].(string)
							folderType, _ := imgMap["type"].(string)
							
							if filename != "" {
								imgData, err := client.DownloadImage(filename, subfolder, folderType)
								if err == nil {
									b64 := base64.StdEncoding.EncodeToString(imgData)
									images = append(images, b64)
								}
							}
						}
					}
				}
			}
		}
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("ComfyUI generated no images")
	}

	return images, nil
}