import { api } from '@dinks/shared'

// Parses the body as JSON unless it's empty — a 201 Created with no body (as the backend returns
// for creates) has Content-Length: 0, and calling .json() on empty text throws a SyntaxError.
async function parseBody<T>(response: Response): Promise<T> {
  const text = await response.text()
  return (text ? JSON.parse(text) : undefined) as T
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, { method, credentials: 'include', headers: body ? { 'Content-Type': 'application/json' } : undefined, body: body ? JSON.stringify(body) : undefined })
  if (!response.ok) {
    const error = await parseBody<{ error?: string }>(response).catch(() => ({}) as { error?: string })
    throw new Error(error.error || 'Request failed')
  }
  return parseBody<T>(response)
}

async function authedRequest<T>(method: string, path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, { method, credentials: 'include', headers: body ? { 'Content-Type': 'application/json' } : undefined, body: body ? JSON.stringify(body) : undefined })
  if (response.status === 401) {
    window.location.assign('/auth/login')
    throw new Error('Sign in required')
  }
  if (!response.ok) {
    const error = await parseBody<{ error?: string }>(response).catch(() => ({}) as { error?: string })
    throw new Error(error.error || 'Request failed')
  }
  return parseBody<T>(response)
}

export const client = api(authedRequest)
export const publicClient = api(request)
