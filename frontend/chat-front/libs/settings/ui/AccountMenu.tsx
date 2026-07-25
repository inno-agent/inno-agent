import { useTranslation } from 'react-i18next'
import { Settings, CircleHelp, LogOut } from 'lucide-react'
import styles from './AccountMenu.module.scss'

interface AccountMenuProps {
    email: string
    onOpenSettings: () => void
    onLogout: () => void
}

export const AccountMenu = ({ email, onOpenSettings, onLogout }: AccountMenuProps) => {
    const { t } = useTranslation()
    return (
        <div className={styles['account-menu']}>
            <div className={styles['account-menu__email']}>{email}</div>

            <div className={styles['account-menu__divider']} />

            <button className={styles['account-menu__item']} onClick={onOpenSettings}>
                <Settings className={styles['account-menu__icon']} />
                {t('accountMenu.settings')}
            </button>
            <button className={styles['account-menu__item']}>
                <CircleHelp className={styles['account-menu__icon']} />
                {t('accountMenu.help')}
            </button>

            <div className={styles['account-menu__divider']} />

            <button className={styles['account-menu__item']} onClick={onLogout}>
                <LogOut className={styles['account-menu__icon']} />
                {t('accountMenu.logout')}
            </button>
        </div>
    )
}
