import { Settings, CircleHelp, LogOut } from 'lucide-react'
import styles from './AccountMenu.module.css'

interface AccountMenuProps {
    email: string
    onOpenSettings: () => void
    onLogout: () => void
}

export const AccountMenu = ({ email, onOpenSettings, onLogout }: AccountMenuProps) => {
    return (
        <div className={styles.menu}>
            <div className={styles.email}>{email}</div>

            <div className={styles.divider} />

            <button className={styles.item} onClick={onOpenSettings}>
                <Settings className={styles.icon} />
                Настройки
            </button>
            <button className={styles.item}>
                <CircleHelp className={styles.icon} />
                Помощь
            </button>

            <div className={styles.divider} />

            <button className={styles.item} onClick={onLogout}>
                <LogOut className={styles.icon} />
                Выйти
            </button>
        </div>
    )
}
