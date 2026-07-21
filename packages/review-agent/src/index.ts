import { serve } from "@hono/node-server"
import { Hono } from "hono"
import { mastra } from "./mastra/index"
import { z } from "zod"
import { randomUUID } from "crypto"

// ─── Hono typed context ─────────────────────────────────────────────────────

type AppEnv = {
  Variables: {
    requestId: string
  }
}

const REVIEW_AGENT_AUTH_TOKEN = process.env.REVIEW_AGENT_AUTH_TOKEN || ""
const REVIEW_TIMEOUT_MS = parseInt(process.env.REVIEW_TIMEOUT_MS || "300000") // 5 minutes default
const CODEGEN_TIMEOUT_MS = parseInt(process.env.CODEGEN_TIMEOUT_MS || "300000") // 5 minutes default
const CACHE_TTL_MS = parseInt(process.env.CACHE_TTL_MS || "3600000") // 1 hour default
const MAX_CONCURRENT_REVIEWS = parseInt(process.env.MAX_CONCURRENT_REVIEWS || "4")

// ─── Rate limiting ──────────────────────────────────────────────────────────

class Semaphore {
  private queue: (() => void)[] = []
  private running = 0
  constructor(private max: number) {}
  async acquire() {
    if (this.running < this.max) {
      this.running++
      return
    }
    return new Promise<void>((resolve) => this.queue.push(resolve))
  }
  release() {
    this.running--
    if (this.queue.length > 0) {
      this.running++
      this.queue.shift()!()
    }
  }
}

const reviewSemaphore = new Semaphore(MAX_CONCURRENT_REVIEWS)

// withTimeout races a workflow run against a deadline and always clears the
// timer, so a completed run doesn't pin a multi-minute timer per request.
//
// It deliberately does NOT cancel the underlying run — Mastra keeps executing
// it. Callers must therefore release their semaphore slot when the run promise
// settles, not when this function returns; see runSlot below.
async function withTimeout<T>(runPromise: Promise<T>, ms: number, label: string): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined
  try {
    return await Promise.race([
      runPromise,
      new Promise<never>((_, reject) => {
        timer = setTimeout(() => reject(new Error(`${label} timeout`)), ms)
      }),
    ])
  } finally {
    if (timer) clearTimeout(timer)
  }
}

// runSlot acquires a concurrency slot, invokes `start`, and releases the slot
// only when the run itself settles.
//
// Releasing in a `finally` around the timeout race would hand the slot back
// while the abandoned run is still burning an Ollama connection, letting real
// concurrency drift above MAX_CONCURRENT_REVIEWS. The returned promise tracks
// the run, so the caller's timeout covers slot-queue time too.
function runSlot<T>(start: () => Promise<T>): Promise<T> {
  return reviewSemaphore.acquire().then(() => {
    let runPromise: Promise<T>
    try {
      runPromise = start()
    } catch (err) {
      reviewSemaphore.release()
      throw err
    }
    runPromise.then(
      () => reviewSemaphore.release(),
      () => reviewSemaphore.release(),
    )
    return runPromise
  })
}

// Simple in-memory cache for PR reviews
interface CacheEntry {
  markdown: string
  createdAt: number
}

const reviewCache = new Map<string, CacheEntry>()

// Fix 11: Cache metrics
let cacheHits = 0
let cacheMisses = 0

function getCachedReview(key: string): string | null {
  const entry = reviewCache.get(key)
  if (!entry) return null
  if (Date.now() - entry.createdAt > CACHE_TTL_MS) {
    reviewCache.delete(key)
    return null
  }
  return entry.markdown
}

function setCachedReview(key: string, markdown: string): void {
  // Evict oldest entries if cache is too large (max 100 entries)
  if (reviewCache.size > 100) {
    const oldestKey = reviewCache.keys().next().value
    if (oldestKey) reviewCache.delete(oldestKey)
  }
  reviewCache.set(key, { markdown, createdAt: Date.now() })
}

const ReviewRequestSchema = z.object({
  owner: z.string(),
  repo: z.string(),
  pullNumber: z.number(),
  headSha: z.string(),
})

const CodegenRequestSchema = z.object({
  owner: z.string(),
  repo: z.string(),
  issueNumber: z.number(),
  defaultBranch: z.string().optional(),
  issueType: z.string().optional(),
  title: z.string().optional(),
  body: z.string().optional(),
})

const app = new Hono<AppEnv>()

// CORS middleware
app.use("*", async (c, next) => {
  c.header("Access-Control-Allow-Origin", "*")
  c.header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
  c.header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

  if (c.req.method === "OPTIONS") {
    return new Response(null, { status: 204 })
  }

  await next()
})

// Request ID middleware
app.use("*", async (c, next) => {
  const requestId = c.req.header("X-Request-ID") || randomUUID()
  c.header("X-Request-ID", requestId)
  c.set("requestId", requestId)
  await next()
})

// Bearer-token auth for the agent endpoints. REVIEW_AGENT_AUTH_TOKEN is the
// shared secret for this service; both /review and /codegen are guarded by it.
const authMiddleware = async (c: any, next: any) => {
  if (!REVIEW_AGENT_AUTH_TOKEN) {
    return c.json({ error: "Auth not configured" }, 503)
  }

  const auth = c.req.header("Authorization")
  if (!auth || !auth.startsWith("Bearer ")) {
    return c.json({ error: "Unauthorized" }, 401)
  }

  const token = auth.slice(7)
  if (token !== REVIEW_AGENT_AUTH_TOKEN) {
    return c.json({ error: "Unauthorized" }, 401)
  }

  await next()
}

app.use("/review", authMiddleware)
app.use("/codegen", authMiddleware)

app.post("/review", async (c) => {
  const requestId = c.get("requestId")
  const body = await c.req.json()
  const parsed = ReviewRequestSchema.safeParse(body)

  if (!parsed.success) {
    return c.json({ error: "Invalid request", details: parsed.error }, 400)
  }

  const { owner, repo, pullNumber, headSha } = parsed.data

  // Check cache first
  const cacheKey = `${owner}/${repo}#${pullNumber}@${headSha}`
  const cached = getCachedReview(cacheKey)
  if (cached) {
    cacheHits++
    console.log(`[${requestId}] Cache hit for ${owner}/${repo}#${pullNumber} (hits: ${cacheHits}/${cacheHits + cacheMisses})`)
    return c.json({ review_markdown: cached })
  }
  cacheMisses++

  console.log(`[${requestId}] Starting review for ${owner}/${repo}#${pullNumber}`)

  try {
    const workflow = mastra.getWorkflow("reviewPipeline")
    const run = await workflow.createRun()

    // Fix 7: Rate limiting
    const resultPromise = runSlot(() => run.start({
      inputData: { owner, repo, pullNumber, headSha },
    }))

    const result = await withTimeout(resultPromise, REVIEW_TIMEOUT_MS, "Review") as any

    if (result.status === "success") {
      const reviewMarkdown = result.result.reviewMarkdown
      console.log(`[${requestId}] Review completed successfully`)
      // Only cache non-empty reviews, so a blank/degraded result isn't pinned
      // for the full TTL and served on every subsequent request for this commit.
      if (typeof reviewMarkdown === "string" && reviewMarkdown.trim()) {
        setCachedReview(cacheKey, reviewMarkdown)
      }
      return c.json({ review_markdown: reviewMarkdown })
    }

    console.error(`[${requestId}] Review failed with status: ${result.status}`)
    return c.json({ error: "Review failed", status: result.status }, 500)
  } catch (error: any) {
    console.error(`[${requestId}] Review error:`, error)
    if (error.message === "Review timeout") {
      return c.json({ error: "Review timeout" }, 504)
    }
    return c.json({ error: "Internal server error" }, 500)
  }
})

// Health check endpoint
app.get("/health", (c) => {
  return c.json({ status: "ok" })
})

// ─── /codegen: issue → generated files via the codegen pipeline ────────────
//
// Mirrors /review: the consumer sends the issue ref (+ optional context from
// the webhook), the Mastra codegen workflow fetches repo context, runs the
// code-generator agent, parses the structured output, and returns the files.
app.post("/codegen", async (c) => {
  const requestId = c.get("requestId")
  const body = await c.req.json()
  const parsed = CodegenRequestSchema.safeParse(body)

  if (!parsed.success) {
    return c.json({ error: "Invalid request", details: parsed.error }, 400)
  }

  const { owner, repo, issueNumber } = parsed.data

  console.log(`[${requestId}] Starting codegen for ${owner}/${repo}#${issueNumber}`)

  try {
    const workflow = mastra.getWorkflow("codegenPipeline")
    const run = await workflow.createRun()

    const resultPromise = runSlot(() => run.start({ inputData: parsed.data }))

    const result = await withTimeout(resultPromise, CODEGEN_TIMEOUT_MS, "Codegen") as any

    if (result.status === "success") {
      const out = result.result
      console.log(`[${requestId}] Codegen completed: ${out.files?.length || 0} files`)
      return c.json({
        summary: out.summary || "",
        files: out.files || [],
      })
    }

    console.error(`[${requestId}] Codegen failed with status: ${result.status}`)
    return c.json({ error: "Codegen failed", status: result.status }, 500)
  } catch (error: any) {
    console.error(`[${requestId}] Codegen error:`, error)
    if (error.message === "Codegen timeout") {
      return c.json({ error: "Codegen timeout" }, 504)
    }
    return c.json({ error: "Internal server error" }, 500)
  }
})

const port = parseInt(process.env.SERVER_PORT || "4100")

// Graceful shutdown
const server_instance = serve({ fetch: app.fetch, port }, (info) => {
  console.log(`Review agent server running on http://localhost:${info.port}`)
})

process.on("SIGTERM", () => {
  console.log("SIGTERM received, shutting down...")
  server_instance.close(() => {
    process.exit(0)
  })
})

process.on("SIGINT", () => {
  console.log("SIGINT received, shutting down...")
  server_instance.close(() => {
    process.exit(0)
  })
})
