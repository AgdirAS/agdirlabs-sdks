<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Resource;

use AgdirLabs\Email\Exception\AgdirEmailException;
use AgdirLabs\Email\Internal\Paginator;
use AgdirLabs\Email\Internal\Transport;
use AgdirLabs\Email\Internal\Wire;
use AgdirLabs\Email\Model\Deleted;
use AgdirLabs\Email\Model\Domain;
use AgdirLabs\Email\Model\Page;
use AgdirLabs\Email\Model\Reference;

final class Domains
{
    /** @internal */
    public function __construct(private readonly Transport $transport)
    {
    }

    /**
     * Adds a sending domain. Publish the returned `records` in DNS, then call `verify()`.
     *
     * @throws AgdirEmailException
     */
    public function create(string $name, ?string $region = null): Domain
    {
        $body = ['name' => $name];
        if ($region !== null) {
            $body['region'] = $region;
        }

        return Domain::fromWire($this->transport->request('POST', '/domains', retryable: false, body: $body));
    }

    /** @throws AgdirEmailException */
    public function get(string $id): Domain
    {
        return Domain::fromWire($this->transport->request('GET', '/domains/' . rawurlencode($id), retryable: true));
    }

    /**
     * One page of domains.
     *
     * @param int|null    $limit 1–100
     * @param string|null $after cursor: the id of the last item of the previous page
     *
     * @return Page<Domain>
     *
     * @throws AgdirEmailException
     */
    public function list(?int $limit = null, ?string $after = null): Page
    {
        $res = $this->transport->request('GET', '/domains', retryable: true, query: ['limit' => $limit, 'after' => $after]);

        return Page::fromWire($res, Domain::fromWire(...));
    }

    /**
     * Every domain, fetching further pages as you iterate.
     *
     * @param int|null $limit page size
     *
     * @return \Generator<int, Domain, mixed, void>
     *
     * @throws AgdirEmailException
     */
    public function listAll(?int $limit = null, ?string $after = null): \Generator
    {
        return Paginator::walk(fn (?string $cursor) => $this->list($limit, $cursor), $after);
    }

    /**
     * Starts a DNS check now instead of waiting for the next automatic one.
     *
     * @throws AgdirEmailException
     */
    public function verify(string $id): Reference
    {
        $res = $this->transport->request('POST', '/domains/' . rawurlencode($id) . '/verify', retryable: true);

        return new Reference(Wire::string($res['id'] ?? null));
    }

    /** @throws AgdirEmailException */
    public function delete(string $id): Deleted
    {
        $res = $this->transport->request('DELETE', '/domains/' . rawurlencode($id), retryable: false);

        return new Deleted(Wire::string($res['id'] ?? null));
    }
}
