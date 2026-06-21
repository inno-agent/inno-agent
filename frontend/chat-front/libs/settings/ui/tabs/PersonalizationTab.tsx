import { useState } from 'react'
import { Button } from '@shared/ui/button'
import { Switch } from '@shared/ui/switch'
import { SettingsRow, SettingsSectionTitle, SettingsSelect } from '@libs/settings/ui/rows/SettingsRow'
import styles from './PersonalizationTab.module.css'

const defaultOptions = ['По умолчанию']

export const PersonalizationTab = () => {
    const [baseStyle, setBaseStyle] = useState(defaultOptions[0])
    const [headingsStyle, setHeadingsStyle] = useState(defaultOptions[0])
    const [emojiStyle, setEmojiStyle] = useState(defaultOptions[0])
    const [nickname, setNickname] = useState('')
    const [profession, setProfession] = useState('')
    const [instructions, setInstructions] = useState('')
    const [useSavedMemory, setUseSavedMemory] = useState(true)
    const [useChatHistory, setUseChatHistory] = useState(true)

    return (
        <>
            <SettingsSectionTitle>Персонализация</SettingsSectionTitle>

            <SettingsRow label="Базовый стиль и тон">
                <SettingsSelect value={baseStyle} options={defaultOptions} onChange={setBaseStyle} />
            </SettingsRow>

            <SettingsRow label="Заголовки и списки">
                <SettingsSelect value={headingsStyle} options={defaultOptions} onChange={setHeadingsStyle} />
            </SettingsRow>

            <SettingsRow label="Эмодзи">
                <SettingsSelect value={emojiStyle} options={defaultOptions} onChange={setEmojiStyle} />
            </SettingsRow>

            <SettingsSectionTitle>О вас</SettingsSectionTitle>

            <div className={styles.field}>
                <label className={styles.fieldLabel}>Псевдоним</label>
                <input
                    className={styles.input}
                    placeholder="Как бы вы хотели, чтобы InnoAgent обращался к вам?"
                    value={nickname}
                    onChange={(e) => setNickname(e.target.value)}
                />
            </div>

            <div className={styles.field}>
                <label className={styles.fieldLabel}>Профессия</label>
                <input
                    className={styles.input}
                    placeholder="Дизайнер интерьеров"
                    value={profession}
                    onChange={(e) => setProfession(e.target.value)}
                />
            </div>

            <div className={styles.field}>
                <label className={styles.fieldLabel}>Инструкции для InnoAgent</label>
                <span className={styles.fieldHint}>
                    InnoAgent будет помнить об этом в чатах в соответствии с <a href="#" className={styles.link}>протоколом</a>
                </span>
                <textarea
                    className={styles.textarea}
                    placeholder="Например, задавай уточняющие вопросы, прежде чем давать подробные ответы"
                    value={instructions}
                    onChange={(e) => setInstructions(e.target.value)}
                />
            </div>

            <SettingsRow label="Память">
                <Button variant="outline" size="sm">
                    Управление
                </Button>
            </SettingsRow>

            <SettingsRow
                label="Ссылаться на сохраненную память"
                description="Позволяет InnoAgent сохранять и использовать память при ответе"
            >
                <Switch checked={useSavedMemory} onCheckedChange={setUseSavedMemory} />
            </SettingsRow>

            <SettingsRow
                label="Ссылаться на историю чата"
                description="Позволяет InnoAgent ссылаться на недавние обсуждения при ответе"
            >
                <Switch checked={useChatHistory} onCheckedChange={setUseChatHistory} />
            </SettingsRow>
        </>
    )
}
