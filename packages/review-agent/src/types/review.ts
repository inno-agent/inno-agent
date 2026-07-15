import { z } from "zod"

export const ReviewIssueSchema = z.object({
  file: z.string().describe("Path to the file"),
  line: z.number().optional().describe("Line number"),
  severity: z.enum(["critical", "warning", "info"]).describe("Issue severity"),
  message: z.string().describe("Description of the issue"),
})

export const ReviewResultSchema = z.object({
  summary: z.string().describe("Overall summary of the PR review"),
  potentialBugs: z.array(ReviewIssueSchema).describe("Potential bugs found"),
  securityIssues: z.array(ReviewIssueSchema).describe("Security issues found"),
  performanceIssues: z.array(ReviewIssueSchema).describe("Performance issues found"),
  suggestedImprovements: z
    .array(
      z.object({
        file: z.string(),
        line: z.number().optional(),
        message: z.string(),
      })
    )
    .describe("Suggested improvements"),
})

export type ReviewResult = z.infer<typeof ReviewResultSchema>
export type ReviewIssue = z.infer<typeof ReviewIssueSchema>
