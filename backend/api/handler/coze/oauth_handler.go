/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package coze

import (
	"context"
	"github.com/coze-dev/coze-studio/backend/domain/user/entity"
	"net/http"
	"sync"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/coze-dev/coze-studio/backend/api/model/passport"
	"github.com/coze-dev/coze-studio/backend/application/oauth"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

var oauthInitOnce sync.Once

// ensureOAuthServiceInit 确保OAuth服务已初始化
func ensureOAuthServiceInit() {
	oauthInitOnce.Do(func() {
		oauth.InitWithUserApplicationService()
	})
}

// PassportAccountInfoV2WithOAuth 带OAuth支持的账户信息接口
// @router /passport/account/info/v2/oauth [POST]
func PassportAccountInfoV2WithOAuth(ctx context.Context, c *app.RequestContext) {
	// 确保OAuth服务已初始化
	ensureOAuthServiceInit()

	var req passport.PassportAccountInfoV2Request
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorx.New(errno.ErrUserInvalidParamCode, errorx.KV("msg", err.Error())))
		return
	}

	// 将HTTP请求信息添加到上下文中，以便OAuth服务获取Authorization头
	// 转换hertz请求为标准http请求格式
	httpReq := &http.Request{
		Header: make(http.Header),
	}
	// 复制Authorization头
	if auth := string(c.GetHeader("Authorization")); auth != "" {
		httpReq.Header.Set("Authorization", auth)
	}
	ctx = oauth.SetRequestToCtx(ctx, httpReq)

	resp, err := oauth.OAuthApplicationSVC.PassportAccountInfoV2WithOAuth(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	// 如果验证失败，返回401状态码
	if resp.Code == 401 {
		c.JSON(http.StatusUnauthorized, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// OAuthCallback OAuth回调处理
// @router /oauth/callback [GET]
func OAuthCallback(ctx context.Context, c *app.RequestContext) {
	// 确保OAuth服务已初始化
	ensureOAuthServiceInit()

	// 获取授权码 (支持code和uckey参数)
	code := c.Query("code")
	if code == "" {
		code = c.Query("uckey") // 兼容99U OAuth的uckey参数
	}
	if code == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"code": 400,
			"msg":  "Missing authorization code",
		})
		return
	}

	// 处理OAuth回调，换取token并创建会话
	result, err := oauth.OAuthApplicationSVC.HandleOAuthCallback(ctx, c, code)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	// 如果成功，设置session cookie并重定向
	if result.SessionID != "" {
		// 设置session cookie
		c.SetCookie(entity.SessionKey, result.SessionID, 86400*7, "/", "", protocol.CookieSameSiteLaxMode, false, true)

		// 设置clinx access token cookie for LLM API calls
		if result.AccessToken != "" {
			c.SetCookie("clinx_access_token", result.AccessToken, 86400*7, "/", "", protocol.CookieSameSiteLaxMode, false, true)
		}

		// 重定向到前端首页
		redirectURL := "/?oauth_success=true"
		if result.RedirectURL != "" {
			redirectURL = result.RedirectURL
		}
		c.Redirect(http.StatusFound, []byte(redirectURL))
		return
	}

	// 如果失败，重定向到登录页面
	c.Redirect(http.StatusFound, []byte("/sign?oauth_error=callback_failed"))
}
