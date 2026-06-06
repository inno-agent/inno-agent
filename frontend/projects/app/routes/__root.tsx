import { createRootRoute, Outlet } from '@tanstack/react-router'
import { AuthProvider } from '@libs/auth/AuthProvider'
import { MyRuntimeProvider } from '@libs/chat/runtime/MyRuntimeProvider'
import { Sidebar } from '../../../libs/sidebar'
import styles from './root.module.css'

export const Route = createRootRoute({
    component: () => (
        <AuthProvider>
            <MyRuntimeProvider>
                <div className={`${styles.layout} dark`}>
                    <Sidebar />
                    <main className={styles.main}>
                        <Outlet />
                    </main>
                </div>
            </MyRuntimeProvider>
        </AuthProvider>
    ),
})
