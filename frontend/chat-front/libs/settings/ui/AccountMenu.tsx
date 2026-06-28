import { useTranslation } from 'react-i18next'
import { Settings, CircleHelp, LogOut } from 'lucide-react'
import styles from './AccountMenu.module.css'

interface AccountMenuProps {
    email: string
    onOpenSettings: () => void
    onLogout: () => void
}

export const AccountMenu = ({ email, onOpenSettings, onLogout }: AccountMenuProps) => {
    const { t } = useTranslation()
    return (
        <div className={styles.menu}>
            <div className={styles.email}>{email}</div>

            <div className={styles.divider} />

            <button className={styles.item} onClick={onOpenSettings}>
                <Settings className={styles.icon} />
                {t('accountMenu.settings')}
            </button>
            <button className={styles.item}>
                <CircleHelp className={styles.icon} />
                {t('accountMenu.help')}
            </button>

            <div className={styles.divider} />

            <button className={styles.item} onClick={onLogout}>
                <LogOut className={styles.icon} />
                {t('accountMenu.logout')}
            </button>
        </div>
    )
}
