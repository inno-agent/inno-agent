import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

const staticClassCache = new Map<string, string>()
const MAX_CACHE_SIZE = 500

export function cn(...inputs: ClassValue[]) {
  if (inputs.length === 0) return ""

  if (inputs.length === 1 && typeof inputs[0] === "string") {
    const singleClass = inputs[0]
    const cached = staticClassCache.get(singleClass)
    if (cached !== undefined) return cached

    const result = twMerge(singleClass)
    if (staticClassCache.size < MAX_CACHE_SIZE) {
      staticClassCache.set(singleClass, result)
    }
    return result
  }

  const allStatic = inputs.every(input => typeof input === "string")

  if (allStatic) {
    const cacheKey = inputs.join("|")
    const cached = staticClassCache.get(cacheKey)
    if (cached !== undefined) return cached

    const result = twMerge(clsx(inputs))
    if (staticClassCache.size < MAX_CACHE_SIZE) {
      staticClassCache.set(cacheKey, result)
    }
    return result
  }

  return twMerge(clsx(inputs))
}
