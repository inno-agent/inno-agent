import { describe, it, expect } from "vitest"
import { GitFlameClient } from "../gitflame-client"

describe("GitFlameClient.getAuthenticatedCloneUrl", () => {
  it("embeds the token as basic auth and builds the .git path", () => {
    const client = new GitFlameClient({ baseUrl: "https://api.gitflame.ru", token: "sekret" })
    const url = client.getAuthenticatedCloneUrl("askarr", "pyfile")
    expect(url).toBe("https://x-access-token:sekret@api.gitflame.ru/askarr/pyfile.git")
  })

  it("URL-encodes owner/repo segments", () => {
    const client = new GitFlameClient({ baseUrl: "https://api.gitflame.ru", token: "sekret" })
    const url = client.getAuthenticatedCloneUrl("weird owner", "repo")
    expect(url).toBe("https://x-access-token:sekret@api.gitflame.ru/weird%20owner/repo.git")
  })
})

describe("GitFlameClient.redactToken", () => {
  it("replaces every occurrence of the token with a placeholder", () => {
    const client = new GitFlameClient({ baseUrl: "https://api.gitflame.ru", token: "sekret" })
    const msg = "fatal: repository 'https://x-access-token:sekret@api.gitflame.ru/o/r.git/' not found (token: sekret)"
    const redacted = client.redactToken(msg)
    expect(redacted).not.toContain("sekret")
    expect(redacted).toContain("***")
  })

  it("is a no-op when the token is empty", () => {
    const client = new GitFlameClient({ baseUrl: "https://api.gitflame.ru", token: "" })
    expect(client.redactToken("some text")).toBe("some text")
  })
})
