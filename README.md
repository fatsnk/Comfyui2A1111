# Comfyui2A1111 - AI 图像生成api中转

*参考项目：https://github.com/Einzieg/Comfyui2api -MIT、https://github.com/xiaopalu-max/novel-api-go?tab=readme-ov-file#-%E8%AE%B8%E5%8F%AF%E8%AF%81 -MIT*

基于 Go 语言开发的 AI 图像生成 API 服务，支持 NovelAI Diffusion 模型（v3 和 v4）、comfyui，
支持将本地 ComfyUI 工作流和云服务商NovelAI的 Diffusion模型api包装为标准的 OpenAI 图像生成接口Dall-E（`/v1/images/generations`） 和聊天补全接口 （`/v1/chat/completions`）以及 stable diffusion webui A1111 兼容接口。提供直观的 Web UI 用于上传工作流 JSON、自动识别并映射提示词/参数节点，支持在 NovelAI 和本地 ComfyUI 之间智能路由请求。

```
简要使用说明
0.前往go官网https://go.dev/dl/，下载并安装适合你的机器的go。（假设你没有安装过）
1.下载源码，并使用cmd进入项目目录。
2.复制.env.example并改名为.env,修改关键参数：
  password: abcd1234 # 这个管理员密码必须改
#  ...
  local:
    path: ./images
    base_url: http://192.168.1.3:8029/images # 这个local中的base_url必须改成你自己机器的局域网ip或公网ip（如果有）或穿透后的域名（如果你设置了内网穿透）
#  ...
  novel_ai: # 这部分是远程novelAI配置，可在web界面设置，这里也可以提前设置。
    base_url: https://xxx.com/ai/generate-image
    key: sk-123
    a1111_path: mytestpath # 重要！必须设置，用于A1111的请求路径，同时也作为/v1/images/generations（DALL-E）和chat/completions（openai兼容格式）的api key!比如，默认的服务地址是http://127.0.0.1:8029，那么你的客户端填写的stable diffusion地址就是http://127.0.0.1:8029/mytestpath，如果使用openai兼容格式，那么apikey就要填写mytestpath，所以一定要修改！
    a1111_no_save: true
  comfyui: # 这里是本地comfyui配置，可在web界面中设置。
    base_url: http://192.168.1.3:8188 # 你的comfyui的地址。comfyui需要开启监听。
    workflows_dir: ""
    auto_route: false
    default_route: novelai

3.先go mod tidy，然后go run main.go，也可以先编译再运行，例如go build -o comfyui2a1111.exe。如果安装依赖时出现卡死或网络报错，请使用代理。
4.go run main.go成功后，控制台会打印类似以下信息：
  Config loaded successfully from .env file
  Logger initialized successfully
  Starting server on :  8029
  日志查询页面: http://localhost:8029/logs
  默认管理密码: abcd1234

用管理员密码登录logs页面即可。如果上面的环境变量中你没有配置novel_ai、A1111和comfyui的内容，请登录后在右上角的设置中配置。
5.配置comfyui工作流。在logs页面中上传你的工作流，然后点击参数配置按钮，在界面上配置节点的功能（选择正面/负面提示词、生成参数等对应的节点）。
配置好后，在右上角设置中选择始终走本地（comfyui）或始终走云端（novelAI），然后就可用使用了。
6.一些说明
/v1/images/generations（DALL-E）兼容接口必须使用非流式解析，注意apikey填写a1111_path的值（即设置页面中的“A1111 接口安全子路径”），stable diffusion（A1111）兼容接口地址则为“http://你的服务所部署的机器的ip:你设置的端口号/你填写的子路径”，例如http://192.168.1.3:8029/mytestpath

```

* 如果你是在pc使用，可直接下载[releases](https://github.com/fatsnk/Comfyui2A1111/releases) 中的可执行文件，放在和你的环境变量文件.env同一个目录下运行。请先查看上面的简要说明中环境变量的配置，配置好.env文件再运行。


> 💡 **提示**：如果是部署在 Railway 或 Render 等需要配置扁平化环境变量的云平台，可以直接参考使用 [RAILWAY_ENV_EXAMPLE.txt](RAILWAY_ENV_EXAMPLE.txt) 中提供的环境变量格式，复制粘贴之后修改配置。

## 功能

### AI 图像生成
- **支持多个 NovelAI 模型**：
  - `nai-diffusion-3` - NAI Diffusion 3.0
  - `nai-diffusion-furry-3` - NAI Diffusion 3.0 兽人版
  - `nai-diffusion-4-full` - NAI Diffusion 4.0 完整版
  - `nai-diffusion-4-curated-preview` - NAI Diffusion 4.0 精选预览版
  - `nai-diffusion-4-5-curated` - NAI Diffusion 4.5 精选版
  - `nai-diffusion-4-5-full` - NAI Diffusion 4.5 完整版

### 智能翻译
- **AI 翻译服务**：自动将中文提示词翻译为英文
- **提示词优化**：内置 NovelAI 专用提示词优化系统
- **可配置开关**：支持启用/禁用翻译功能

### 云存储支持
- **腾讯云 COS**：支持腾讯云对象存储
- **MinIO**：支持自建或第三方 MinIO 服务
- **Alist**：支持 Alist 网盘聚合服务

### 可配置
- **参数可调**：支持图像尺寸、采样器、步数等全参数配置
- **YAML 配置**：使用 `env` 文件进行集中配置管理
- **多环境支持**：易于部署到不同环境

## 🛠️ 快速开始

### 系统要求
- Go 1.23.0 或更高版本

### 1️⃣ 安装步骤

**克隆项目**
```bash
git clone <repository-url>
cd novel-api-go
mv .env.example .env
```

**安装依赖**
```bash
go mod tidy
```

**配置服务**

复制并编辑 `env` 配置文件（参考 `.env.example`）：

```yaml
# 启动端口号
server:
  addr: 8029

# 管理密码
logs_admin:
  password: abcd1234  # 请务必修改

# 存储选择
cos:
  backet: Local
  
# ...其它配置

# 图像生成参数
parameters:
  width: 832
  height: 1216
  scale: 5.5
  sampler: "k_euler_ancestral"
  steps: 28
  # ... 更多参数配置
```

### 2️⃣ 启动服务器

```bash
go run main.go
```

或者编译后运行：

```bash
go build -o a1111-server .
./a1111-server
```

服务启动后会显示：
```
Config loaded successfully
Logger initialized successfully
Starting server on : 8029
日志查询页面: http://localhost:8029/logs
默认管理密码: ...
```

### 3️⃣ 访问日志查询页面

打开浏览器访问：`http://localhost:8029/logs`

### 4️⃣ 登录系统

- 输入设置的密码
- Token 有效期：24小时

## 🔧 API 使用

### 图像生成 API

#### OpenAI 兼容格式

**请求地址**：`POST /v1/chat/completions`

**请求头**：
```
Authorization: Bearer your-novel-ai-token
Content-Type: application/json
```

**请求体**：
```json
{
  "model": "nai-diffusion-4-curated-preview",
  "messages": [
    {
      "role": "user", 
      "content": "一只猫"
    }
  ]
}
```

**响应**：
```json
{
  "id": "chatcmpl-xxxxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "nai-diffusion-4-curated-preview",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "https://your-storage.com/path/to/generated-image.png"
      },
      "finish_reason": "stop"
    }
  ]
}
```

#### DALL-E 兼容格式

**请求地址**：`POST /v1/images/generations`

**请求体**：
```json
{
  "model": "nai-diffusion-4-5-full",
  "prompt": "a cat",
  "n": 1,
  "size": "832x1216"
}
```

### 日志管理 API

#### 登录
```
POST /api/login
Content-Type: application/json

{
  "password": "abcd1234"
}
```

响应：
```json
{
  "success": true,
  "token": "xxxxx",
  "message": "登录成功"
}
```

#### 查询日志
```
GET /api/logs?page=1&page_size=20&keyword=girl
Authorization: Bearer <token>
```

响应：
```json
{
  "success": true,
  "data": [...],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

#### 获取日志详情
```
GET /api/logs/detail?id=<log_id>
Authorization: Bearer <token>
```

### 前端页面
```
GET  /                       # 日志查询页面
GET  /logs                   # 日志查询页面
```

### 支持的模型

| 模型名称 | 描述 | 版本 |
|---------|------|------|
| `nai-diffusion-3` | NAI Diffusion 3.0 标准版 | v3 |
| `nai-diffusion-furry-3` | NAI Diffusion 3.0 furry| v3 |
| `nai-diffusion-4-full` | NAI Diffusion 4.0 完整版 | v4 |
| `nai-diffusion-4-curated-preview` | NAI Diffusion 4.0 精选预览版 | v4 |
| `nai-diffusion-4-5-curated` | NAI Diffusion 4.5 精选版 | v4 |
| `nai-diffusion-4-5-full` | NAI Diffusion 4.5 完整版 | v4 |

## 其它功能

### 翻译系统

当启用翻译功能时，系统会自动：
1. 检测用户输入是否为中文
2. 调用配置的 AI 翻译服务
3. 将中文描述转换为 NovelAI 英文提示词
4. 使用翻译后的提示词生成图像

**翻译示例**：
- 输入：`"一个穿着白色长裙的天使"`
- 输出：`"{1girl},angel,white dress,{detailed eyes},{shine golden eyes},halo,{white wings}"`

### 日志查询系统

#### 🔐 密码认证
- 基于 Token 的认证机制
- Token 自动过期（24小时）
- 本地存储 Token，无需重复登录

#### 📋 自动日志记录
- 自动记录所有生成请求
- 记录成功和失败状态
- 保存用户IP、提示词、模型等信息
- 日志文件位置：`logs/image_logs.json`

每条日志包含：
- `id`: 唯一标识
- `timestamp`: 生成时间
- `model`: 使用的模型
- `prompt`: 提示词
- `image_url`: 图片URL
- `user_ip`: 用户IP地址
- `status`: 状态（success/failed）
- `error`: 错误信息（如果失败）

#### 🖼️ 图片预览
- 表格中显示缩略图
- 点击图片全屏预览
- ESC 键或点击背景关闭
- 平滑的缩放动画

#### 🔍 搜索功能
- 实时搜索
- 支持搜索提示词、模型、IP地址
- 防抖处理，避免频繁请求
- 长提示词智能折叠展开

### 图像处理流程

1. **请求解析**：解析 OpenAI 兼容的请求格式
2. **翻译处理**：可选的中文到英文提示词翻译
3. **模型路由**：根据模型名称选择对应的处理器
4. **图像生成**：调用 NovelAI API 生成图像
5. **文件上传**：将生成的图像上传到配置的存储
6. **日志记录**：自动记录生成结果
7. **响应返回**：返回图像访问链接


## 🔧 配置说明

### 服务器配置
- `server.addr`：服务监听端口

### 日志管理配置
- `logs_admin.password`：日志查询系统管理密码

### 存储配置
- `cos.backet`：选择存储服务类型（Local/Tengxun/Minio/Alist）

### 翻译配置
- `translation.enable`：是否启用翻译功能
- `translation.url`：翻译 API 地址
- `translation.key`：翻译 API 密钥
- `translation.model`：使用的翻译模型
- `translation.role`：翻译提示词模板

### 图像参数配置
- `parameters.width/height`：图像尺寸
- `parameters.scale`：生成比例（0.1-10.0）
- `parameters.sampler`：采样器类型
- `parameters.steps`：生成步数（1-50）
- `parameters.n_samples`：生成图像数量

## 部署指南

### 本地开发部署
```bash
# 开发模式启动
go run main.go

# 构建二进制文件
go build -o novel-api-server .
./novel-api-server
```

### Docker 部署
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o novel-api-server main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/novel-api-server .
COPY --from=builder /app/env .
COPY --from=builder /app/web ./web
COPY --from=builder /app/logs ./logs
CMD ["./novel-api-server"]
```

## 接入其它项目

通过使用本项目，便可以将comfyui和novelAI的模型接入到one-api等api聚合平台、rikkahub/kelivo/chatbox/sillytavern等AI聊天对话平台直接使用。

### 接入forksilly

本项目是安卓AIRP平台 [forksilly](https://github.com/fatsnk/forksilly.doc/blob/main/README.zh-CN.md) 的配套项目，有兴趣可下载试用一下。

## License

MIT。
