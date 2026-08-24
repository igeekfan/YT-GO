import {backendMode, clearAuthToken, fetchWebConfig, getAuthToken, VerifyAuthToken} from './backend'

export type AuthStatus = 'checking' | 'connection-error' | 'login-required' | 'ready'

export interface AuthBootstrapDependencies {
    mode: 'desktop' | 'web'
    fetchConfig: typeof fetchWebConfig
    getToken: typeof getAuthToken
    verifyToken: typeof VerifyAuthToken
    clearToken: typeof clearAuthToken
}

const defaultDependencies: AuthBootstrapDependencies = {
    mode: backendMode,
    fetchConfig: fetchWebConfig,
    getToken: getAuthToken,
    verifyToken: VerifyAuthToken,
    clearToken: clearAuthToken,
}

export async function resolveAuthStatus(
    dependencies: AuthBootstrapDependencies = defaultDependencies,
): Promise<AuthStatus> {
    if (dependencies.mode === 'desktop') return 'ready'

    const config = await dependencies.fetchConfig()
    if (!config?.authRequired) {
        dependencies.clearToken()
        return 'ready'
    }
    if (!dependencies.getToken()) return 'login-required'

    try {
        await dependencies.verifyToken()
        return 'ready'
    } catch (error) {
        // apiFetch clears the token on 401. Preserve a valid stored token for
        // transient network/server failures so retry can recover the session.
        if ((error as {status?: number})?.status === 401 || !dependencies.getToken()) {
            dependencies.clearToken()
            return 'login-required'
        }
        throw error
    }
}
