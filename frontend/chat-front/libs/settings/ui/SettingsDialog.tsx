import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { X, Settings, User, Sparkles } from 'lucide-react'
import { Dialog, DialogContent } from '@shared/ui/dialog'
import type { SettingsTab } from '@libs/settings/model/types'
import { GeneralTab } from './tabs/GeneralTab'
import { AccountTab } from './tabs/AccountTab'
import { PersonalizationTab } from './tabs/PersonalizationTab'
import styles from './SettingsDialog.module.css'

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
                className={styles.content}
            >
                <div className={styles.header}>
                    <span className={styles.title}>{t('settings')}</span>
                    <button className={styles.close} onClick={() => onOpenChange(false)}>
                        <X />
                    </button>
                </div>

                <div className={styles.body}>
                    <nav className={styles.tabs}>
                        {tabs.map(({ id, label, icon: Icon }) => (
                            <button
                                key={id}
                                className={[styles.tab, activeTab === id ? styles.tabActive : ''].join(' ')}
                                onClick={() => setActiveTab(id)}
                            >
                                <Icon className={styles.tabIcon} />
                                {label}
                            </button>
                        ))}
                    </nav>

                    <div className={styles.panel}>
                        {activeTab === 'general' && <GeneralTab />}
                        {activeTab === 'account' && <AccountTab email={email} onLogout={onLogout} />}
                        {activeTab === 'personalization' && <PersonalizationTab />}
                    </div>
                </div>
            </DialogContent>
        </Dialog>
    )
}
