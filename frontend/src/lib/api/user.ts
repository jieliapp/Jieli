import { api } from './client';
import type {
  ActivityDay,
  ApiKeyInfo,
  ApiKeyRotateResult,
  AvatarUploadPayload,
  Profile,
  ProfileUpdatePayload,
  PublicProfile,
  User
} from './types';

export const userApi = {
  profile: () => api.get<Profile>('/user/profile'),
  publicProfile: (username: string) => api.get<PublicProfile>(`/users/${encodeURIComponent(username)}/profile`),
  publicActivity: (username: string) => api.get<ActivityDay[]>(`/users/${encodeURIComponent(username)}/activity`),
  updateProfile: (payload: ProfileUpdatePayload) => api.patch<User>('/user/profile', payload),
  uploadAvatar: (payload: AvatarUploadPayload) => api.post<User>('/user/avatar', payload),
  apiKey: () => api.get<ApiKeyInfo>('/user/api-key'),
  rotateApiKey: () => api.post<ApiKeyRotateResult>('/user/api-key/rotate'),
  activity: () => api.get<ActivityDay[]>('/user/activity'),
  setLocale: (locale: string) =>
    api.patch<void>('/user/locale', { locale }),
  setAutoCleanup: (enabled: boolean) =>
    api.put<{ auto_cleanup_enabled: boolean }>('/user/auto-cleanup', { enabled }),
  restoreThreads: () => api.post<{ restored: number }>('/user/restore-threads')
};
