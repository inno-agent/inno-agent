import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SettingsRow, SettingsSectionTitle, SettingsSelect } from '@libs/settings/ui/rows/SettingsRow'
import { useSettings } from '@libs/settings/model/useSettings'
import i18n from '@libs/settings/model/i18n'
import type {Theme} from "@libs/settings/model/settingsContext"


export const GeneralTab = () => {
    const { theme, setTheme } = useSettings()
    const { t } = useTranslation()
    const themeOptions: Record<string, Theme> = {
        [t('general.appearanceOptions.system')]: 'system',
        [t('general.appearanceOptions.dark')]: 'dark',
        [t('general.appearanceOptions.light')]: 'light',
    }
    const langOptions: Record<string, string> = {
        [t('general.languageOptions.auto')]: navigator.language.startsWith('en') ? 'en' : 'ru',
        [t('general.languageOptions.ru')]: 'ru',
        [t('general.languageOptions.en')]: 'en',
    }
    const baseStyleOptions = (['default', 'professional', 'friendly', 'frank', 'quirky', 'efficient', 'cynical'] as const).map(
        (k) => t(`general.baseStyleOptions.${k}`, { returnObjects: true }) as { label: string; description: string }
    )
    const [baseStyle, setBaseStyle] = useState(baseStyleOptions[0].label)
    const [language, setLanguage] = useState(
        i18n.language === 'en' ? t('general.languageOptions.en') : t('general.languageOptions.ru')
    )
    const themeLabel = Object.entries(themeOptions).find(([, v]) => v === theme)?.[0] ?? t('general.appearanceOptions.system')
    const handleThemeChange = (label: string) => {
        setTheme(themeOptions[label] as Theme)
    }
    const handleLanguageChange = (label: string) => {
        setLanguage(label)
        i18n.changeLanguage(langOptions[label])
        localStorage.setItem('lang', langOptions[label])
    }

    return (
        <>
            <SettingsSectionTitle>{t('general.sectionTitle')}</SettingsSectionTitle>

            <SettingsRow label={t('general.appearance')}>
                <SettingsSelect value={themeLabel} options={Object.keys(themeOptions)} onChange={handleThemeChange} />
            </SettingsRow>

<SettingsRow label={t('general.language')}>
                <SettingsSelect value={language} options={Object.keys(langOptions)} onChange={handleLanguageChange} />
            </SettingsRow>

            <SettingsRow label={t('general.baseStyle')}>
                <SettingsSelect value={baseStyle} options={baseStyleOptions} onChange={setBaseStyle} />
            </SettingsRow>
        </>
    )
}
