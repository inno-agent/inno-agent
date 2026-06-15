export interface ModelOption {
    id: string
    label: string
    description: string
}

export const AVAILABLE_MODELS: ModelOption[] = [
    {
        id: 'llama3.2:3b',
        label: 'General',
        description: 'General Q&A and conversation',
    },
    {
        id: 'qwen2.5-coder:7b',
        label: 'Code',
        description: 'Programming, debugging, code generation',
    },
    {
        id: 'deepseek-r1:8b',
        label: 'Math',
        description: 'Mathematics and complex reasoning',
    },
]

export const DEFAULT_MODEL_ID = AVAILABLE_MODELS[0].id
