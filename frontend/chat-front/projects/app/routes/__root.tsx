/* eslint-disable react-refresh/only-export-components */
import { createRootRoute, Outlet } from '@tanstack/react-router'
import { SettingsProvider } from '@libs/settings/SettingsProvider'
import { AuthProvider } from '@libs/auth/AuthProvider'
import { useAuth } from '@libs/auth/useAuth'
import { Sidebar } from '../../../libs/sidebar'
import styles from './root.module.css'
import { useState } from 'react'

function AppShell() {
    const { loading, token } = useAuth()
    const isCallback = window.location.pathname === '/callback'
    const [sidebarOpen, setSidebarOpen] = useState(true)

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
            <button
                className={styles.menuButton}
                onClick={() => setSidebarOpen(true)}
                aria-label="Открыть меню"
            >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <line x1="3" y1="12" x2="21" y2="12" />
                    <line x1="3" y1="6" x2="21" y2="6" />
                    <line x1="3" y1="18" x2="21" y2="18" />
                </svg>
            </button>
            <Sidebar isOpen={sidebarOpen} onClose={() => setSidebarOpen(false)} />
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
