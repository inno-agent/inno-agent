export default {
    settings: 'Settings',

    tabs: {
        general: 'General',
        account: 'Account',
        personalization: 'Personalization',
    },

    general: {
        sectionTitle: 'General',
        appearance: 'Appearance',
        contrast: 'Contrast',
        accentColor: 'Accent color',
        language: 'Language',
        appearanceOptions: {
            system: 'System',
            dark: 'Dark',
            light: 'Light',
        },
        contrastOptions: {
            system: 'System',
            standard: 'Standard',
            high: 'High',
        },
        accentColorOptions: {
            default: 'Default',
            blue: 'Blue',
            purple: 'Purple',
            green: 'Green',
            yellow: 'Yellow',
            orange: 'Orange',
        },
        languageOptions: {
            auto: 'Auto detect',
            ru: 'Русский',
            en: 'English',
        },
    },

    personalization: {
        sectionTitle: 'Personalization',
        baseStyle: 'Base style and tone',
        headings: 'Headings and lists',
        emoji: 'Emoji',
        defaultOption: 'Default',
        aboutYou: 'About you',
        nickname: 'Nickname',
        nicknamePlaceholder: 'How would you like InnoAgent to address you?',
        profession: 'Profession',
        professionPlaceholder: 'Interior designer',
        instructions: 'Instructions for InnoAgent',
        instructionsPlaceholder: 'For example, ask clarifying questions before giving detailed answers',
        instructionsHint: 'InnoAgent will keep this in mind in chats according to the',
        instructionsHintLink: 'protocol',
        memory: 'Memory',
        memoryManage: 'Manage',
        memoryUse: 'Reference saved memory',
        memoryUseDesc: 'Allows InnoAgent to save and use memory when responding',
        chatHistory: 'Reference chat history',
        chatHistoryDesc: 'Allows InnoAgent to reference recent discussions when responding',
    },
} as const