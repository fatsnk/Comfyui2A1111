package models

import (
	"log"
	"math/rand"
	"novel-api/config"
	"strings"
	"sync"
	"time"
)

// ProviderState 维护每个 Provider 的状态
type ProviderState struct {
	mu         sync.Mutex
	RecentKeys []string // 记录最近使用的 Key，FIFO 队列
}

var (
	providerStates = make(map[string]*ProviderState)
	stateMu        sync.Mutex
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// getProviderState 获取或创建 Provider 的状态
func getProviderState(providerName string) *ProviderState {
	stateMu.Lock()
	defer stateMu.Unlock()

	if state, exists := providerStates[providerName]; exists {
		return state
	}

	newState := &ProviderState{
		RecentKeys: make([]string, 0),
	}
	providerStates[providerName] = newState
	return newState
}

// GetNextKey 获取下一个可用的 Key (负载均衡算法)
func GetNextKey(provider config.Provider) string {
	if len(provider.Keys) == 0 {
		return ""
	}

	// 1. 如果只有一个 Key，直接返回
	if len(provider.Keys) == 1 {
		return provider.Keys[0]
	}

	state := getProviderState(provider.Name)
	state.mu.Lock()
	defer state.mu.Unlock()

	// 2. 计算需要排除的数量 N
	var excludeCount int
	if len(provider.Keys) > 3 {
		excludeCount = 3
	} else {
		excludeCount = 1
	}

	// 3. 构建可选池 (Available Keys)
	availableKeys := make([]string, 0)
	for _, key := range provider.Keys {
		isRecent := false
		for _, recentKey := range state.RecentKeys {
			if key == recentKey {
				isRecent = true
				break
			}
		}
		if !isRecent {
			availableKeys = append(availableKeys, key)
		}
	}

	// 如果可选池为空（理论上不应该发生，除非配置被动态修改导致 keys 减少），则清空历史记录并使用所有 keys
	if len(availableKeys) == 0 {
		log.Printf("Warning: Available keys pool is empty for provider %s, resetting history.", provider.Name)
		state.RecentKeys = make([]string, 0)
		availableKeys = provider.Keys
	}

	// 4. 随机选择
	selectedIndex := rand.Intn(len(availableKeys))
	selectedKey := availableKeys[selectedIndex]

	// 5. 更新 RecentKeys 队列
	state.RecentKeys = append(state.RecentKeys, selectedKey)
	if len(state.RecentKeys) > excludeCount {
		// 移除最老的记录 (FIFO)
		state.RecentKeys = state.RecentKeys[1:]
	}

	return selectedKey
}

// RouteModel 解析模型名称，返回目标 BaseURL, Key 和真实的 ModelName
func RouteModel(cfg *config.Config, originalModelName string) (baseURL string, key string, realModelName string) {
	// 默认值
	baseURL = cfg.NovelAI.BaseURL
	key = cfg.NovelAI.Key
	realModelName = originalModelName

	// 尝试拆分 providerName.modelName
	parts := strings.SplitN(originalModelName, ".", 2)
	
	var targetProvider *config.Provider

	if len(parts) == 2 {
		// 找到了前缀
		providerName := parts[0]
		realModelName = parts[1]

		// 查找匹配的 Provider
		for _, p := range cfg.NovelAI.Providers {
			if p.Name == providerName {
				targetProvider = &p
				break
			}
		}
	}

	// 如果没有找到匹配的 Provider，并且全局的 BaseURL 和 Key 都未配置，但配置了 Providers 列表，才默认使用第一个
	if targetProvider == nil && cfg.NovelAI.BaseURL == "" && cfg.NovelAI.Key == "" && len(cfg.NovelAI.Providers) > 0 {
		targetProvider = &cfg.NovelAI.Providers[0]
		// 注意：这里不修改 realModelName，因为它可能本来就没有前缀
	}

	// 如果找到了 Provider，则使用其配置
	if targetProvider != nil {
		if targetProvider.BaseURL != "" {
			baseURL = targetProvider.BaseURL
		}
		key = GetNextKey(*targetProvider)
	}

	// 打印路由日志
	log.Printf("[Router] Model: %s -> RealModel: %s, BaseURL: %s, Key: %s", originalModelName, realModelName, baseURL, maskKey(key))

	return baseURL, key, realModelName
}

// maskKey 隐藏 Key 的中间部分，只显示前三位和后三位
func maskKey(key string) string {
	if len(key) == 0 {
		return ""
	}
	if len(key) <= 9 {
		return "***" + key[len(key)-1:]
	}
	return key[:3] + "***" + key[len(key)-3:]
}