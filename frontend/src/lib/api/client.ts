/**
 * Unified API client for Jieli backend.
 * Backend response envelope (aligned with one-mcp):
 *   { "success": boolean, "message": string, "data": T }
 */

export interface ApiEnvelope<T> {
  success: boolean;
  message: string;
  data?: T;
}

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

/** Subscribers notified whenever a 401 is observed, so session state can reset. */
type UnauthorizedHandler = () => void;
const unauthorizedHandlers = new Set<UnauthorizedHandler>();

export function onUnauthorized(handler: UnauthorizedHandler): () => void {
  unauthorizedHandlers.add(handler);
  return () => unauthorizedHandlers.delete(handler);
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  query?: Record<string, string | number | boolean | undefined | null>;
  signal?: AbortSignal;
}

function buildUrl(path: string, query?: RequestOptions['query']): string {
  if (!query) return path;
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null && value !== '') {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  return qs ? `${path}?${qs}` : path;
}

export async function apiRequest<T>(
  path: string,
  options: RequestOptions = {}
): Promise<T> {
  const { method = 'GET', body, query, signal } = options;

  const headers: Record<string, string> = {};
  let payload: BodyInit | undefined;
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }

  let res: Response;
  try {
    res = await fetch(buildUrl(`/web/api${path}`, query), {
      method,
      headers,
      body: payload,
      credentials: 'include',
      signal
    });
  } catch (err) {
    // Preserve abort semantics: an aborted request must keep its 'AbortError'
    // name so callers can distinguish a user-superseded request (e.g. a newer
    // search) from a genuine network failure and avoid showing an error.
    if (err instanceof Error && err.name === 'AbortError') {
      throw err;
    }
    throw new ApiError(
      err instanceof Error ? err.message : 'Network error',
      0
    );
  }

  if (res.status === 401) {
    unauthorizedHandlers.forEach((h) => h());
  }

  let envelope: ApiEnvelope<T> | null = null;
  const text = await res.text();
  if (text) {
    try {
      envelope = JSON.parse(text) as ApiEnvelope<T>;
    } catch {
      envelope = null;
    }
  }

  if (!res.ok || (envelope && envelope.success === false)) {
    const message =
      envelope?.message || `Request failed (${res.status})`;
    throw new ApiError(message, res.status);
  }

  return (envelope?.data ?? (undefined as T)) as T;
}

export const api = {
  get: <T>(path: string, query?: RequestOptions['query'], signal?: AbortSignal) =>
    apiRequest<T>(path, { method: 'GET', query, signal }),
  post: <T>(path: string, body?: unknown) =>
    apiRequest<T>(path, { method: 'POST', body }),
  patch: <T>(path: string, body?: unknown) =>
    apiRequest<T>(path, { method: 'PATCH', body }),
  put: <T>(path: string, body?: unknown) =>
    apiRequest<T>(path, { method: 'PUT', body }),
  delete: <T>(path: string) => apiRequest<T>(path, { method: 'DELETE' })
};
