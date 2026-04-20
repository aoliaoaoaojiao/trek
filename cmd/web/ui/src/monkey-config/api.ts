const DEV_API_BASE = "http://127.0.0.1:17888"

const API_BASE = import.meta.env.DEV
  ? DEV_API_BASE
  : `${window.location.protocol}//${window.location.host}`

export { API_BASE, DEV_API_BASE }

export async function postJSON<T>(path: string, payload: unknown): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
  const data = (await response.json()) as T & { error?: string }
  if (!response.ok) {
    throw new Error(data.error ?? `请求失败: ${response.status}`)
  }
  return data
}

