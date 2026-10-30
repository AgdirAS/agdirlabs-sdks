<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Exception;

/**
 * Thrown by every client method when a request fails.
 *
 * `$e->errorCode` is the machine-readable API error name (e.g. `validation_error`,
 * `rate_limit_exceeded`), or `network_error` / `timeout` when no response was received.
 */
class AgdirEmailException extends \RuntimeException
{
    public const MISSING_API_KEY = 'missing_api_key';
    public const INVALID_API_KEY = 'invalid_api_key';
    public const RESTRICTED_API_KEY = 'restricted_api_key';
    public const VALIDATION_ERROR = 'validation_error';
    public const NOT_FOUND = 'not_found';
    public const INVALID_IDEMPOTENT_REQUEST = 'invalid_idempotent_request';
    public const CONCURRENT_IDEMPOTENT_REQUESTS = 'concurrent_idempotent_requests';
    public const DAILY_QUOTA_EXCEEDED = 'daily_quota_exceeded';
    public const MONTHLY_QUOTA_EXCEEDED = 'monthly_quota_exceeded';
    public const RATE_LIMIT_EXCEEDED = 'rate_limit_exceeded';
    public const INTERNAL_SERVER_ERROR = 'internal_server_error';
    public const NETWORK_ERROR = 'network_error';
    public const TIMEOUT = 'timeout';

    /**
     * @param string     $errorCode  API error name, or `network_error` / `timeout`
     * @param int        $status     HTTP status, or 0 when no response was received
     * @param float|null $retryAfter seconds the API asked us to wait (from `retry-after`), if any
     */
    public function __construct(
        string $message,
        public readonly string $errorCode,
        public readonly int $status,
        public readonly ?float $retryAfter = null,
        ?\Throwable $previous = null,
    ) {
        // \Exception::$code must be an int: expose the HTTP status there too.
        parent::__construct($message, $status, $previous);
    }

    /** Machine-readable error code, e.g. `validation_error`. Same as `$errorCode`. */
    public function getErrorCode(): string
    {
        return $this->errorCode;
    }

    /** HTTP status, or 0 when no response was received. */
    public function getStatus(): int
    {
        return $this->status;
    }

    /** Seconds the API asked us to wait before retrying, if it said. */
    public function getRetryAfter(): ?float
    {
        return $this->retryAfter;
    }
}
