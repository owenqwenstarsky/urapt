import { describe, expect, it } from "vitest";
import { toApiError, ApiError } from "./errors";

function mockRes(body: unknown, status: number, statusText = ""): Response {
  return {
    status,
    statusText,
    ok: status >= 200 && status < 300,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(JSON.stringify(body)),
  } as Response;
}

describe("toApiError", () => {
  it("parses a uniform error envelope", async () => {
    const err = await toApiError(
      mockRes({ error: { code: "not_found", message: "no such repo" } }, 404),
    );
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(404);
    expect(err.code).toBe("not_found");
    expect(err.message).toBe("no such repo");
  });

  it("falls back to statusText for non-JSON bodies", async () => {
    const res = {
      ...mockRes({}, 500, "Internal Server Error"),
      json: () => Promise.reject(new Error("bad")),
    };
    const err = await toApiError(res as Response);
    expect(err.status).toBe(500);
    expect(err.code).toBe("internal");
  });
});
