import * as SecureStore from 'expo-secure-store'
import * as AuthSession from 'expo-auth-session'
import * as WebBrowser from 'expo-web-browser'

WebBrowser.maybeCompleteAuthSession()

const TOKEN_KEY = 'dinks_id_token'

const issuer = process.env.EXPO_PUBLIC_OIDC_ISSUER ?? 'https://keycloak.internal.rayq.app/realms/home'
const clientId = process.env.EXPO_PUBLIC_OIDC_CLIENT_ID ?? 'dinks-mobile'

/**
 * Keycloak matches the incoming redirect_uri against the client's registered
 * "Valid Redirect URIs". `makeRedirectUri({ scheme })` on its own produces the
 * hostless `dinks://`, which can never equal a registered entry like
 * `dinks://oauthredirect`, so the authorization request is rejected with
 * "Invalid parameter: redirect_uri". Naming the path yields the concrete URI
 * that the `dinks-mobile` client must have registered.
 */
export const redirectUri = AuthSession.makeRedirectUri({ scheme: 'dinks', path: 'oauthredirect' })

/**
 * The OIDC ID token, when the member signed in with Keycloak. The backend
 * verifies a mobile bearer token by audience, expecting `dinks-mobile`; only the
 * ID token carries that, since a Keycloak access token's audience is "account".
 * Sending the access token here is what produced "expected audience
 * dinks-mobile got [account]" and a 401 on every request.
 */
export const token = () => SecureStore.getItemAsync(TOKEN_KEY)

/**
 * Keycloak authorization-code + PKCE, opened in the system browser. Returns
 * false when the member backs out, so the caller can stay on the sign-in screen
 * without treating a cancellation as an error.
 */
export async function signInWithKeycloak(): Promise<boolean> {
  const discovery = await AuthSession.fetchDiscoveryAsync(issuer)
  const request = new AuthSession.AuthRequest({
    clientId,
    redirectUri,
    responseType: AuthSession.ResponseType.Code,
    scopes: ['openid', 'profile', 'email'],
    usePKCE: true,
  })
  await request.makeAuthUrlAsync(discovery)
  const response = await request.promptAsync(discovery)
  if (response.type === 'cancel' || response.type === 'dismiss') return false
  if (response.type !== 'success') throw new Error('Sign in was not completed')
  const codeVerifier = request.codeVerifier
  if (!codeVerifier) throw new Error('Missing PKCE code verifier')
  const tokens = await AuthSession.exchangeCodeAsync(
    { clientId, code: response.params.code, redirectUri, extraParams: { code_verifier: codeVerifier } },
    discovery,
  )
  const idToken = tokens.idToken
  if (!idToken) throw new Error('No ID token returned')
  await SecureStore.setItemAsync(TOKEN_KEY, idToken)
  return true
}

/** Clears the Keycloak token. The session cookie is cleared by POST /auth/logout. */
export const signOut = async () => {
  await SecureStore.deleteItemAsync(TOKEN_KEY)
  // Remove a token left by older builds, which stored the wrong one.
  await SecureStore.deleteItemAsync('dinks_access_token')
}
