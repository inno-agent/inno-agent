export interface Chat {
    id: number;
    title: string;
    createdAt: string;
    projectId?: number;
}

export const mockChats: Chat[] = [
    { id: 1, title: 'Как настроить CI/CD', createdAt: '2026-06-01' },
    { id: 2, title: 'Баг в авторизации', createdAt: '2026-06-02' },
    { id: 3, title: 'Код ревью PR #42', createdAt: '2026-06-03' },
]