const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '').toString().replace(/\/+$/, '')

type JsonValue = null | boolean | number | string | JsonValue[] | { [key: string]: JsonValue }

async function readJsonSafe(res: Response): Promise<JsonValue | null> {
  const text = await res.text().catch(() => '')
  if (!text) return null
  try {
    return JSON.parse(text) as JsonValue
  } catch {
    return null
  }
}

function pickMsg(data: JsonValue | null): string | null {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  const msg = (data as Record<string, JsonValue>).msg
  return typeof msg === 'string' ? msg : null
}

export async function postJSON<T>(path: string, body: unknown, token?: string): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      ...(token ? { authorization: `Bearer ${token}` } : null),
    },
    body: JSON.stringify(body),
  })

  const data = await readJsonSafe(res)

  if (!res.ok) {
    throw new Error(pickMsg(data) ?? `Request failed (${res.status})`)
  }

  return data as T
}
