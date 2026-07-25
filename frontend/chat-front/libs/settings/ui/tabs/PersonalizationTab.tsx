import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@shared/ui/button'
import { Switch } from '@shared/ui/switch'
import { SettingsRow, SettingsSectionTitle } from '@libs/settings/ui/rows/SettingsRow'
import styles from './PersonalizationTab.module.scss'

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

            <div className={styles['personalization-tab__field']}>
                <label className={styles['personalization-tab__field-label']}>{t('personalization.nickname')}</label>
                <input
                    className={styles['personalization-tab__input']}
                    placeholder={t('personalization.nicknamePlaceholder')}
                    value={nickname}
                    onChange={(e) => setNickname(e.target.value)}
                />
            </div>

            <div className={styles['personalization-tab__field']}>
                <label className={styles['personalization-tab__field-label']}>{t('personalization.profession')}</label>
                <input
                    className={styles['personalization-tab__input']}
                    placeholder={t('personalization.professionPlaceholder')}
                    value={profession}
                    onChange={(e) => setProfession(e.target.value)}
                />
            </div>

            <div className={styles['personalization-tab__field']}>
                <label className={styles['personalization-tab__field-label']}>{t('personalization.instructions')}</label>
                <span className={styles['personalization-tab__field-hint']}>
                    {t('personalization.instructionsHint')} <a href="#" className={styles['personalization-tab__link']}>{t('personalization.instructionsHintLink')}</a>
                </span>
                <textarea
                    className={styles['personalization-tab__textarea']}
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
