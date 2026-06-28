/* eslint-disable react-refresh/only-export-components */
import { createRootRoute, Outlet } from '@tanstack/react-router'
import { SettingsProvider } from '@libs/settings/SettingsProvider'
import { AuthProvider } from '@libs/auth/AuthProvider'
import { useAuth } from '@libs/auth/useAuth'
import { Sidebar } from '../../../libs/sidebar'
import styles from './root.module.css'

function AppShell() {
    const { loading, token } = useAuth()
    const isCallback = window.location.pathname === '/callback'

    if (isCallback && !token) {
        return <Outlet />
    }

    if (loading || !token) {
        return (
            <main className={styles.main}>
                <div>Авторизация...</div>
            </main>
        )
    }

    return (
        <div className={`${styles.layout}`}>
            <Sidebar />
            <main className={styles.main}>
                <Outlet />
            </main>
        </div>
    )
}

export const Route = createRootRoute({
    component: () => (
        <SettingsProvider>
            <AuthProvider>
                <AppShell />
            </AuthProvider>
        </SettingsProvider>
    ),
})
