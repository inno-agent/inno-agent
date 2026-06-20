import { useState } from 'react'
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

const tabs: { id: SettingsTab; label: string; icon: typeof Settings }[] = [
    { id: 'general', label: 'Общее', icon: Settings },
    { id: 'account', label: 'Аккаунт', icon: User },
    { id: 'personalization', label: 'Персонализация', icon: Sparkles },
]

export const SettingsDialog = ({ open, onOpenChange, email, onLogout }: SettingsDialogProps) => {
    const [activeTab, setActiveTab] = useState<SettingsTab>('general')

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent
                showCloseButton={false}
                overlayClassName="backdrop-blur-sm"
                className={styles.content}
            >
                <div className={styles.header}>
                    <span className={styles.title}>Настройки</span>
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
