import { readFileSync } from "fs"
import { join, dirname } from "path"
import { fileURLToPath } from "url"

const __dirname = dirname(fileURLToPath(import.meta.url))

export function buildReviewPrompt(): string {
  return readFileSync(join(__dirname, "review.md"), "utf-8")
}
