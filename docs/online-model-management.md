# 在线模型管理功能

## 概述

当 `ENABLE_OAUTH=true` 时，系统将从在线 API 获取模型列表，而不是使用静态配置文件。

## 功能特性

### 自动切换机制

- **静态模式** (`ENABLE_OAUTH=false` 或未设置): 使用本地配置文件 (`backend/conf/model/`)
- **在线模式** (`ENABLE_OAUTH=true`): 从 `CLINX_API_BASE_URL/v1/models` 获取模型列表

### 在线模型 API

**接口地址**: `${CLINX_API_BASE_URL}/v1/models`  
**请求方法**: `GET`  
**认证方式**: `Authorization: Bearer ${clinx_access_token}`  
**响应格式**:

```json
{
  "data": [
    {
      "id": "gpt-4o-2024-08-06",
      "object": "model",
      "created": 1626777600,
      "owned_by": "openai",
      "supported_endpoint_types": ["openai"]
    },
    {
      "id": "claude-sonnet-4-20250514",
      "object": "model", 
      "created": 1626777600,
      "owned_by": "vertex-ai",
      "supported_endpoint_types": ["openai"]
    },
    {
      "id": "deepseek-v3-0324",
      "object": "model",
      "created": 1626777600,
      "owned_by": "custom",
      "supported_endpoint_types": ["openai"]
    }
  ],
  "success": true
}
```

### 响应字段说明

- **id**: 模型唯一标识符
- **object**: 固定值 "model"
- **created**: 模型创建时间戳
- **owned_by**: 模型提供商 (`openai`, `vertex-ai`, `custom`, `jimeng`)
- **supported_endpoint_types**: 支持的端点类型（通常为 `["openai"]`）

### 模型信息推断

由于 API 返回的是简化格式，系统会根据 `id` 和 `owned_by` 推断详细信息：

#### 协议推断

| owned_by | 模型 ID 模式 | 推断协议 |
|---|---|---|
| `openai` | - | `openai` |
| `vertex-ai` | `claude-*` | `claude` |
| `vertex-ai` | `gemini-*` | `gemini` |
| `custom` | `claude-*` | `claude` |
| `custom` | `deepseek-*` | `deepseek` |
| `custom` | `doubao-*` | `ark` |
| `custom` | `gemini-*` | `gemini` |
| `custom` | `kimi-*` | `qwen` |
| `jimeng` | - | `openai` |

#### 能力推断

系统根据模型 ID 自动推断能力：

| 模型类型 | 推断能力 |
|---|---|
| Claude 系列 | Function Call, JSON Mode, Reasoning, Vision |
| GPT-4 系列 | Function Call, JSON Mode, Vision |
| DeepSeek 系列 | Function Call, JSON Mode, Reasoning |
| 音频模型 (`*audio*`) | Audio Input/Output |
| 视频模型 (`vidu*`, `kling*`, `*vgfm*`) | Video Input/Output |

#### 显示名称生成

| 模型 ID | 显示名称 |
|---|---|
| `claude-sonnet-4-20250514` | Claude 4 Sonnet |
| `gpt-4o-2024-08-06` | GPT-4o |
| `deepseek-v3-0324` | DeepSeek V3 |
| `gemini-2.5-pro` | Gemini 2.5 Pro |

### 模型状态

- `active`: 映射为 `StatusInUse` (可用)
- `offline`, `disabled`: 映射为 `StatusDeleted` (不可用)

## 认证机制

### 访问令牌获取

在线模型 API 使用 CLINX 访问令牌进行认证：

1. **Cookie 获取**: 系统从用户请求的 Cookie 中获取 `clinx_access_token`
2. **中间件处理**: Session 中间件自动将 cookie 值存储到请求上下文中
3. **自动添加**: 在线模型管理器自动从上下文获取 token 并添加到 API 请求头

### 认证流程

```
用户请求 → Session 中间件 → 提取 clinx_access_token → 存储到上下文 
    ↓
模型管理器 → 从上下文获取 token → 添加 Authorization header → 调用 CLINX API
```

### 认证格式

系统使用标准的 Bearer token 认证格式：

```
Authorization: Bearer {clinx_access_token}
```

## 模型配置

### 协议支持

在线模型使用 OpenAI 协议 (`ProtocolOpenAI`)，具有以下特性：

1. **OpenAI 兼容**: 使用标准的 OpenAI API 格式，支持相同的参数和响应结构
2. **标准端点**: 统一使用标准的 `/v1/chat/completions` 端点
3. **统一认证**: 使用标准的 Bearer token 认证方式

### BaseURL 设置

在线模型的 `ConnConfig.BaseURL` 会自动设置为 `CLINX_API_BASE_URL` 环境变量的值。这确保：

1. **统一基础地址**: 所有在线模型使用相同的 CLINX API 基础地址
2. **标准端点**: 使用标准的 OpenAI 兼容端点 `/v1/chat/completions`
3. **动态配置**: 可通过环境变量灵活配置不同环境的 API 地址

### API 端点

所有模型统一使用标准的 OpenAI 兼容端点：

```
BaseURL + /v1/chat/completions
```

例如：`https://newapi.clinx.work/v1/chat/completions`

### API Key 管理

- **ConnConfig.APIKey**: 在模型配置中留空，运行时会使用用户的 `clinx_access_token`
- **动态注入**: 实际的 API 调用时，系统会自动注入用户的访问令牌

## 配置

### 环境变量

```bash
# 启用 OAuth 和在线模型管理
ENABLE_OAUTH=true

# CLINX API 基础地址
CLINX_API_BASE_URL=https://newapi.clinx.work
```

### 日志输出

启用在线模式时，系统会输出以下日志：

```
[initModelMgr] ENABLE_OAUTH is true, using online model manager
[fetchModelsFromClinx] Added Authorization header with access token
[fetchModelsFromClinx] fetched X models from CLINX API
```

如果没有找到访问令牌，会输出警告日志：

```
[fetchModelsFromClinx] No clinx_access_token found in context
```

## 错误处理

### API 调用失败

- HTTP 状态码非 200: 返回相应错误信息
- 网络超时: 30 秒超时设置
- JSON 解析失败: 返回解析错误

### 降级策略

如果在线 API 不可用，系统不会自动降级到静态配置，而是返回错误。建议在部署时确保 CLINX API 服务的可用性。

## 实现细节

### 核心文件

- `backend/infra/impl/modelmgr/online/modelmgr.go`: 在线模型管理器实现
- `backend/application/base/appinfra/modelmgr.go`: 模型管理器初始化逻辑

### 接口兼容性

在线模型管理器完全实现了 `modelmgr.Manager` 接口：

```go
type Manager interface {
    ListModel(ctx context.Context, req *ListModelRequest) (*ListModelResponse, error)
    ListInUseModel(ctx context.Context, limit int, Cursor *string) (*ListModelResponse, error)
    MGetModelByID(ctx context.Context, req *MGetModelRequest) ([]*Model, error)
}
```

### 测试覆盖

- 模型列表获取测试
- 在用模型过滤测试
- API 失败处理测试
- 协议推断测试
- 能力解析测试
- **认证令牌处理测试**
- **无令牌场景测试**

## 使用示例

### 启用在线模式

1. 设置环境变量:
   ```bash
   export ENABLE_OAUTH=true
   export CLINX_API_BASE_URL=https://newapi.clinx.work
   ```

2. 启动服务:
   ```bash
   make server
   ```

3. 调用模型列表 API（需要用户登录状态）:
   ```bash
   curl -X POST http://localhost:8888/api/bot/get_type_list \
     -H "Content-Type: application/json" \
     -H "Cookie: clinx_access_token=your-access-token" \
     -d '{"model": true}'
   ```

### 日志验证

查看日志确认在线模式已启用：

```bash
tail -f backend.log | grep "using online model manager"
```

## 注意事项

1. **用户认证**: 在线模式需要用户通过 OAuth 登录，获取有效的 `clinx_access_token`
2. **网络依赖**: 在线模式依赖外部 API，确保网络连通性
3. **性能考虑**: 每次调用都会请求在线 API，考虑添加缓存机制
4. **错误监控**: 建议添加 API 调用失败的监控和告警
5. **数据一致性**: 在线模型数据可能实时变化，注意前端缓存更新
6. **令牌管理**: 确保 `clinx_access_token` 的有效性和及时更新

## 后续优化

- [ ] 添加模型列表缓存机制
- [ ] 支持令牌刷新和重试机制
- [ ] 添加更多模型提供商支持
- [ ] 实现配置热更新
- [ ] 添加模型可用性检查
- [ ] API 调用限流和防护