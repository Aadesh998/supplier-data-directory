import type { ApiErrorBody } from "./types";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;
  readonly details?: ApiErrorBody["error"]["details"];

  constructor(status: number, code: string, message: string, requestId?: string, details?: ApiErrorBody["error"]["details"]) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
    this.details = details;
  }

  get fieldErrors(): Record<string, string> {
    const fields = this.details?.fields;
    return fields && typeof fields === "object" ? fields : {};
  }

  get reasons(): string[] {
    const reasons = this.details?.reasons;
    return Array.isArray(reasons) ? reasons.map(String) : [];
  }
}

function isErrorBody(value: unknown): value is ApiErrorBody {
  if (!value || typeof value !== "object") return false;
  const err = (value as { error?: unknown }).error;
  return !!err && typeof err === "object" && typeof (err as { code?: unknown }).code === "string";
}

/** Build an ApiError from a non-2xx Response. Never throws while parsing. */
export async function toApiError(res: Response): Promise<ApiError> {
  let body: unknown = null;
  try {
    const text = await res.text();
    body = text ? JSON.parse(text) : null;
  } catch {
    body = null;
  }
  if (isErrorBody(body)) {
    const e = body.error;
    return new ApiError(res.status, e.code, e.message || defaultMessage(res.status), e.request_id, e.details);
  }
  return new ApiError(res.status, statusCode(res.status), defaultMessage(res.status));
}

function statusCode(status: number): string {
  switch (status) {
    case 401:
      return "unauthorized";
    case 403:
      return "forbidden";
    case 404:
      return "not_found";
    case 409:
      return "conflict";
    case 410:
      return "gone";
    case 429:
      return "rate_limited";
    default:
      return status >= 500 ? "server_error" : "request_failed";
  }
}

function defaultMessage(status: number): string {
  switch (status) {
    case 401:
      return "Your session has expired. Please sign in again.";
    case 403:
      return "You do not have permission to do this.";
    case 404:
      return "Not found.";
    case 429:
      return "Too many requests. Please wait a moment and try again.";
    default:
      return status >= 500 ? "The service is temporarily unavailable. Please try again." : "The request could not be completed.";
  }
}

/** Human-friendly message for any thrown value. */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 403) return `Not permitted: ${err.message}`;
    return err.message;
  }
  if (err instanceof Error) return err.message;
  return "Something went wrong.";
}
