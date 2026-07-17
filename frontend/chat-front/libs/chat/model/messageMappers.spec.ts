import { describe, it, expect } from 'vitest'
import {
    createUserTextMessage,
    fromApiMessage,
    toChatRequestMessages,
    upsertAssistantText,
    appendAssistantError,
} from './messageMappers'
import type { Message } from './types'

describe('messageMappers', () => {
    describe('createUserTextMessage', () => {
        it('creates user message with text content', () => {
            const result = createUserTextMessage('Hello')
            expect(result).toEqual({
                role: 'user',
                content: [{ type: 'text', text: 'Hello' }],
            })
        })
    })

    describe('fromApiMessage', () => {
        it('converts API message to thread message', () => {
            const apiMessage: Message = {
                role: 'assistant',
                content: 'Response text',
            }
            const result = fromApiMessage(apiMessage)
            expect(result).toEqual({
                role: 'assistant',
                content: [{ type: 'text', text: 'Response text' }],
            })
        })
    })

    describe('toChatRequestMessages', () => {
        it('converts thread messages to request format', () => {
            const messages = [
                { role: 'user' as const, content: [{ type: 'text' as const, text: 'Question' }] },
                { role: 'assistant' as const, content: [{ type: 'text' as const, text: 'Answer' }] },
            ]
            const result = toChatRequestMessages(messages)
            expect(result).toEqual([
                { role: 'user', content: 'Question' },
                { role: 'assistant', content: 'Answer' },
            ])
        })

        it('handles string content', () => {
            const messages = [{ role: 'user' as const, content: 'Direct string' }]
            const result = toChatRequestMessages(messages)
            expect(result).toEqual([{ role: 'user', content: 'Direct string' }])
        })
    })

    describe('upsertAssistantText', () => {
        it('adds assistant message when none exists', () => {
            const messages = [{ role: 'user' as const, content: [{ type: 'text' as const, text: 'Hi' }] }]
            const result = upsertAssistantText(messages, 'Hello')
            expect(result).toHaveLength(2)
            expect(result[1]).toEqual({
                role: 'assistant',
                content: [{ type: 'text', text: 'Hello' }],
            })
        })

        it('updates existing assistant message', () => {
            const messages = [
                { role: 'user' as const, content: [{ type: 'text' as const, text: 'Hi' }] },
                { role: 'assistant' as const, content: [{ type: 'text' as const, text: 'Old' }] },
            ]
            const result = upsertAssistantText(messages, 'New')
            expect(result).toHaveLength(2)
            expect(result[1].content).toEqual([{ type: 'text', text: 'New' }])
        })
    })

    describe('appendAssistantError', () => {
        it('appends error message to assistant', () => {
            const messages = [{ role: 'user' as const, content: [{ type: 'text' as const, text: 'Hi' }] }]
            const result = appendAssistantError(messages)
            expect(result[1].content).toEqual([
                { type: 'text', text: 'Sorry, an error occurred. Please try again.' },
            ])
        })
    })
})
