export default {
    settings: 'Настройки',

    tabs: {
        general: 'Общее',
        account: 'Аккаунт',
        personalization: 'Персонализация',
    },

    general: {
        sectionTitle: 'Общее',
        appearance: 'Внешний вид',
        contrast: 'Контраст',
        accentColor: 'Акцентный цвет',
        language: 'Язык',
        appearanceOptions: {
            system: 'Системный',
            dark: 'Тёмный',
            light: 'Светлый',
        },
        contrastOptions: {
            system: 'Системный',
            standard: 'Стандартный',
            high: 'Высокий',
        },
        accentColorOptions: {
            default: 'По умолчанию',
            blue: 'Синий',
            purple: 'Фиолетовый',
            green: 'Зелёный',
            yellow: 'Жёлтый',
            orange: 'Оранжевый',
        },
        languageOptions: {
            auto: 'Автоматическое определение',
            ru: 'Русский',
            en: 'English',
        },
    },

    personalization: {
        sectionTitle: 'Персонализация',
        baseStyle: 'Базовый стиль и тон',
        headings: 'Заголовки и списки',
        emoji: 'Эмодзи',
        defaultOption: 'По умолчанию',
        aboutYou: 'О вас',
        nickname: 'Псевдоним',
        nicknamePlaceholder: 'Как бы вы хотели, чтобы InnoAgent обращался к вам?',
        profession: 'Профессия',
        professionPlaceholder: 'Дизайнер интерьеров',
        instructions: 'Инструкции для InnoAgent',
        instructionsPlaceholder: 'Например, задавай уточняющие вопросы, прежде чем давать подробные ответы',
        instructionsHint: 'InnoAgent будет помнить об этом в чатах в соответствии с',
        instructionsHintLink: 'протоколом',
        memory: 'Память',
        memoryManage: 'Управление',
        memoryUse: 'Ссылаться на сохраненную память',
        memoryUseDesc: 'Позволяет InnoAgent сохранять и использовать память при ответе',
        chatHistory: 'Ссылаться на историю чата',
        chatHistoryDesc: 'Позволяет InnoAgent ссылаться на недавние обсуждения при ответе',
    },
} as const