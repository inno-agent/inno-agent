import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@shared/ui/button'
import { Switch } from '@shared/ui/switch'
import { SettingsRow, SettingsSectionTitle } from '@libs/settings/ui/rows/SettingsRow'
import styles from './PersonalizationTab.module.css'

export const PersonalizationTab = () => {
    const { t } = useTranslation()
    const [nickname, setNickname] = useState('')
    const [profession, setProfession] = useState('')
    const [instructions, setInstructions] = useState('')
    const [useSavedMemory, setUseSavedMemory] = useState(true)
    const [useChatHistory, setUseChatHistory] = useState(true)

    return (
        <>
            <SettingsSectionTitle>{t('personalization.aboutYou')}</SettingsSectionTitle>

            <div className={styles.field}>
                <label className={styles.fieldLabel}>{t('personalization.nickname')}</label>
                <input
                    className={styles.input}
                    placeholder={t('personalization.nicknamePlaceholder')}
                    value={nickname}
                    onChange={(e) => setNickname(e.target.value)}
                />
            </div>

            <div className={styles.field}>
                <label className={styles.fieldLabel}>{t('personalization.profession')}</label>
                <input
                    className={styles.input}
                    placeholder={t('personalization.professionPlaceholder')}
                    value={profession}
                    onChange={(e) => setProfession(e.target.value)}
                />
            </div>

            <div className={styles.field}>
                <label className={styles.fieldLabel}>{t('personalization.instructions')}</label>
                <span className={styles.fieldHint}>
                    {t('personalization.instructionsHint')} <a href="#" className={styles.link}>{t('personalization.instructionsHintLink')}</a>
                </span>
                <textarea
                    className={styles.textarea}
                    placeholder={t('personalization.instructionsPlaceholder')}
                    value={instructions}
                    onChange={(e) => setInstructions(e.target.value)}
                />
            </div>

            <SettingsRow label={t('personalization.memory')}>
                <Button variant="outline" size="sm">
                    {t('personalization.memoryManage')}
                </Button>
            </SettingsRow>

            <SettingsRow
                label={t('personalization.memoryUse')}
                description={t('personalization.memoryUseDesc')}
            >
                <Switch checked={useSavedMemory} onCheckedChange={setUseSavedMemory} />
            </SettingsRow>

            <SettingsRow
                label={t('personalization.chatHistory')}
                description={t('personalization.chatHistoryDesc')}
            >
                <Switch checked={useChatHistory} onCheckedChange={setUseChatHistory} />
            </SettingsRow>
        </>
    )
}
