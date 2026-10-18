import { api } from './client';
import type {
  ThreadDetail,
  ThreadFacets,
  ThreadListFilters,
  ThreadListResult,
  ThreadSummary,
  Visibility
} from './types';

export interface ThreadMetaUpdate {
  title?: string;
  visibility?: Visibility;
  labels?: string[];
}

export const threadsApi = {
  list: (filters: ThreadListFilters = {}, signal?: AbortSignal) =>
    api.get<ThreadListResult>(
      '/threads',
      {
        search: filters.search,
        provider: filters.provider,
        repo: filters.repo,
        label: filters.label,
        visibility: filters.visibility,
        archived: filters.archived,
        sort: filters.sort,
        page: filters.page,
        page_size: filters.page_size
      },
      signal
    ),

  recent: (limit = 9) =>
    api.get<ThreadListResult>('/threads', {
      page_size: limit,
      page: 1,
      sort: 'updated',
      archived: false
    }),

  facets: () => api.get<ThreadFacets>('/threads/facets'),

  detail: (id: number | string) => api.get<ThreadDetail>(`/threads/${id}`),

  shared: (slug: string) => api.get<ThreadDetail>(`/share/${slug}`),

  updateMeta: (id: number | string, meta: ThreadMetaUpdate) =>
    api.patch<ThreadSummary>(`/threads/${id}/meta`, meta),

  archive: (id: number | string, archived: boolean) =>
    api.post<ThreadSummary>(`/threads/${id}/archive`, { archived }),

  remove: (id: number | string) => api.delete<void>(`/threads/${id}`)
};
