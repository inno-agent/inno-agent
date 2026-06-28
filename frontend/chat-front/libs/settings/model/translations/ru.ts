export default {
    settings: 'Настройки',

    tabs: {
        general: 'Общее',
        account: 'Аккаунт',
        personalization: 'Персонализация',
    },

    accountMenu: {
        settings: 'Настройки',
        help: 'Помощь',
        logout: 'Выйти',
    },

    general: {
        sectionTitle: 'Общее',
        appearance: 'Внешний вид',
        contrast: 'Контраст',
        language: 'Язык',
        baseStyle: 'Базовый стиль и тон',
        defaultOption: 'По умолчанию',
        baseStyleOptions: {
            default:       { label: 'По умолчанию',   description: 'Предпочитаемый стиль и тон' },
            professional:  { label: 'Профессиональный', description: 'Тактичный и точный' },
            friendly:      { label: 'Дружелюбный',     description: 'Тёплый и разговорчивый' },
            frank:         { label: 'Откровенный',     description: 'Прямой и мотивирующий' },
            quirky:        { label: 'Причудливый',     description: 'Веселый и творческий' },
            efficient:     { label: 'Эффективный',     description: 'Немногословный и четкий' },
            cynical:       { label: 'Циничный',        description: 'Критикующий и саркастичный' },
        },
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
        languageOptions: {
            auto: 'Автоматическое определение',
            ru: 'Русский',
            en: 'English',
        },
    },

    sidebar: {
        newChat: 'Новый чат',
        searchChat: 'Искать чат',
        projects: 'Проекты',
        recent: 'Недавние',
        loading: 'Загрузка...',
        noChats: 'Чатов пока нет',
        loadError: 'Не удалось загрузить чаты',
        deleteError: 'Не удалось удалить чат',
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