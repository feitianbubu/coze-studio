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

import { useLocation, useNavigate } from 'react-router-dom';

import { useCheckLoginBase } from '@coze-foundation/account-base';

import { signPath, signRedirectKey } from '../utils/constants';
import { checkLoginImpl } from '../utils';

const useGoLogin = (loginFallbackPath?: string) => {
  const navigate = useNavigate();
  const { pathname, search } = useLocation();
  return () => {
    // 构建OAuth重定向URL，redirect_uri指向clinx的OAuth端点，实际回调会由后端处理
    const callbackURL = encodeURIComponent(window.location.origin + '/oauth/callback');
    
    const redirectURL = (typeof import.meta !== 'undefined' && import.meta.env?.OAUTH_REDIRECT_URL) || 
      `https://uc-component.101.com/?re_login=true&redirect_type=window&send_uckey=true&redirect_uri=${callbackURL}&sdp-app-id=2f8492db-41c2-4ed3-bd09-78832ca95f37&lang=zh-CN#/login`;
    window.location.href = redirectURL;
  };
};

export const useCheckLogin = ({
  needLogin,
  loginFallbackPath,
}: {
  needLogin?: boolean;
  loginFallbackPath?: string;
}) => {
  const goLogin = useGoLogin(loginFallbackPath);
  useCheckLoginBase(!!needLogin, checkLoginImpl, goLogin);
};
