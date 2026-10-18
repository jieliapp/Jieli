export interface ExpiringStateStorage {
  getItem: (key: string) => string | null;
  setItem: (key: string, value: string) => void;
  removeItem: (key: string) => void;
}

export interface ExpiringStateEnvelope<T> {
  state: T;
  createdAtMs: number;
}

export function saveExpiringState<T>(storage: ExpiringStateStorage, prefix: string, id: string, state: T): void {
  const envelope: ExpiringStateEnvelope<T> = {
    state,
    createdAtMs: Date.now()
  };
  storage.setItem(expiringStateKey(prefix, id), JSON.stringify(envelope));
}

export function loadExpiringState<T>(
  storage: ExpiringStateStorage,
  prefix: string,
  id: string,
  ttlMs: number
): T | null {
  const key = expiringStateKey(prefix, id);
  const raw = storage.getItem(key);
  if (!raw) return null;
  try {
    const envelope = JSON.parse(raw) as Partial<ExpiringStateEnvelope<T>>;
    if (typeof envelope.createdAtMs !== 'number' || !Number.isFinite(envelope.createdAtMs)) {
      throw new Error('created_at_ms is required');
    }
    if (Date.now() - envelope.createdAtMs > ttlMs) {
      storage.removeItem(key);
      return null;
    }
    if (!Object.prototype.hasOwnProperty.call(envelope, 'state') && Object.keys(envelope).length > 1) {
      return envelope as T;
    }
    if (envelope.state === undefined || envelope.state === null) {
      storage.removeItem(key);
      return null;
    }
    return envelope.state;
  } catch {
    storage.removeItem(key);
    return null;
  }
}

export function loadValidatedExpiringState<T, R>(
  storage: ExpiringStateStorage,
  prefix: string,
  id: string,
  ttlMs: number,
  validate: (state: T) => R | null
): R | null {
  const state = loadExpiringState<T>(storage, prefix, id, ttlMs);
  if (state === null) return null;
  const validated = validate(state);
  if (validated === null) {
    clearExpiringState(storage, prefix, id);
    return null;
  }
  return validated;
}

export function clearExpiringState(storage: ExpiringStateStorage, prefix: string, id: string): void {
  storage.removeItem(expiringStateKey(prefix, id));
}

function expiringStateKey(prefix: string, id: string): string {
  return `${prefix}${id}`;
}
