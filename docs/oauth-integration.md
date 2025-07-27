# OAuth 集成说明

本文档说明如何在 coze-studio 中集成 clinx OAuth 登录功能。

## 设计原则

- **最小化修改**: 通过新增文件的方式实现OAuth功能，避免修改现有代码
- **向后兼容**: 支持传统登录和OAuth登录两种方式
- **环境控制**: 通过环境变量控制是否启用OAuth功能
- **不修改IDL**: 通过msg字段传递重定向URL，避免修改thrift文件

## 文件结构

### 后端新增文件

1. `/backend/application/oauth/oauth.go` - OAuth应用服务
2. `/backend/application/oauth/request_util.go` - HTTP请求工具函数
3. `/backend/api/handler/coze/oauth_handler.go` - OAuth路由处理器

### 前端新增文件

1. `/frontend/packages/foundation/account-adapter/src/oauth-adapter/index.ts` - OAuth适配器
2. `/frontend/packages/foundation/account-adapter/src/utils/oauth-utils.ts` - OAuth工具函数

### 已修改的文件

1. `/clinx-admin/controller/clinx.go` - 增强了checkToken接口，返回OAuth重定向URL

## 核心功能

### 后端OAuth服务

- `PassportAccountInfoV2WithOAuth`: 带OAuth支持的账户信息获取
- `callClinxCheckToken`: 调用clinx的checkToken接口进行token验证
- `createUserFromClinx`: 根据clinx用户信息自动创建新用户
- `getUserByEmail`: 通过email查找现有用户

### 前端OAuth适配器

- `checkLoginWithOAuth`: 带OAuth支持的登录检查
- `handleOAuthCallback`: 处理OAuth回调
- 自动重定向到OAuth登录页面

## 使用方法

### 启用OAuth功能

1. 复制环境变量配置文件：
   ```bash
   cp .env.oauth.example .env.oauth
   ```
2. 修改 `.env.oauth` 文件中的配置：
   - 设置 `ENABLE_OAUTH=true`
   - 设置 `VITE_ENABLE_OAUTH=true`
   - 配置正确的 `OAUTH_REDIRECT_URL`
3. 确保clinx服务在 `http://localhost:3000` 运行
4. 重启coze-studio服务

### 路由配置

新增的OAuth处理器函数已包含路由注解：

```go
// PassportAccountInfoV2WithOAuth - OAuth版本的账户信息接口
// @router /passport/account/info/v2/oauth [POST]

// OAuthCallback - OAuth回调处理  
// @router /oauth/callback [GET]
```

这些路由会通过代码生成工具自动注册。

### 前端集成

```typescript
import { handleOAuthCallback, checkLogin } from './utils/oauth-utils';

// 在应用启动时处理OAuth回调
if (handleOAuthCallback()) {
  // OAuth登录成功，刷新页面状态
  window.location.reload();
}

// 使用带OAuth支持的登录检查
const { userInfo } = await checkLogin();
```

## OAuth流程

1. **用户访问受保护页面**
2. **前端调用带OAuth的登录检查**
3. **后端检查token有效性**
   - 如果token无效，在msg字段中返回JSON格式的重定向信息
4. **前端解析msg中的JSON，自动重定向到99U OAuth登录页面**
   - redirect_uri指向: `https://newapi.clinx.work/oauth/nd99u`
   - callback_uri指向: `http://localhost:8888/oauth/callback`
5. **用户在99U完成登录**
6. **99U重定向到clinx的OAuth端点，clinx处理后重定向回coze的callback页面**
7. **后端OAuth回调处理流程**：
   - 接收authorization code
   - 调用 `https://newapi.clinx.work/api/oauth/nd99u?code=xxx` 换取access_token
   - 使用access_token调用 `https://newapi.clinx.work/api/user/self` 获取用户信息
   - 根据email查找或创建本地用户
   - 为用户创建session
   - 重定向到前端首页
8. **用户成功登录coze系统**

## 重定向URL传递方式

为了避免修改thrift IDL文件，我们在msg字段中使用JSON格式传递重定向信息：

```json
{
  "message": "请先登录99U",  
  "redirect_url": "https://99u.com/oauth/authorize?..."
}
```

## 配置参数

### clinx OAuth配置

在 `buildOAuthRedirectURL` 函数中配置：
- `baseURL`: OAuth授权服务器地址
- `clientID`: coze应用的client_id  
- `redirectURI`: OAuth回调地址
- `scope`: 请求的权限范围

### 环境变量

- `ENABLE_OAUTH`: 是否启用OAuth功能 (true/false)
- 其他clinx相关配置参数

## 注意事项

1. **服务地址**: 确保clinx服务地址配置正确
2. **CORS配置**: 确保跨域请求配置正确
3. **环境变量**: 生产环境需要正确配置OAuth相关参数
4. **安全考虑**: 确保OAuth client_secret等敏感信息安全存储

## 故障排除

1. **OAuth重定向失败**: 检查redirect_uri配置是否正确
2. **Token验证失败**: 检查clinx服务是否正常运行
3. **用户创建失败**: 检查数据库连接和用户表结构
4. **前端重定向循环**: 检查环境变量和API端点配置

## 测试OAuth功能

### 测试步骤

1. **启动服务**:
   ```bash
   # 后端
   make server
   
   # 前端
   cd frontend/apps/coze-studio
   rushx dev
   ```

2. **启用OAuth**:
   ```bash
   cp .env.oauth.example .env.oauth
   # 编辑.env.oauth文件，确保ENABLE_OAUTH=true
   ```

3. **测试登录流程**:
   - 访问 `http://localhost:3000` (前端dev server)
   - 系统应自动重定向到99U登录页面
   - 在99U完成登录授权
   - 系统应重定向回coze并显示登录成功

4. **验证API调用**:
   ```bash
   # 测试OAuth API端点
   curl -X POST http://localhost:8888/api/passport/account/info/v2/oauth \
     -H "Content-Type: application/json" \
     -d "{}"
   ```

### 预期行为

- **未登录状态**: 返回401状态码，msg字段包含重定向URL的JSON
- **已登录状态**: 返回200状态码，data字段包含用户信息
- **OAuth回调**: 成功时重定向到 `/?oauth_success=true`

## 扩展功能

- 支持多种OAuth提供商
- 实现token刷新机制
- 添加OAuth用户权限管理
- 集成单点登出(SSO)功能