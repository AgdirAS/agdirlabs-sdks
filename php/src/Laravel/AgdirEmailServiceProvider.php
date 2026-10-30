<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Laravel;

use AgdirLabs\Email\AgdirEmail;
use AgdirLabs\Email\Mailer\AgdirTransport;
use Illuminate\Contracts\Container\Container;
use Illuminate\Mail\MailManager;
use Illuminate\Support\ServiceProvider;

/**
 * Registers the `AgdirEmail` client (config `agdir-email`) and the `agdir` mail transport.
 *
 * Auto-discovered. To send Laravel mail through Agdir Email set `MAIL_MAILER=agdir`: an `agdir` mailer
 * (`['transport' => 'agdir']`) is added to `mail.mailers` unless `config/mail.php` defines one. A mailer may
 * set its own `key` (and `base_url`) to use another API key.
 */
final class AgdirEmailServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->mergeConfigFrom(__DIR__ . '/../../config/agdir-email.php', 'agdir-email');

        $this->app->singleton(AgdirEmail::class, static function (Container $app): AgdirEmail {
            /** @var array{api_key?: string|null, base_url?: string|null, max_retries?: int|string|null, timeout?: float|int|string|null} $config */
            $config = $app->make('config')->get('agdir-email', []);

            return self::client($config['api_key'] ?? null, $config['base_url'] ?? null, $config);
        });
        $this->app->alias(AgdirEmail::class, 'agdir-email');

        $config = $this->app->make('config');
        if ($config->get('mail.mailers.agdir') === null) {
            $config->set('mail.mailers.agdir', ['transport' => 'agdir']);
        }
    }

    public function boot(): void
    {
        if ($this->app->runningInConsole()) {
            $this->publishes([
                __DIR__ . '/../../config/agdir-email.php' => $this->app->configPath('agdir-email.php'),
            ], 'agdir-email-config');
        }

        $this->callAfterResolving('mail.manager', static function (MailManager $manager, Container $app): void {
            $manager->extend('agdir', static function (array $config = []) use ($app): AgdirTransport {
                $key = $config['key'] ?? $config['api_key'] ?? null;
                $baseUrl = $config['base_url'] ?? null;
                $client = is_string($key) && $key !== ''
                    ? self::client($key, is_string($baseUrl) ? $baseUrl : null, $app->make('config')->get('agdir-email', []))
                    : $app->make(AgdirEmail::class);

                return new AgdirTransport($client);
            });
        });
    }

    /**
     * @param array<mixed> $config
     */
    private static function client(?string $apiKey, ?string $baseUrl, array $config): AgdirEmail
    {
        $maxRetries = $config['max_retries'] ?? 2;
        $timeout = $config['timeout'] ?? 30;

        return new AgdirEmail(
            apiKey: $apiKey,
            baseUrl: $baseUrl,
            maxRetries: is_numeric($maxRetries) ? (int) $maxRetries : 2,
            timeout: is_numeric($timeout) ? (float) $timeout : 30.0,
        );
    }

    /** @return list<string> */
    public function provides(): array
    {
        return [AgdirEmail::class, 'agdir-email'];
    }
}
