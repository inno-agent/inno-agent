import { serve } from "@hono/node-server"
import { Hono } from "hono"
import { MastraServer } from "@mastra/hono"
import { mastra } from "./mastra/index"
import { z } from "zod"

const ReviewRequestSchema = z.object({
  owner: z.string(),
  repo: z.string(),
  pullNumber: z.number(),
  headSha: z.string(),
})

const app = new Hono()
const server = new MastraServer({ app, mastra })
await server.init()

app.post("/review", async (c) => {
  const body = await c.req.json()
  const parsed = ReviewRequestSchema.safeParse(body)

  if (!parsed.success) {
    return c.json({ error: "Invalid request", details: parsed.error }, 400)
  }

  const { owner, repo, pullNumber, headSha } = parsed.data

  try {
    const workflow = mastra.getWorkflow("reviewPipeline")
    const run = await workflow.createRun()
    const result = await run.start({
      inputData: { owner, repo, pullNumber, headSha },
    })

    if (result.status === "success") {
      return c.json({ review_markdown: result.result.reviewMarkdown })
    }

    return c.json({ error: "Review failed", status: result.status }, 500)
  } catch (error) {
    console.error("Review error:", error)
    return c.json({ error: "Internal server error" }, 500)
  }
})

const port = parseInt(process.env.SERVER_PORT || "4100")
serve({ fetch: app.fetch, port }, (info) => {
  console.log(`Review agent server running on http://localhost:${info.port}`)
})
