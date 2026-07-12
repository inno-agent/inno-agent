import { serve } from "@hono/node-server"
import { Hono } from "hono"
import { MastraServer } from "@mastra/hono"
import { mastra } from "./mastra/index"
import { z } from "zod"
import { randomUUID } from "crypto"

const REVIEW_AGENT_AUTH_TOKEN = process.env.REVIEW_AGENT_AUTH_TOKEN || ""
const REVIEW_TIMEOUT_MS = parseInt(process.env.REVIEW_TIMEOUT_MS || "300000") // 5 minutes default

const ReviewRequestSchema = z.object({
  owner: z.string(),
  repo: z.string(),
  pullNumber: z.number(),
  headSha: z.string(),
})

const app = new Hono()

// CORS middleware
app.use("*", async (c, next) => {
  c.header("Access-Control-Allow-Origin", "*")
  c.header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
  c.header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

  if (c.req.method === "OPTIONS") {
    return c.text("", 204)
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

// Auth middleware for /review endpoint
app.use("/review", async (c, next) => {
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
})

const server = new MastraServer({ app, mastra })
await server.init()

app.post("/review", async (c) => {
  const requestId = c.get("requestId") as string
  const body = await c.req.json()
  const parsed = ReviewRequestSchema.safeParse(body)

  if (!parsed.success) {
    return c.json({ error: "Invalid request", details: parsed.error }, 400)
  }

  const { owner, repo, pullNumber, headSha } = parsed.data

  console.log(`[${requestId}] Starting review for ${owner}/${repo}#${pullNumber}`)

  try {
    const workflow = mastra.getWorkflow("reviewPipeline")
    const run = await workflow.createRun()
    
    // Add timeout
    const timeoutPromise = new Promise((_, reject) => {
      setTimeout(() => reject(new Error("Review timeout")), REVIEW_TIMEOUT_MS)
    })

    const resultPromise = run.start({
      inputData: { owner, repo, pullNumber, headSha },
    })

    const result = await Promise.race([resultPromise, timeoutPromise]) as any

    if (result.status === "success") {
      console.log(`[${requestId}] Review completed successfully`)
      return c.json({ review_markdown: result.result.reviewMarkdown })
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
