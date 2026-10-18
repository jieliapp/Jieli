import { writable } from 'svelte/store';
import { authApi } from '../api/auth';
import { ApiError, onUnauthorized } from '../api/client';
import { applyPreferredLocale } from './locale';
import type { User } from '../api/types';

export interface SessionState {
  user: User | null;
  loading: boolean;
  loaded: boolean;
}

const initial: SessionState = { user: null, loading: false, loaded: false };

export const session = writable<SessionState>(initial);

export function updateSessionUser(user: User): void {
  session.update((s) => ({ ...s, user: s.user ? { ...s.user, ...user } : user }));
}

/** Reset to signed-out whenever the API client observes a 401. */
onUnauthorized(() => {
  session.set({ user: null, loading: false, loaded: true });
});

export async function loadSession(): Promise<void> {
  session.update((s) => ({ ...s, loading: true }));
  try {
    const user = await authApi.me();
    session.set({ user, loading: false, loaded: true });
    applyPreferredLocale(user?.preferred_locale);
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      session.set({ user: null, loading: false, loaded: true });
    } else {
      session.set({ user: null, loading: false, loaded: true });
    }
  }
}

export async function logout(): Promise<void> {
  try {
    await authApi.logout();
  } finally {
    session.set({ user: null, loading: false, loaded: true });
  }
}
