export default {
    settings: 'Settings',

    tabs: {
        general: 'General',
        account: 'Account',
        personalization: 'Personalization',
    },

    accountMenu: {
        settings: 'Settings',
        help: 'Help',
        logout: 'Log out',
    },

    general: {
        sectionTitle: 'General',
        appearance: 'Appearance',
        contrast: 'Contrast',
        language: 'Language',
        baseStyle: 'Base style and tone',
        defaultOption: 'Default',
        baseStyleOptions: {
            default:       { label: 'Default',        description: 'Preferred style and tone' },
            professional:  { label: 'Professional',   description: 'Tactful and precise' },
            friendly:      { label: 'Friendly',       description: 'Warm and conversational' },
            frank:         { label: 'Frank',          description: 'Direct and motivating' },
            quirky:        { label: 'Quirky',         description: 'Playful and creative' },
            efficient:     { label: 'Efficient',      description: 'Concise and clear' },
            cynical:       { label: 'Cynical',        description: 'Critical and sarcastic' },
        },
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
        languageOptions: {
            auto: 'Auto detect',
            ru: 'Русский',
            en: 'English',
        },
    },

    sidebar: {
        newChat: 'New chat',
        searchChat: 'Search chats',
        projects: 'Projects',
        recent: 'Recent',
        loading: 'Loading...',
        noChats: 'No chats yet',
        loadError: 'Failed to load chats',
        deleteError: 'Failed to delete chat',
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