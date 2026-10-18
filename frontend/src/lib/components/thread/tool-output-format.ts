export type ToolOutputBlock = {
  kind: 'text-block';
  value: string;
};

export function formatToolOutputBlock(output: string): ToolOutputBlock {
  const parsed = parseJsonString(output);
  if (!parsed.ok) return { kind: 'text-block', value: output };
  const readable = readableJsonValue(parsed.value);
  return { kind: 'text-block', value: readable || JSON.stringify(parsed.value, null, 2) };
}

function readableJsonValue(value: unknown, path = ''): string {
  if (typeof value === 'string') {
    return path ? `${path}:\n${value}` : value;
  }
  if (typeof value === 'number' || typeof value === 'boolean' || value === null) {
    return path ? `${path}: ${String(value)}` : String(value);
  }
  if (Array.isArray(value)) {
    return value
      .map((item, index) => readableJsonValue(item, path ? `${path}[${index}]` : `[${index}]`))
      .filter(Boolean)
      .join('\n\n');
  }
  if (value && typeof value === 'object') {
    return Object.entries(value)
      .map(([key, item]) => readableJsonValue(item, path ? `${path}.${key}` : key))
      .filter(Boolean)
      .join('\n\n');
  }
  return '';
}

function parseJsonString(value: string): { ok: true; value: unknown } | { ok: false } {
  const trimmed = value.trim();
  if (!((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']')))) {
    return { ok: false };
  }
  try {
    return { ok: true, value: JSON.parse(trimmed) };
  } catch {
    return { ok: false };
  }
}
