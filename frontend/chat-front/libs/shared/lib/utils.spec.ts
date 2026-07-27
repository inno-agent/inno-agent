import { describe, it, expect } from 'vitest'
import { cn } from './utils'

describe('cn', () => {
    it('returns empty string for no inputs', () => {
        expect(cn()).toBe('')
    })

    it('merges single class string', () => {
        expect(cn('text-red-500')).toBe('text-red-500')
    })

    it('merges multiple class strings', () => {
        expect(cn('text-red-500', 'bg-blue-500')).toBe('text-red-500 bg-blue-500')
    })

    it('handles tailwind conflicts correctly', () => {
        expect(cn('px-2', 'px-4')).toBe('px-4')
    })

    it('handles conditional classes', () => {
        const isActive = true
        const isDisabled = false
        expect(cn('base-class', isActive && 'conditional-class')).toBe('base-class conditional-class')
        expect(cn('base-class', isDisabled && 'conditional-class')).toBe('base-class')
    })

    it('caches static strings', () => {
        const result1 = cn('text-red-500')
        const result2 = cn('text-red-500')
        expect(result1).toBe(result2)
    })
})
