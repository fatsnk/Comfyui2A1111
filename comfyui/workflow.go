package comfyui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Workflow 解析与参数映射

type TargetNode struct {
	NodeID    string `json:"node_id"`
	InputKey  string `json:"input_key"`
	ClassType string `json:"class_type"`
	Title     string `json:"title"`
	Score     int    `json:"score,omitempty"`
}

type NodeCandidateList struct {
	Autodetect      *TargetNode   `json:"autodetect"`
	AutodetectError string        `json:"autodetect_error,omitempty"`
	Candidates      []TargetNode  `json:"candidates"`
}

type WorkflowTargets struct {
	PositivePrompt NodeCandidateList `json:"positive_prompt"`
	NegativePrompt NodeCandidateList `json:"negative_prompt"`
	Image          NodeCandidateList `json:"image"`
	Width          NodeCandidateList `json:"width"`
	Height         NodeCandidateList `json:"height"`
	Steps          NodeCandidateList `json:"steps"`
	Seed           NodeCandidateList `json:"seed"`
	Cfg            NodeCandidateList `json:"cfg"`
	Sampler        NodeCandidateList `json:"sampler"`
	Scheduler      NodeCandidateList `json:"scheduler"`
}

type WorkflowParams struct {
	Version            int    `json:"version"`
	Kind               string `json:"kind"`
	PromptNode         string `json:"prompt_node"`
	NegativePromptNode string `json:"negative_prompt_node"`
	ImageNode          string `json:"image_node"`
	WidthNode          string `json:"width_node"`
	HeightNode         string `json:"height_node"`
	StepsNode          string `json:"steps_node"`
	SeedNode           string `json:"seed_node"`
	CfgNode            string `json:"cfg_node"`
	SamplerNode        string `json:"sampler_node"`
	SchedulerNode      string `json:"scheduler_node"`
}

func ExtractPromptAndExtra(obj map[string]interface{}) (map[string]interface{}, error) {
	if prompt, ok := obj["prompt"].(map[string]interface{}); ok {
		return prompt, nil
	}
	
	// 检查是否已经是 API 格式
	isApiFormat := false
	for _, v := range obj {
		if node, ok := v.(map[string]interface{}); ok {
			if _, hasClassType := node["class_type"]; hasClassType {
				isApiFormat = true
				break
			}
		}
	}
	
	if isApiFormat {
		return obj, nil
	}
	
	if _, hasNodes := obj["nodes"]; hasNodes {
		return nil, fmt.Errorf("UI workflow JSON detected (contains 'nodes'/'links'). Export 'API format' from ComfyUI and retry")
	}
	
	return nil, fmt.Errorf("Unrecognized workflow JSON format. Expected API prompt format")
}

func GetNodeTitle(node map[string]interface{}) string {
	if meta, ok := node["_meta"].(map[string]interface{}); ok {
		if title, ok := meta["title"].(string); ok {
			return title
		}
	}
	return ""
}

func AsStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func DetectKind(prompt map[string]interface{}) string {
	hasLoadImage := false
	hasSaveImage := false
	
	for _, v := range prompt {
		node, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		cls := strings.ToLower(AsStr(node["class_type"]))
		if strings.Contains(cls, "loadimage") {
			hasLoadImage = true
		}
		if strings.Contains(cls, "saveimage") {
			hasSaveImage = true
		}
	}
	
	if hasSaveImage && hasLoadImage {
		return "img2img"
	} else if hasSaveImage {
		return "txt2img"
	}
	return "unknown"
}

func FindTextPromptTargets(prompt map[string]interface{}) ([]TargetNode, []TargetNode) {
	var pos []TargetNode
	var neg []TargetNode
	var generic []TargetNode
	
	seenPos := make(map[string]bool)
	seenNeg := make(map[string]bool)
	seenGeneric := make(map[string]bool)
	
	for nodeID, v := range prompt {
		node, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		cls := AsStr(node["class_type"])
		title := GetNodeTitle(node)
		inputs, ok := node["inputs"].(map[string]interface{})
		if !ok {
			continue
		}
		
		var stringInputs []string
		for k, val := range inputs {
			if _, isStr := val.(string); isStr {
				stringInputs = append(stringInputs, k)
			}
		}
		
		if len(stringInputs) == 0 {
			continue
		}
		
		titleL := strings.ToLower(title)
		clsL := strings.ToLower(cls)
		isEncode := strings.Contains(clsL, "textencode")
		
		preferredKey := stringInputs[0]
		if _, hasText := inputs["text"].(string); hasText {
			preferredKey = "text"
		}
		
		target := TargetNode{
			NodeID: nodeID,
			InputKey: preferredKey,
			ClassType: cls,
			Title: title,
		}
		
		key := fmt.Sprintf("%s.%s", nodeID, preferredKey)
		
		if strings.Contains(titleL, "negative") || strings.Contains(titleL, "neg") {
			if !seenNeg[key] {
				neg = append(neg, target)
				seenNeg[key] = true
			}
		} else if strings.Contains(titleL, "positive") || strings.Contains(titleL, "pos") {
			if !seenPos[key] {
				pos = append(pos, target)
				seenPos[key] = true
			}
		} else if isEncode {
			if !seenGeneric[key] {
				generic = append(generic, target)
				seenGeneric[key] = true
			}
		}
	}
	
	if len(pos) == 0 {
		pos = append(pos, generic...)
	}
	if len(neg) == 0 {
		neg = append(neg, generic...)
	}
	
	return pos, neg
}

func FindLoadImageTargets(prompt map[string]interface{}) []TargetNode {
	var candidates []TargetNode
	
	for nodeID, v := range prompt {
		node, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		cls := AsStr(node["class_type"])
		if !strings.Contains(strings.ToLower(cls), "loadimage") {
			continue
		}
		inputs, ok := node["inputs"].(map[string]interface{})
		if !ok {
			continue
		}
		
		if _, hasImage := inputs["image"].(string); hasImage {
			candidates = append(candidates, TargetNode{
				NodeID: nodeID,
				InputKey: "image",
				ClassType: cls,
				Title: GetNodeTitle(node),
			})
			continue
		}
		
		for k, val := range inputs {
			if _, isStr := val.(string); isStr {
				candidates = append(candidates, TargetNode{
					NodeID: nodeID,
					InputKey: k,
					ClassType: cls,
					Title: GetNodeTitle(node),
				})
				break
			}
		}
	}
	
	return candidates
}

func FindStringTargets(prompt map[string]interface{}, paramType string) []TargetNode {
	var candidates []TargetNode
	
	exactKeys := map[string][]string{
		"sampler":   {"sampler_name", "sampler"},
		"scheduler": {"scheduler"},
	}
	
	semanticTokens := map[string][]string{
		"sampler":   {"sampler", "ksampler"},
		"scheduler": {"sampler", "ksampler", "scheduler"},
	}
	
	for nodeID, v := range prompt {
		node, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		cls := AsStr(node["class_type"])
		title := GetNodeTitle(node)
		inputs, ok := node["inputs"].(map[string]interface{})
		if !ok {
			continue
		}
		
		for inputKey, val := range inputs {
			// Check if string
			if _, isStr := val.(string); isStr {
				norm := strings.ToLower(strings.Map(func(r rune) rune {
					if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
						return r
					}
					return -1
				}, inputKey))
				
				score := 50
				
				// Exact match or alias
				for _, k := range exactKeys[paramType] {
					if norm == k {
						score += 20
						break
					}
				}
				
				// Semantic match
				titleL := strings.ToLower(title)
				clsL := strings.ToLower(cls)
				for _, token := range semanticTokens[paramType] {
					if strings.Contains(titleL, token) || strings.Contains(clsL, token) {
						score += 15
						break
					}
				}
				
				if score > 50 {
					candidates = append(candidates, TargetNode{
						NodeID: nodeID,
						InputKey: inputKey,
						ClassType: cls,
						Title: title,
						Score: score,
					})
				}
			}
		}
	}
	
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	
	return candidates
}

func FindNumericTargets(prompt map[string]interface{}, paramType string) []TargetNode {
	var candidates []TargetNode
	
	exactKeys := map[string][]string{
		"width":  {"width", "imagewidth", "latentwidth"},
		"height": {"height", "imageheight", "latentheight"},
		"steps":  {"steps", "numsteps"},
		"cfg":    {"cfg", "cfgscale", "guidance", "guidancescale"},
		"seed":   {"seed", "noiseseed", "randomseed"},
	}
	
	semanticTokens := map[string][]string{
		"width":  {"latent", "image", "size", "empty"},
		"height": {"latent", "image", "size", "empty"},
		"steps":  {"sampler", "ksampler"},
		"cfg":    {"sampler", "guidance", "ksampler"},
		"seed":   {"sampler", "noise", "ksampler"},
	}
	
	for nodeID, v := range prompt {
		node, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		cls := AsStr(node["class_type"])
		title := GetNodeTitle(node)
		inputs, ok := node["inputs"].(map[string]interface{})
		if !ok {
			continue
		}
		
		for inputKey, val := range inputs {
			// Check if numeric
			switch val.(type) {
			case float64, int, int64:
				norm := strings.ToLower(strings.Map(func(r rune) rune {
					if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
						return r
					}
					return -1
				}, inputKey))
				
				score := 50
				
				// Exact match or alias
				for _, k := range exactKeys[paramType] {
					if norm == k {
						score += 20
						break
					}
				}
				
				// Semantic match
				titleL := strings.ToLower(title)
				clsL := strings.ToLower(cls)
				for _, token := range semanticTokens[paramType] {
					if strings.Contains(titleL, token) || strings.Contains(clsL, token) {
						score += 15
						break
					}
				}
				
				if score > 50 {
					candidates = append(candidates, TargetNode{
						NodeID: nodeID,
						InputKey: inputKey,
						ClassType: cls,
						Title: title,
						Score: score,
					})
				}
			}
		}
	}
	
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	
	return candidates
}

func PickUniqueTarget(kind string, candidates []TargetNode) (*TargetNode, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("No %s prompt text node found in workflow", kind)
	}
	
	for i := range candidates {
		score := 0
		c := &candidates[i]
		
		titleL := strings.ToLower(c.Title)
		clsL := strings.ToLower(c.ClassType)
		
		if c.InputKey == "text" {
			score += 10
		}
		if strings.Contains(clsL, "textencode") {
			score += 5
		}
		
		if kind == "positive" {
			if strings.Contains(titleL, "positive") || strings.Contains(titleL, "pos") {
				score += 100
			}
		} else {
			if strings.Contains(titleL, "negative") || strings.Contains(titleL, "neg") {
				score += 100
			}
		}
		c.Score = score
	}
	
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	
	bestScore := candidates[0].Score
	var best []TargetNode
	for _, c := range candidates {
		if c.Score == bestScore {
			best = append(best, c)
		}
	}
	
	if len(best) == 1 {
		return &best[0], nil
	}
	
	return &best[0], fmt.Errorf("Ambiguous %s prompt node. Candidates have same score", kind)
}

func PickUniqueLoadImageTarget(candidates []TargetNode) (*TargetNode, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("No LoadImage node found in workflow")
	}
	if len(candidates) == 1 {
		return &candidates[0], nil
	}
	
	for i := range candidates {
		score := 0
		c := &candidates[i]
		
		titleL := strings.ToLower(c.Title)
		if strings.Contains(titleL, "load") {
			score += 10
		}
		if c.InputKey == "image" {
			score += 5
		}
		c.Score = score
	}
	
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	
	return &candidates[0], nil
}

func LoadWorkflowParams(workflowsDir, workflowName string) (*WorkflowParams, error) {
	sidecarDir := filepath.Join(workflowsDir, ".comfyui2api")
	ext := filepath.Ext(workflowName)
	nameWithoutExt := workflowName[:len(workflowName)-len(ext)]
	sidecarPath := filepath.Join(sidecarDir, nameWithoutExt+".params.json")
	
	if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
		return nil, nil
	}
	
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return nil, err
	}
	
	var params WorkflowParams
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, err
	}
	
	return &params, nil
}

func SaveWorkflowParams(workflowsDir, workflowName string, params *WorkflowParams) error {
	sidecarDir := filepath.Join(workflowsDir, ".comfyui2api")
	if err := os.MkdirAll(sidecarDir, 0755); err != nil {
		return err
	}
	
	ext := filepath.Ext(workflowName)
	nameWithoutExt := workflowName[:len(workflowName)-len(ext)]
	sidecarPath := filepath.Join(sidecarDir, nameWithoutExt+".params.json")
	
	data, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(sidecarPath, data, 0644)
}

func ApplyOverrides(prompt map[string]interface{}, params *WorkflowParams, req map[string]interface{}) error {
	applyValue := func(nodeRef string, val interface{}) error {
		if nodeRef == "" {
			return nil
		}
		parts := strings.SplitN(nodeRef, ".", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid node ref: %s", nodeRef)
		}
		nodeID, inputKey := parts[0], parts[1]
		
		node, ok := prompt[nodeID].(map[string]interface{})
		if !ok {
			return fmt.Errorf("node %s not found", nodeID)
		}
		
		inputs, ok := node["inputs"].(map[string]interface{})
		if !ok {
			inputs = make(map[string]interface{})
			node["inputs"] = inputs
		}
		
		inputs[inputKey] = val
		return nil
	}
	
	if val, ok := req["prompt"]; ok && val != "" && params.PromptNode != "" {
		_ = applyValue(params.PromptNode, val)
	}
	if val, ok := req["negative_prompt"]; ok && val != "" && params.NegativePromptNode != "" {
		_ = applyValue(params.NegativePromptNode, val)
	}
	if val, ok := req["image"]; ok && val != "" && params.ImageNode != "" {
		_ = applyValue(params.ImageNode, val)
	}
	if val, ok := req["width"]; ok && val != 0 && params.WidthNode != "" {
		_ = applyValue(params.WidthNode, val)
	}
	if val, ok := req["height"]; ok && val != 0 && params.HeightNode != "" {
		_ = applyValue(params.HeightNode, val)
	}
	if val, ok := req["steps"]; ok && val != 0 && params.StepsNode != "" {
		_ = applyValue(params.StepsNode, val)
	}
	if val, ok := req["seed"]; ok && val != 0 && params.SeedNode != "" {
		_ = applyValue(params.SeedNode, val)
	}
	if val, ok := req["cfg_scale"]; ok && val != 0 && params.CfgNode != "" {
		_ = applyValue(params.CfgNode, val)
	}
	if val, ok := req["sampler_name"]; ok && val != "" && params.SamplerNode != "" {
		_ = applyValue(params.SamplerNode, val)
	}
	if val, ok := req["scheduler"]; ok && val != "" && params.SchedulerNode != "" {
		_ = applyValue(params.SchedulerNode, val)
	}
	
	return nil
}

func GetWorkflowTargets(obj map[string]interface{}) (*WorkflowTargets, error) {
	prompt, err := ExtractPromptAndExtra(obj)
	if err != nil {
		return nil, err
	}
	
	pos, neg := FindTextPromptTargets(prompt)
	img := FindLoadImageTargets(prompt)
	
	width := FindNumericTargets(prompt, "width")
	height := FindNumericTargets(prompt, "height")
	steps := FindNumericTargets(prompt, "steps")
	seed := FindNumericTargets(prompt, "seed")
	cfg := FindNumericTargets(prompt, "cfg")
	sampler := FindStringTargets(prompt, "sampler")
	scheduler := FindStringTargets(prompt, "scheduler")
	
	targets := &WorkflowTargets{}
	
	// Pos
	targets.PositivePrompt.Candidates = pos
	if auto, err := PickUniqueTarget("positive", pos); err == nil {
		targets.PositivePrompt.Autodetect = auto
	} else {
		targets.PositivePrompt.AutodetectError = err.Error()
		if len(pos) > 0 {
			targets.PositivePrompt.Autodetect = &pos[0] // fallback
		}
	}
	
	// Neg
	targets.NegativePrompt.Candidates = neg
	if auto, err := PickUniqueTarget("negative", neg); err == nil {
		targets.NegativePrompt.Autodetect = auto
	} else {
		targets.NegativePrompt.AutodetectError = err.Error()
		if len(neg) > 0 {
			targets.NegativePrompt.Autodetect = &neg[0]
		}
	}
	
	// Img
	targets.Image.Candidates = img
	if auto, err := PickUniqueLoadImageTarget(img); err == nil {
		targets.Image.Autodetect = auto
	} else {
		targets.Image.AutodetectError = err.Error()
		if len(img) > 0 {
			targets.Image.Autodetect = &img[0]
		}
	}
	
	// Numeric
	setNumericTarget := func(list *NodeCandidateList, cands []TargetNode) {
		list.Candidates = cands
		if len(cands) > 0 {
			list.Autodetect = &cands[0]
		}
	}
	
	setNumericTarget(&targets.Width, width)
	setNumericTarget(&targets.Height, height)
	setNumericTarget(&targets.Steps, steps)
	setNumericTarget(&targets.Seed, seed)
	setNumericTarget(&targets.Cfg, cfg)
	setNumericTarget(&targets.Sampler, sampler)
	setNumericTarget(&targets.Scheduler, scheduler)
	
	return targets, nil
}