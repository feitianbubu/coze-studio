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

import { type UserInfo } from '@coze-foundation/account-base';

export const checkLoginWithOAuth = async (): Promise<{ userInfo?: UserInfo }> => {
  try {
    const res = await fetch('/api/passport/account/info/v2/oauth', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    });
    
    const data = await res.json() as { data: UserInfo; code: number; msg: string };
    
    if (data.code === 401) {
      try {
        const msgData = JSON.parse(data.msg) as { redirect_url?: string };
        if (msgData.redirect_url) {
          window.location.href = msgData.redirect_url;
        }
      } catch {
        console.error('Authentication failed:', data.msg);
      }
      return { userInfo: undefined };
    }
    
    return { userInfo: data.code === 0 ? data.data : undefined };
  } catch (error) {
    console.error('OAuth check login failed:', error);
    return { userInfo: undefined };
  }
};