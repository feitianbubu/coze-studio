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

import {
  refreshUserInfoBase,
  logoutBase,
  checkLoginBase,
  type Connector2Redirect,
} from '@coze-foundation/account-base';

import { passportApi } from '../passport-api';
import { oauthApi, checkLoginWithOAuth } from '../oauth-adapter';

// 环境变量控制是否启用OAuth，确保在浏览器环境中安全访问
const ENABLE_OAUTH = (typeof import.meta !== 'undefined' && import.meta.env?.VITE_ENABLE_OAUTH) === 'true' ||
  (typeof process !== 'undefined' && process.env?.ENABLE_OAUTH) === 'true';

export const refreshUserInfo = () =>
  refreshUserInfoBase(ENABLE_OAUTH ? oauthApi.checkLoginWithOAuth : passportApi.checkLogin);

export const logout = () => logoutBase(passportApi.logout);

export const checkLoginImpl = ENABLE_OAUTH ? checkLoginWithOAuth : async () => {
  try {
    const res = await passportApi.checkLogin();
    return { userInfo: res };
  } catch (e) {
    return { userInfo: undefined };
  }
};

export const checkLogin = () => checkLoginBase(checkLoginImpl);

// 开源版本不支持渠道授权，暂无实现
export const connector2Redirect: Connector2Redirect = () => undefined;

// OAuth回调处理
export const handleOAuthCallback = () => {
  if (ENABLE_OAUTH) {
    return oauthApi.handleOAuthCallback();
  }
  return false;
};