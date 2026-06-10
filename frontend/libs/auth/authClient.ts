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
        // Token/JWKS go through the identity service proxy so the browser
        // only talks to our origin; authorize is a top-level redirect to the IdP.
        metadata: {
            issuer: authority,
            authorization_endpoint,
            token_endpoint: window.location.origin + '/identity/v1/oidc/token',
            jwks_uri: window.location.origin + '/identity/v1/oidc/jwks',
        },
    }

    return new UserManager(settings)
}
