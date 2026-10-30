/** Error codes returned by the Agdir Email API, plus client-side `network_error` / `timeout`. */
export type AgdirEmailErrorCode =
  | 'missing_api_key'
  | 'invalid_api_key'
  | 'restricted_api_key'
  | 'validation_error'
  | 'not_found'
  | 'invalid_idempotent_request'
  | 'concurrent_idempotent_requests'
  | 'daily_quota_exceeded'
  | 'monthly_quota_exceeded'
  | 'rate_limit_exceeded'
  | 'internal_server_error'
  | 'network_error'
  | 'timeout'
  | (string & {});

/** Thrown by every client method when a request fails. */
export class AgdirEmailError extends Error {
	override readonly name = 'AgdirEmailError';

	constructor(
		message: string,
		/** Machine-readable code, e.g. `validation_error` or `rate_limit_exceeded`. */
		readonly code: AgdirEmailErrorCode,
		/** HTTP status, or 0 when no response was received. */
		readonly status: number,
		/** Seconds the API asked us to wait (from `retry-after`), if any. */
		readonly retryAfter?: number,
		options?: { cause?: unknown }
	) {
		super(message, options);
	}
}

/** Thrown by `verifyWebhook` when a payload's signature or timestamp is invalid. */
export class WebhookVerificationError extends Error {
	override readonly name = 'WebhookVerificationError';
}
