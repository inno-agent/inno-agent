import { createRootRoute, Outlet } from '@tanstack/react-router'
import { Sidebar } from '../widgets/Sidebar'
import styles from './root.module.css'

export const Route = createRootRoute({
    component: () => (
        <div className={styles.layout}>
            <Sidebar />
            <main className={styles.main}>
                <Outlet />
            </main>
        </div>
    ),
})
