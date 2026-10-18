import { api } from './client';
import type { User } from './types';

export const authApi = {
  me: () => api.get<User>('/auth/me'),
  logout: () => api.post<void>('/auth/logout'),
  signInUrl: () => '/web/auth/signin',
  /** Full-page navigation entry point for GitHub OAuth (server-driven). */
  githubLoginUrl: () => '/web/api/auth/github',
  /** Full-page navigation entry point for Google OAuth (server-driven). */
  googleLoginUrl: () => '/web/api/auth/google',
  /** Full-page navigation entry point for dev auto-login (dev mode only). */
  devLoginUrl: () => '/web/api/auth/dev-login'
};

export interface ServerConfig {
  cloud_billing?: boolean;
  self_hosted?: boolean;
  dev_mode: boolean;
  github_oauth: boolean;
  google_oauth: boolean;
}

export const configApi = {
  get: (): Promise<ServerConfig> => {
    const injected = typeof window !== 'undefined' ? window.__APP_CONFIG__ : undefined;
    if (injected) {
      return Promise.resolve(injected);
    }
    return api.get<ServerConfig>('/config');
  }
};
