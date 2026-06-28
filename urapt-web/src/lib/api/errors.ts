import type { ApiErrorBody, ErrorCode } from "./types";

/**
 * ApiError wraps a uniform urapt error response:
 *   {"error":{"code":"...","message":"...","details":...}}
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: ErrorCode | string;
  readonly details?: unknown;

  constructor(status: number, code: string, message: string, details?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }

  static isApiError(e: unknown): e is ApiError {
    return e instanceof ApiError;
  }

  /** A human-friendly message suitable for toast/UI display. */
  display(): string {
    return this.message || this.code || "request failed";
  }
}

/** Parse a Response into an ApiError, falling back to a network error. */
export async function toApiError(res: Response): Promise<ApiError> {
  let body: ApiErrorBody | undefined;
  try {
    body = (await res.json()) as ApiErrorBody;
  } catch {
    // non-JSON response
  }
  if (body?.error) {
    return new ApiError(
      res.status,
      body.error.code,
      body.error.message,
      body.error.details,
    );
  }
  return new ApiError(res.status, "internal", res.statusText || "request failed");
}

export const NETWORK_ERROR = new ApiError(0, "internal", "network error");
