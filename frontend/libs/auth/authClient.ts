import { UserManager, type UserManagerSettings } from 'oidc-client-ts'

export async function createUserManager(): Promise<UserManager> {
    const resp = await fetch('/identity/v1/config')
    const { authority, client_id, authorization_endpoint } = await resp.json()

    const settings: UserManagerSettings = {
        authority,
        client_id,
        redirect_uri: window.location.origin + '/callback',
        scope: 'openid email profile',
        response_type: 'code',
        // Proxy Zitadel OIDC endpoints through auth service (HTTPS) to avoid
        // mixed-content blocks when the app is on HTTPS but Zitadel is on HTTP.
        metadata: {
            issuer: authority,
            authorization_endpoint,
            token_endpoint: window.location.origin + '/identity/v1/oidc/token',
            jwks_uri: window.location.origin + '/identity/v1/oidc/jwks',
        },
    }

    return new UserManager(settings)
}
