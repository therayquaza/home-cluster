import { api, type Fetcher } from '@dinks/shared'
export const baseURL = process.env.EXPO_PUBLIC_API_URL ?? 'https://dinks.internal.rayq.app'
export function mobileAPI(token: () => Promise<string | null>) {
  const fetcher: Fetcher = async (method, path, body) => {
    const accessToken = await token()
    const response = await fetch(baseURL + path, { method, headers: { ...(body ? {'Content-Type':'application/json'} : {}), ...(accessToken ? {Authorization:`Bearer ${accessToken}`} : {}) }, body: body ? JSON.stringify(body) : undefined })
    if (!response.ok) { const error = await response.json().catch(()=>({})); throw new Error(error.error || 'Request failed') }
    return response.status === 204 ? undefined : response.json()
  }
  return api(fetcher)
}
