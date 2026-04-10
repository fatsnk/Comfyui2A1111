package config

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

type Config struct {
	// 启动端口号变量
	Server struct {
		Addr string `mapstructure:"addr" yaml:"addr"`
	} `mapstructure:"server"`

	// 日志管理密码
	LogsAdmin struct {
		Password string `mapstructure:"password" yaml:"password"`
	} `mapstructure:"logs_admin" yaml:"logs_admin"`

	// 存储桶选择器配置
	COS struct {
		Bucket string `mapstructure:"backet" yaml:"backet"` // 注意这里保持和.env文件中的拼写一致
	} `mapstructure:"cos" yaml:"cos"`

	// 翻译服务变量
	Translation struct {
		URL    string `mapstructure:"url" yaml:"url"`
		Key    string `mapstructure:"key" yaml:"key"`
		Model  string `mapstructure:"model" yaml:"model"`
		Role   string `mapstructure:"role" yaml:"role"`
		Enable bool   `mapstructure:"enable" yaml:"enable"`
	} `mapstructure:"translation" yaml:"translation"`

	// 腾讯云COS配置变量
	TencentCOS struct {
		SecretID  string `mapstructure:"secret_id" yaml:"secret_id"`
		SecretKey string `mapstructure:"secret_key" yaml:"secret_key"`
		Region    string `mapstructure:"region" yaml:"region"`
		Bucket    string `mapstructure:"bucket" yaml:"bucket"`
		BaseURL   string `mapstructure:"base_url" yaml:"base_url"`
	} `mapstructure:"tencent_cos" yaml:"tencent_cos"`

	// Minio配置变量
	Minio struct {
		Endpoint        string `mapstructure:"endpoint" yaml:"endpoint"`
		AccessKeyID     string `mapstructure:"access_key_id" yaml:"access_key_id"`
		SecretAccessKey string `mapstructure:"secret_access_key" yaml:"secret_access_key"`
		BucketName      string `mapstructure:"bucket_name" yaml:"bucket_name"`
		UseSSL          bool   `mapstructure:"use_ssl" yaml:"use_ssl"`
		BaseURL         string `mapstructure:"base_url" yaml:"base_url"`
	} `mapstructure:"minio" yaml:"minio"`

	// Alist配置变量
	Alist struct {
		BaseURL  string `mapstructure:"base_url" yaml:"base_url"`
		Token    string `mapstructure:"token" yaml:"token"`
		Path     string `mapstructure:"path" yaml:"path"`
		Username string `mapstructure:"username" yaml:"username"`
		Password string `mapstructure:"password" yaml:"password"`
	} `mapstructure:"alist" yaml:"alist"`

	// Lsky图床配置变量
	Lsky struct {
		BaseURL    string `mapstructure:"base_url" yaml:"base_url"`
		Token      string `mapstructure:"token" yaml:"token"`
		StrategyID int    `mapstructure:"strategy_id" yaml:"strategy_id"` // 存储策略ID，可选
	} `mapstructure:"lsky" yaml:"lsky"`

	// 本地存储配置变量
	Local struct {
		Path    string `mapstructure:"path" yaml:"path"`
		BaseURL string `mapstructure:"base_url" yaml:"base_url"`
	} `mapstructure:"local" yaml:"local"`

	// 图片质量变量
	Parameters struct {
		ParamsVersion                      int     `mapstructure:"params_version" yaml:"params_version"`
		Width                              int     `mapstructure:"width" yaml:"width"`
		Height                             int     `mapstructure:"height" yaml:"height"`
		Scale                              float64 `mapstructure:"scale" yaml:"scale"`
		Sampler                            string  `mapstructure:"sampler" yaml:"sampler"`
		Steps                              int     `mapstructure:"steps" yaml:"steps"`
		NSamples                           int     `mapstructure:"n_samples" yaml:"n_samples"`
		UCPreset                           int     `mapstructure:"ucPreset" yaml:"ucPreset"`
		QualityToggle                      bool    `mapstructure:"qualityToggle" yaml:"qualityToggle"`
		SM                                 bool    `mapstructure:"sm" yaml:"sm"`
		SMDyn                              bool    `mapstructure:"sm_dyn" yaml:"sm_dyn"`
		DynamicThresholding                bool    `mapstructure:"dynamic_thresholding" yaml:"dynamic_thresholding"`
		ControlnetStrength                 int     `mapstructure:"controlnet_strength" yaml:"controlnet_strength"`
		Legacy                             bool    `mapstructure:"legacy" yaml:"legacy"`
		AddOriginalImage                   bool    `mapstructure:"add_original_image" yaml:"add_original_image"`
		CFGRescale                         int     `mapstructure:"cfg_rescale" yaml:"cfg_rescale"`
		NoiseSchedule                      string  `mapstructure:"noise_schedule" yaml:"noise_schedule"`
		LegacyV3Extend                     bool    `mapstructure:"legacy_v3_extend" yaml:"legacy_v3_extend"`
		SkipCFGAboveSigma                  int     `mapstructure:"skip_cfg_above_sigma" yaml:"skip_cfg_above_sigma"`
		DeliberateEulerAncestralBug        bool    `mapstructure:"deliberate_euler_ancestral_bug" yaml:"deliberate_euler_ancestral_bug"`
		PreferBrownian                     bool    `mapstructure:"prefer_brownian" yaml:"prefer_brownian"`
		CustomAntiWords                    string  `mapstructure:"custom_anti_words" yaml:"custom_anti_words"`
		AutoSmea                           bool    `mapstructure:"autoSmea" yaml:"autoSmea"`
		UseCoords                          bool    `mapstructure:"use_coords" yaml:"use_coords"`
		LegacyUC                           bool    `mapstructure:"legacy_uc" yaml:"legacy_uc"`
		NormalizeReferenceStrengthMultiple bool    `mapstructure:"normalize_reference_strength_multiple" yaml:"normalize_reference_strength_multiple"`
		InpaintImg2ImgStrength             int     `mapstructure:"inpaintImg2ImgStrength" yaml:"inpaintImg2ImgStrength"`
		UseNewSharedTrial                  bool    `mapstructure:"use_new_shared_trial" yaml:"use_new_shared_trial"`
	} `mapstructure:"parameters" yaml:"parameters"`

	// NovelAI 全局配置
	NovelAI struct {
		BaseURL     string `mapstructure:"base_url" yaml:"base_url"`
		Key         string `mapstructure:"key" yaml:"key"`
		A1111Path   string `mapstructure:"a1111_path" yaml:"a1111_path"`
		A1111NoSave bool   `mapstructure:"a1111_no_save" yaml:"a1111_no_save"`
	} `mapstructure:"novel_ai" yaml:"novel_ai"`

	// ComfyUI 配置
	ComfyUI struct {
		BaseURL      string `mapstructure:"base_url" yaml:"base_url"`           // ComfyUI的地址，如 http://127.0.0.1:8188
		WorkflowsDir string `mapstructure:"workflows_dir" yaml:"workflows_dir"` // 工作流存放目录，默认 comfyui-workflows
		AutoRoute    bool   `mapstructure:"auto_route" yaml:"auto_route"`       // 是否开启自动路由
		DefaultRoute string `mapstructure:"default_route" yaml:"default_route"` // 关闭自动路由时的默认去向： "novelai" 或 "comfyui"
	} `mapstructure:"comfyui" yaml:"comfyui"`
}
