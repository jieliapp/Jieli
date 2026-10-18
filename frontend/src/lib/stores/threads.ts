import { writable } from 'svelte/store';
import type { ThreadListFilters } from '../api/types';

export const DEFAULT_PAGE_SIZE = 20;

export const defaultFilters: ThreadListFilters = {
  search: '',
  provider: '',
  repo: '',
  label: '',
  visibility: '',
  archived: false,
  page: 1,
  page_size: DEFAULT_PAGE_SIZE
};

export const threadFilters = writable<ThreadListFilters>({ ...defaultFilters });

export function resetFilters() {
  threadFilters.set({ ...defaultFilters });
}
