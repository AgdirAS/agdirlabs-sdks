<?php

declare(strict_types=1);

return [
    /*
    | Your Agdir Email API key (re_…). Create one at https://app.agdir.email.
    */
    'api_key' => env('AGDIR_EMAIL_API_KEY'),

    /*
    | API base URL. Leave unset for https://api.agdir.email.
    */
    'base_url' => env('AGDIR_EMAIL_BASE_URL'),

    /*
    | Retries for rate limits, server errors and network failures (0 disables).
    | Sends are only retried with an idempotency key, so they are never delivered twice.
    */
    'max_retries' => (int) env('AGDIR_EMAIL_MAX_RETRIES', 2),

    /*
    | Timeout in seconds for each attempt.
    */
    'timeout' => (float) env('AGDIR_EMAIL_TIMEOUT', 30),
];
