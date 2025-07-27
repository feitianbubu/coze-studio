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

package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/coze-dev/coze-studio/backend/api/model/passport"
	"github.com/coze-dev/coze-studio/backend/domain/user/entity"
	userSvc "github.com/coze-dev/coze-studio/backend/domain/user/service"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

var OAuthApplicationSVC = &OAuthApplicationService{}

type OAuthApplicationService struct {
	DomainSVC userSvc.User
}

// Init 初始化OAuth应用服务
func (o *OAuthApplicationService) Init(userSVC userSvc.User) {
	o.DomainSVC = userSVC
}

// ClinxCheckTokenResponse clinx checkToken接口的响应结构
type ClinxCheckTokenResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RedirectURL string `json:"redirect_url,omitempty"`
		ID          int    `json:"id,omitempty"`
		Username    string `json:"username,omitempty"`
		DisplayName string `json:"display_name,omitempty"`
		Email       string `json:"email,omitempty"`
		Role        int    `json:"role,omitempty"`
		Group       string `json:"group,omitempty"`
		Quota       int    `json:"quota,omitempty"`
		UsedQuota   int    `json:"used_quota,omitempty"`
	} `json:"data"`
}

// callClinxCheckToken 调用clinx的checkToken接口
func (o *OAuthApplicationService) callClinxCheckToken(ctx context.Context, token string) (*ClinxCheckTokenResponse, error) {
	clinxBaseURL := os.Getenv("CLINX_API_BASE_URL")

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", clinxBaseURL+"/api/checkToken", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var clinxResp ClinxCheckTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&clinxResp)
	if err != nil {
		return nil, err
	}

	return &clinxResp, nil
}

// createUserFromClinx 根据clinx用户信息创建新用户
func (o *OAuthApplicationService) createUserFromClinx(ctx context.Context, clinxUser *ClinxCheckTokenResponse) (*entity.User, error) {
	// 验证email格式
	if !isValidEmail(clinxUser.Data.Email) {
		return nil, errorx.New(errno.ErrUserInvalidParamCode, errorx.KV("msg", "Invalid email from clinx"))
	}

	// 使用clinx用户信息创建新用户
	userInfo, err := o.DomainSVC.Create(ctx, &userSvc.CreateUserRequest{
		Email:    clinxUser.Data.Email,
		Password: "", // OAuth用户无需密码
		Locale:   "zh",
	})
	if err != nil {
		return nil, err
	}

	// 更新用户信息为clinx提供的信息
	displayName := clinxUser.Data.DisplayName
	if displayName == "" {
		displayName = clinxUser.Data.Username
	}

	err = o.DomainSVC.UpdateProfile(ctx, &userSvc.UpdateProfileRequest{
		UserID:      userInfo.UserID,
		Name:        &displayName,
		UniqueName:  &clinxUser.Data.Username,
		Description: nil,
		Locale:      nil,
	})
	if err != nil {
		return nil, err
	}

	return userInfo, nil
}

// OAuthCallbackResult OAuth回调处理结果
type OAuthCallbackResult struct {
	SessionID   string
	RedirectURL string
	UserInfo    *entity.User
	AccessToken string // clinx access token for LLM API calls
}

// ClinxUserInfoResponse clinx用户信息响应结构
type ClinxUserInfoResponse struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        int    `json:"role"`
	Group       string `json:"group"`
	Quota       int    `json:"quota"`
	UsedQuota   int    `json:"used_quota"`
}

// exchangeCodeForToken 使用授权码换取访问令牌
func (o *OAuthApplicationService) exchangeCodeForToken(_ context.Context, c *app.RequestContext, code string) (string, error) {
	clinxBaseURL := os.Getenv("CLINX_API_BASE_URL")

	// 构建token交换请求URL (99U后端验证接口)
	tokenURL := clinxBaseURL + "/api/oauth/nd99u?code=" + code

	client := resty.New().SetTimeout(10 * time.Second)

	resp, err := client.R().Get(tokenURL)
	if err != nil {
		return "", err
	}

	type TokenResponse struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			ID               int         `json:"id"`
			Username         string      `json:"username"`
			Password         string      `json:"password"`
			OriginalPassword string      `json:"original_password"`
			DisplayName      string      `json:"display_name"`
			Role             int         `json:"role"`
			Status           int         `json:"status"`
			Email            string      `json:"email"`
			GithubID         string      `json:"github_id"`
			OidcID           string      `json:"oidc_id"`
			WechatID         string      `json:"wechat_id"`
			TelegramID       string      `json:"telegram_id"`
			VerificationCode string      `json:"verification_code"`
			AccessToken      string      `json:"access_token"`
			Quota            int         `json:"quota"`
			UsedQuota        int         `json:"used_quota"`
			RequestCount     int         `json:"request_count"`
			Group            string      `json:"group"`
			AffCode          string      `json:"aff_code"`
			AffCount         int         `json:"aff_count"`
			AffQuota         int         `json:"aff_quota"`
			AffHistoryQuota  int         `json:"aff_history_quota"`
			InviterID        int         `json:"inviter_id"`
			DeletedAt        interface{} `json:"DeletedAt"`
			LinuxDoID        string      `json:"linux_do_id"`
			Setting          string      `json:"setting"`
			StripeCustomer   string      `json:"stripe_customer"`
			NDAccessToken    string      `json:"nd_access_token"`
		} `json:"data"`
	}

	var tokenResp TokenResponse
	err = json.Unmarshal(resp.Body(), &tokenResp)
	if err != nil {
		return "", err
	}

	if !tokenResp.Success {
		return "", fmt.Errorf("token exchange failed: %s", tokenResp.Message)
	}

	if ndAccessToken := tokenResp.Data.NDAccessToken; ndAccessToken != "" {
		c.SetCookie("nd_access_token", ndAccessToken, 86400*7, "/", "", protocol.CookieSameSiteLaxMode, false, true)
	}

	return tokenResp.Data.AccessToken, nil
}

// getUserInfoWithToken 使用访问令牌获取用户信息
func (o *OAuthApplicationService) getUserInfoWithToken(ctx context.Context, accessToken string) (*ClinxUserInfoResponse, error) {
	clinxBaseURL := os.Getenv("CLINX_API_BASE_URL")

	client := resty.New().SetTimeout(10 * time.Second)
	req := client.R().SetHeader("Authorization", "Bearer "+accessToken).SetHeader("Content-Type", "application/json")

	resp, err := req.Get(clinxBaseURL + "/api/user/self")
	if err != nil {
		return nil, err
	}

	type UserInfoResponse struct {
		Success bool                  `json:"success"`
		Message string                `json:"message"`
		Data    ClinxUserInfoResponse `json:"data"`
	}

	var userResp UserInfoResponse
	err = json.Unmarshal(resp.Body(), &userResp)
	if err != nil {
		return nil, err
	}

	if !userResp.Success {
		return nil, fmt.Errorf("get user info failed: %s", userResp.Message)
	}

	return &userResp.Data, nil
}

// HandleOAuthCallback 处理OAuth回调，换取token并创建会话
func (o *OAuthApplicationService) HandleOAuthCallback(ctx context.Context, c *app.RequestContext, code string) (*OAuthCallbackResult, error) {
	if code == "" {
		return nil, errorx.New(errno.ErrUserInvalidParamCode, errorx.KV("msg", "Invalid authorization code"))
	}

	// 1. 使用code换取access_token
	accessToken, err := o.exchangeCodeForToken(ctx, c, code)
	if err != nil {
		return nil, errorx.New(errno.ErrUserInvalidParamCode, errorx.KV("msg", "Failed to exchange code for token: "+err.Error()))
	}

	// 2. 使用access_token获取用户信息
	clinxUserInfo, err := o.getUserInfoWithToken(ctx, accessToken)
	if err != nil {
		return nil, errorx.New(errno.ErrUserInvalidParamCode, errorx.KV("msg", "Failed to get user info: "+err.Error()))
	}

	// 3. 查找或创建本地用户
	var userInfo *entity.User
	userInfo, err = o.getUserByEmail(ctx, clinxUserInfo.Email)
	if err != nil {
		// 用户不存在，创建新用户
		userInfo, err = o.createUserFromClinxUserInfo(ctx, clinxUserInfo)
		if err != nil {
			return nil, err
		}
	}

	// 4. 为OAuth用户创建session
	// 调用用户服务为OAuth用户生成session
	sessionKey, err := o.createSessionForOAuthUser(ctx, userInfo)
	if err != nil {
		return nil, errorx.New(errno.ErrUserInvalidParamCode, errorx.KV("msg", "Failed to create session: "+err.Error()))
	}

	return &OAuthCallbackResult{
		SessionID:   sessionKey,
		RedirectURL: "/?oauth_success=true",
		UserInfo:    userInfo,
		AccessToken: accessToken, // Pass through the access token for LLM API calls
	}, nil
}

// createSessionForOAuthUser 为OAuth用户创建session
func (o *OAuthApplicationService) createSessionForOAuthUser(ctx context.Context, userInfo *entity.User) (string, error) {
	// 模拟登录过程来创建session，使用空密码
	// 这里我们直接调用用户领域服务的内部方法来生成session
	// 但由于我们无法直接访问私有方法，我们需要通过用户服务的公开接口

	// 重新获取用户信息以获取最新的session key
	updatedUser, err := o.DomainSVC.GetUserInfo(ctx, userInfo.UserID)
	if err != nil {
		return "", err
	}

	// 如果用户已经有session key，直接返回
	if updatedUser.SessionKey != "" {
		return updatedUser.SessionKey, nil
	}

	// 如果没有session key，通过模拟登录来生成一个
	// 这是一个临时方案，实际上应该有专门的session创建接口
	// 但考虑到OAuth用户可能没有密码，我们生成一个临时session

	// 生成一个基于用户ID和时间戳的简单session key
	// 实际生产环境中应该使用更安全的方法
	timestamp := time.Now().UnixNano()
	sessionData := fmt.Sprintf("oauth_%s_%d", userInfo.UserID, timestamp)

	// 这里应该调用实际的session生成逻辑，但由于架构限制，
	// 我们暂时返回一个简化的session key
	return fmt.Sprintf("oauth_session_%x", []byte(sessionData)[:16]), nil
}

// createUserFromClinxUserInfo 根据clinx用户信息创建新用户
func (o *OAuthApplicationService) createUserFromClinxUserInfo(ctx context.Context, clinxUser *ClinxUserInfoResponse) (*entity.User, error) {
	// 验证email格式
	if !isValidEmail(clinxUser.Email) {
		return nil, errorx.New(errno.ErrUserInvalidParamCode, errorx.KV("msg", "Invalid email from clinx"))
	}

	// 使用clinx用户信息创建新用户
	userInfo, err := o.DomainSVC.Create(ctx, &userSvc.CreateUserRequest{
		Email:    clinxUser.Email,
		Password: "", // OAuth用户无需密码
		Locale:   "zh-CN",
	})
	if err != nil {
		return nil, err
	}

	// 更新用户信息为clinx提供的信息
	displayName := clinxUser.DisplayName
	if displayName == "" {
		displayName = clinxUser.Username
	}

	err = o.DomainSVC.UpdateProfile(ctx, &userSvc.UpdateProfileRequest{
		UserID:      userInfo.UserID,
		Name:        &displayName,
		Description: nil,
		Locale:      nil,
	})
	if err != nil {
		return nil, err
	}

	return userInfo, nil
}

// getUserByEmail 通过email查找用户
func (o *OAuthApplicationService) getUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	// 使用邮箱登录的方式来查找用户，如果用户不存在会返回错误
	// 这里我们传入空密码，只是为了触发用户查找逻辑
	loginResp, err := o.DomainSVC.Login(ctx, email, "")
	if err != nil {
		// 如果是用户不存在的错误，返回nil让上层创建新用户
		return nil, errorx.New(errno.ErrUserResourceNotFound, errorx.KV("type", "user"))
	}

	return loginResp, nil
}

// PassportAccountInfoV2WithOAuth 带OAuth支持的账户信息获取
func (o *OAuthApplicationService) PassportAccountInfoV2WithOAuth(ctx context.Context, req *passport.PassportAccountInfoV2Request) (
	resp *passport.PassportAccountInfoV2Response, err error,
) {
	// 获取环境变量
	clinxAuthURL := os.Getenv("CLINX_AUTH_URL")
	clinxAppID := os.Getenv("SDP_APP_ID")
	callbackURL := os.Getenv("OAUTH_CALLBACK_URL")

	// 尝试通过clinx验证
	// 从请求头获取Authorization token
	token := ""
	if req := GetRequestFromCtx(ctx); req != nil {
		token = req.Header.Get("Authorization")
	}

	if token == "" {
		// 无token，返回需要OAuth登录
		// 构建OAuth授权URL，redirect_uri指向clinx的OAuth端点，callback_uri指向我们的回调地址
		oauthURL := clinxAuthURL + "/?re_login=true&redirect_type=window&send_uckey=true" +
			"&redirect_uri=" + url.QueryEscape(callbackURL) +
			"&sdp-app-id=" + clinxAppID + "&lang=zh-CN#/login"

		// 将重定向URL放在msg中，使用JSON格式
		msgData := map[string]interface{}{
			"message":      "请先登录99U",
			"redirect_url": oauthURL,
		}
		msgJSON, _ := json.Marshal(msgData)

		return &passport.PassportAccountInfoV2Response{
			Data: nil,
			Code: 401,
			Msg:  string(msgJSON),
		}, nil
	}

	// 调用clinx checkToken验证
	clinxResp, err := o.callClinxCheckToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if clinxResp.Code != 0 {
		// clinx验证失败，返回OAuth重定向
		// 构建OAuth授权URL
		oauthURL := clinxAuthURL + "/?re_login=true&redirect_type=window&send_uckey=true" +
			"&redirect_uri=" + url.QueryEscape(callbackURL) +
			"&sdp-app-id=" + clinxAppID + "&lang=zh-CN#/login"

		// 将重定向URL放在msg中，使用JSON格式
		msgData := map[string]interface{}{
			"message":      clinxResp.Message,
			"redirect_url": oauthURL,
		}
		msgJSON, _ := json.Marshal(msgData)

		return &passport.PassportAccountInfoV2Response{
			Data: nil,
			Code: 401,
			Msg:  string(msgJSON),
		}, nil
	}

	// clinx验证成功，查找或创建用户
	var userInfo *entity.User
	userInfo, err = o.getUserByEmail(ctx, clinxResp.Data.Email)
	if err != nil {
		// 用户不存在，创建新用户
		userInfo, err = o.createUserFromClinx(ctx, clinxResp)
		if err != nil {
			return nil, err
		}
	}

	return &passport.PassportAccountInfoV2Response{
		Data: userDo2PassportTo(userInfo),
		Code: 0,
	}, nil
}

// 添加一个简单的 email 验证函数
func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func userDo2PassportTo(userDo *entity.User) *passport.User {
	var locale *string
	if userDo.Locale != "" {
		locale = ptr.Of(userDo.Locale)
	}

	return &passport.User{
		UserIDStr:      userDo.UserID,
		Name:           userDo.Name,
		ScreenName:     ptr.Of(userDo.Name),
		UserUniqueName: userDo.UniqueName,
		Email:          userDo.Email,
		Description:    userDo.Description,
		AvatarURL:      userDo.IconURL,
		AppUserInfo: &passport.AppUserInfo{
			UserUniqueName: userDo.UniqueName,
		},
		Locale: locale,

		UserCreateTime: userDo.CreatedAt / 1000,
	}
}
