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
	"net/http"
)

const requestContextKey = "oauth.http_request"

// SetRequestToCtx 将HTTP请求添加到上下文
func SetRequestToCtx(ctx context.Context, req *http.Request) context.Context {
	return context.WithValue(ctx, requestContextKey, req)
}

// GetRequestFromCtx 从上下文获取HTTP请求
func GetRequestFromCtx(ctx context.Context) *http.Request {
	if req, ok := ctx.Value(requestContextKey).(*http.Request); ok {
		return req
	}
	return nil
}
