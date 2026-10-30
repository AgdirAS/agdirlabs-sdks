<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Mailer;

use AgdirLabs\Email\AgdirEmail;
use Symfony\Component\Mailer\Exception\UnsupportedSchemeException;
use Symfony\Component\Mailer\Transport\AbstractTransportFactory;
use Symfony\Component\Mailer\Transport\Dsn;
use Symfony\Component\Mailer\Transport\TransportInterface;

/**
 * Creates the Agdir Email transport from a DSN:
 *
 * - `agdir+api://KEY@default` — https://api.agdir.email
 * - `agdir+api://KEY@host[:port]` — another API host (https)
 * - options: `?max_retries=2&timeout=30`
 *
 * `agdir://` is accepted as an alias of `agdir+api://`.
 */
final class AgdirTransportFactory extends AbstractTransportFactory
{
    public function create(Dsn $dsn): TransportInterface
    {
        if (!in_array($dsn->getScheme(), $this->getSupportedSchemes(), true)) {
            throw new UnsupportedSchemeException($dsn, 'agdir', $this->getSupportedSchemes());
        }

        $host = $dsn->getHost();
        $baseUrl = $host === 'default'
            ? AgdirEmail::DEFAULT_BASE_URL
            : 'https://' . $host . ($dsn->getPort() !== null ? ':' . $dsn->getPort() : '');
        $maxRetries = $dsn->getOption('max_retries');
        $timeout = $dsn->getOption('timeout');

        $client = new AgdirEmail(
            apiKey: $this->getUser($dsn),
            baseUrl: $baseUrl,
            maxRetries: is_numeric($maxRetries) ? (int) $maxRetries : 2,
            timeout: is_numeric($timeout) ? (float) $timeout : 30.0,
        );

        return new AgdirTransport($client, $this->dispatcher, $this->logger);
    }

    /** @return list<string> */
    protected function getSupportedSchemes(): array
    {
        return ['agdir+api', 'agdir'];
    }
}
