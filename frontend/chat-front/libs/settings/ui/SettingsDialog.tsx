import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { X, Settings, User, Sparkles } from 'lucide-react'
import { Dialog, DialogContent } from '@shared/ui/dialog'
import type { SettingsTab } from '@libs/settings/model/types'
import { GeneralTab } from './tabs/GeneralTab'
import { AccountTab } from './tabs/AccountTab'
import { PersonalizationTab } from './tabs/PersonalizationTab'
import styles from './SettingsDialog.module.scss'

interface SettingsDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    email: string
    onLogout: () => void
}

const tabIcons: Record<SettingsTab, typeof Settings> = {
    general: Settings,
    account: User,
    personalization: Sparkles,
}

export const SettingsDialog = ({ open, onOpenChange, email, onLogout }: SettingsDialogProps) => {
    const [activeTab, setActiveTab] = useState<SettingsTab>('general')
    const { t } = useTranslation()

    const tabs: { id: SettingsTab; label: string; icon: typeof Settings }[] = [
        { id: 'general', label: t('tabs.general'), icon: tabIcons.general },
        { id: 'account', label: t('tabs.account'), icon: tabIcons.account },
        { id: 'personalization', label: t('tabs.personalization'), icon: tabIcons.personalization },
    ]

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent
                showCloseButton={false}
                overlayClassName="backdrop-blur-sm"
                className={styles['settings-dialog__content']}
            >
                <div className={styles['settings-dialog__header']}>
                    <span className={styles['settings-dialog__title']}>{t('settings')}</span>
                    <button className={styles['settings-dialog__close']} onClick={() => onOpenChange(false)}>
                        <X />
                    </button>
                </div>

                <div className={styles['settings-dialog__body']}>
                    <nav className={styles['settings-dialog__tabs']}>
                        {tabs.map(({ id, label, icon: Icon }) => (
                            <button
                                key={id}
                                className={[styles['settings-dialog__tab'], activeTab === id ? styles['settings-dialog__tab--active'] : ''].join(' ')}
                                onClick={() => setActiveTab(id)}
                            >
                                <Icon className={styles['settings-dialog__tab-icon']} />
                                {label}
                            </button>
                        ))}
                    </nav>

                    <div className={styles['settings-dialog__panel']}>
                        {activeTab === 'general' && <GeneralTab />}
                        {activeTab === 'account' && <AccountTab email={email} onLogout={onLogout} />}
                        {activeTab === 'personalization' && <PersonalizationTab />}
                    </div>
                </div>
            </DialogContent>
        </Dialog>
    )
}
