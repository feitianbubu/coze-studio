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

package ctxutil

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-studio/backend/pkg/ctxcache"
)

func getUserLLMAPIKey(ctx context.Context) (string, error) {
	if accessToken, ok := ctxcache.Get[string](ctx, "clinx_access_token"); ok && accessToken != "" {
		return accessToken, nil
	}

	return "", errors.New("user access key not available - using configured key as fallback")
}

func TryGetUserAPIKey(ctx context.Context, originalAPIKey string) string {
	if userAPIKey, err := getUserLLMAPIKey(ctx); err == nil && userAPIKey != "" {
		return userAPIKey
	}
	return originalAPIKey
}
