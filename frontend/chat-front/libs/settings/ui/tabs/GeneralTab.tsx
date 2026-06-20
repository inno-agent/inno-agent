import { useState } from 'react'
import { SettingsRow, SettingsSectionTitle, SettingsSelect } from '@libs/settings/ui/rows/SettingsRow'

const appearanceOptions = ['Системный', 'Тёмный', 'Светлый']
const contrastOptions = ['Системный', 'Стандартный', 'Высокий']
const accentColorOptions = ['По умолчанию', 'Синий', 'Фиолетовый', 'Зелёный']
const languageOptions = ['Автоматическое определение', 'Русский', 'English']

export const GeneralTab = () => {
    const [appearance, setAppearance] = useState(appearanceOptions[0])
    const [contrast, setContrast] = useState(contrastOptions[0])
    const [accentColor, setAccentColor] = useState(accentColorOptions[0])
    const [language, setLanguage] = useState(languageOptions[0])

    return (
        <>
            <SettingsSectionTitle>Общее</SettingsSectionTitle>

            <SettingsRow label="Внешний вид">
                <SettingsSelect value={appearance} options={appearanceOptions} onChange={setAppearance} />
            </SettingsRow>

            <SettingsRow label="Контраст">
                <SettingsSelect value={contrast} options={contrastOptions} onChange={setContrast} />
            </SettingsRow>

            <SettingsRow label="Акцентный цвет">
                <SettingsSelect value={accentColor} options={accentColorOptions} onChange={setAccentColor} />
            </SettingsRow>

            <SettingsRow label="Язык">
                <SettingsSelect value={language} options={languageOptions} onChange={setLanguage} />
            </SettingsRow>
        </>
    )
}
