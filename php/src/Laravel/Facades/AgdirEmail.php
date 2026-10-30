<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Laravel\Facades;

use Illuminate\Support\Facades\Facade;

/**
 * @property-read \AgdirLabs\Email\Resource\Emails   $emails
 * @property-read \AgdirLabs\Email\Resource\Domains  $domains
 * @property-read \AgdirLabs\Email\Resource\Webhooks $webhooks
 *
 * Use `AgdirEmail::emails()->send([...])`, `AgdirEmail::domains()`, `AgdirEmail::webhooks()`,
 * or resolve the client: `app(\AgdirLabs\Email\AgdirEmail::class)->emails->send([...])`.
 *
 * @method static \AgdirLabs\Email\Resource\Emails   emails()
 * @method static \AgdirLabs\Email\Resource\Domains  domains()
 * @method static \AgdirLabs\Email\Resource\Webhooks webhooks()
 *
 * @see \AgdirLabs\Email\AgdirEmail
 */
final class AgdirEmail extends Facade
{
    /**
     * Facades proxy method calls, not properties: `AgdirEmail::emails()` returns `$client->emails`.
     *
     * @param array<mixed> $args
     */
    public static function __callStatic($method, $args): mixed
    {
        $client = static::getFacadeRoot();
        if ($client instanceof \AgdirLabs\Email\AgdirEmail && $args === []) {
            switch ($method) {
                case 'emails':
                    return $client->emails;
                case 'domains':
                    return $client->domains;
                case 'webhooks':
                    return $client->webhooks;
            }
        }

        return parent::__callStatic($method, $args);
    }

    protected static function getFacadeAccessor(): string
    {
        return \AgdirLabs\Email\AgdirEmail::class;
    }
}
