import { createRootRoute, Outlet } from '@tanstack/react-router'
import { MyRuntimeProvider } from '@libs/chat/runtime/MyRuntimeProvider'
import { Sidebar } from '../../../libs/sidebar'
import styles from './root.module.css'

export const Route = createRootRoute({
    component: () => (
        <MyRuntimeProvider chatId="temp-chat-id" userId="temp-user-id">
            <div className={`${styles.layout} dark`}>
                <Sidebar />
                <main className={styles.main}>
                    <Outlet />
                </main>
            </div>
        </MyRuntimeProvider>
    ),
})
