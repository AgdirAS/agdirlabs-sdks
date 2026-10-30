<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Mailer;

use AgdirLabs\Email\AgdirEmail;
use AgdirLabs\Email\Exception\AgdirEmailException;
use Psr\EventDispatcher\EventDispatcherInterface;
use Psr\Log\LoggerInterface;
use Symfony\Component\Mailer\Envelope;
use Symfony\Component\Mailer\Exception\InvalidArgumentException;
use Symfony\Component\Mailer\Exception\TransportException;
use Symfony\Component\Mailer\Header\MetadataHeader;
use Symfony\Component\Mailer\Header\TagHeader;
use Symfony\Component\Mailer\SentMessage;
use Symfony\Component\Mailer\Transport\AbstractTransport;
use Symfony\Component\Mime\Address;
use Symfony\Component\Mime\Email;
use Symfony\Component\Mime\Header\HeaderInterface;
use Symfony\Component\Mime\Header\UnstructuredHeader;
use Symfony\Component\Mime\Message;
use Symfony\Component\Mime\MessageConverter;

/**
 * Symfony Mailer transport that sends through the Agdir Email API (used by Laravel's `agdir` mailer too).
 *
 * Mapping from a Symfony `Email`:
 * - From, To, Cc, Bcc, Reply-To, Subject, HTML and text bodies → the matching API fields.
 *   Envelope recipients that are not Cc/Bcc are sent as `to`.
 * - `X-Agdir-Tag-<name>: <value>` headers, Symfony/Laravel `MetadataHeader` (`->metadata(k, v)`) and
 *   `TagHeader` (`->tag(t)`, sent as tag `t` = `true`) → API `tags`.
 * - Any other non-standard header (e.g. `X-Entity-Ref-ID`, `List-Unsubscribe`) → API `headers`.
 * - Attachments (including inline images) are not supported by the API yet and throw `UnsupportedAttachmentException`.
 *
 * The API email id is returned as `SentMessage::getMessageId()` (in Laravel too).
 */
final class AgdirTransport extends AbstractTransport
{
    public const TAG_HEADER_PREFIX = 'X-Agdir-Tag-';

    /** Headers the API sets itself, or that map to dedicated fields. */
    private const RESERVED_HEADERS = [
        'from', 'to', 'cc', 'bcc', 'reply-to', 'sender', 'subject', 'date', 'message-id',
        'content-type', 'content-transfer-encoding', 'mime-version', 'return-path',
    ];

    public function __construct(
        private readonly AgdirEmail $client,
        ?EventDispatcherInterface $dispatcher = null,
        ?LoggerInterface $logger = null,
    ) {
        parent::__construct($dispatcher, $logger);
    }

    public function __toString(): string
    {
        $host = parse_url($this->client->baseUrl, PHP_URL_HOST);

        return 'agdir+api://' . (is_string($host) ? $host : 'api.agdir.email');
    }

    protected function doSend(SentMessage $message): void
    {
        $original = $message->getOriginalMessage();
        if (!$original instanceof Message) {
            throw new InvalidArgumentException('The Agdir Email transport only sends Symfony\Component\Mime\Message instances.');
        }
        try {
            $email = MessageConverter::toEmail($original);
        } catch (\Exception $e) {
            throw new InvalidArgumentException(sprintf('Unable to send message with the "%s" transport: %s', self::class, $e->getMessage()), 0, $e);
        }

        try {
            $result = $this->client->emails->send(self::payload($email, $message->getEnvelope()));
        } catch (AgdirEmailException $e) {
            throw new TransportException(
                sprintf('Agdir Email API refused the message (%s): %s', $e->errorCode, $e->getMessage()),
                $e->status,
                $e,
            );
        }

        $message->setMessageId($result->id);
    }

    /**
     * The API send array for a Symfony email.
     *
     * @return array<string, mixed>
     */
    public static function payload(Email $email, ?Envelope $envelope = null): array
    {
        if ($email->getAttachments() !== []) {
            throw new UnsupportedAttachmentException(
                'Agdir Email does not support attachments yet: remove them (including inline/embedded images) or link to the file instead.',
            );
        }

        $from = $email->getFrom()[0] ?? $envelope?->getSender();
        if ($from === null) {
            throw new InvalidArgumentException('The email needs a From address.');
        }

        $cc = self::addresses($email->getCc());
        $bcc = self::addresses($email->getBcc());
        if ($envelope !== null) {
            // Envelope recipients can differ from the To header (e.g. a dev "send everything to me" listener).
            $copied = array_map(static fn (Address $a) => strtolower($a->getAddress()), [...$email->getCc(), ...$email->getBcc()]);
            $to = self::addresses(array_values(array_filter(
                $envelope->getRecipients(),
                static fn (Address $a) => !in_array(strtolower($a->getAddress()), $copied, true),
            )));
        } else {
            $to = self::addresses($email->getTo());
        }

        $payload = [
            'from' => self::address($from),
            'to' => $to,
            'subject' => (string) $email->getSubject(),
        ];
        if ($cc !== []) {
            $payload['cc'] = $cc;
        }
        if ($bcc !== []) {
            $payload['bcc'] = $bcc;
        }
        $replyTo = self::addresses($email->getReplyTo());
        if ($replyTo !== []) {
            $payload['replyTo'] = $replyTo;
        }
        $html = self::body($email->getHtmlBody());
        if ($html !== null) {
            $payload['html'] = $html;
        }
        $text = self::body($email->getTextBody());
        if ($text !== null) {
            $payload['text'] = $text;
        }

        $headers = [];
        $tags = [];
        foreach ($email->getHeaders()->all() as $header) {
            $name = $header->getName();
            $lower = strtolower($name);
            if ($header instanceof TagHeader) {
                $tags[$header->getValue()] = 'true';
            } elseif ($header instanceof MetadataHeader) {
                $tags[$header->getKey()] = $header->getValue();
            } elseif (str_starts_with($lower, strtolower(self::TAG_HEADER_PREFIX))) {
                $tags[substr($name, strlen(self::TAG_HEADER_PREFIX))] = self::headerValue($header);
            } elseif (!in_array($lower, self::RESERVED_HEADERS, true)) {
                $headers[$name] = self::headerValue($header);
            }
        }
        if ($headers !== []) {
            $payload['headers'] = $headers;
        }
        if ($tags !== []) {
            $payload['tags'] = $tags;
        }

        return $payload;
    }

    /**
     * @param list<Address>|array<Address> $addresses
     *
     * @return list<string>
     */
    private static function addresses(array $addresses): array
    {
        return array_values(array_map(self::address(...), $addresses));
    }

    private static function address(Address $address): string
    {
        $name = trim(str_replace(['"', "\r", "\n"], '', $address->getName()));
        if ($name === '') {
            return $address->getAddress();
        }
        if (preg_match('/[()<>\[\]:;@\\\\,.]/', $name) === 1) {
            $name = '"' . $name . '"';
        }

        return sprintf('%s <%s>', $name, $address->getAddress());
    }

    private static function headerValue(HeaderInterface $header): string
    {
        // The API encodes headers itself: pass unstructured values through unencoded.
        return $header instanceof UnstructuredHeader ? $header->getValue() : $header->getBodyAsString();
    }

    /** @param resource|string|null $body */
    private static function body(mixed $body): ?string
    {
        if (is_resource($body)) {
            if (stream_get_meta_data($body)['seekable']) {
                rewind($body);
            }
            $body = stream_get_contents($body);
        }

        return is_string($body) ? $body : null;
    }
}
