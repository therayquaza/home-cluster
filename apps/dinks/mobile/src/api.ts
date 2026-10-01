import { api, type Fetcher } from '@dinks/shared'
import { token } from './auth'

export const baseURL = process.env.EXPO_PUBLIC_API_URL ?? 'https://dinks.internal.rayq.app'

// A 401 means the session is gone (cookie expired, token revoked). Web redirects
// to /auth/login; a native app has no address bar, so it hands control back to
// the shell, which drops to the sign-in screen.
let onUnauthorized: () => void = () => {}
export function setUnauthorizedHandler(handler: () => void) {
  onUnauthorized = handler
}

// A 201 Created with no body (what the backend returns for creates) has
// Content-Length: 0, and calling .json() on empty text throws a SyntaxError.
async function parseBody<T>(response: Response): Promise<T> {
  const text = await response.text()
  return (text ? JSON.parse(text) : undefined) as T
}

function makeFetcher(authed: boolean): Fetcher {
  return async <T>(method: string, path: string, body?: unknown): Promise<T> => {
    // Read the token per request so a sign-in that happens after this module is
    // imported is still picked up.
    const accessToken = authed ? await token() : null
    const response = await fetch(baseURL + path, {
      method,
      // The email/password and Keycloak callback flows both set an scs session
      // cookie; React Native's fetch persists it, which is what lets the mobile
      // client mirror the web session flow.
      credentials: 'include',
      headers: {
        ...(body ? { 'Content-Type': 'application/json' } : {}),
        ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    })
    if (authed && response.status === 401) {
      onUnauthorized()
      throw new Error('Sign in required')
    }
    if (!response.ok) {
      const error = await parseBody<{ error?: string }>(response).catch(() => ({}) as { error?: string })
      throw new Error(error.error || 'Request failed')
    }
    return parseBody<T>(response)
  }
}

/** The API surface the screens receive, so they can be typed without the singleton. */
export type ApiClient = ReturnType<typeof api>

export const client: ApiClient = api(makeFetcher(true))
export const publicClient: ApiClient = api(makeFetcher(false))
