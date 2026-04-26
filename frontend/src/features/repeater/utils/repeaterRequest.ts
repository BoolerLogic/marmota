import type {
    HistoryEntry,
    RequestView,
} from "@/features/http-history/state/httpHistoryStore";

export type RepeaterScheme = "http" | "https";
export type RepeaterHttpVersion = "HTTP/1.0" | "HTTP/1.1" | "HTTP/2";
export type RepeaterValidationLevel = "error" | "warning";
export type RepeaterValidationIssue = {
    id: string;
    level: RepeaterValidationLevel;
    line: number | null;
    message: string;
};
export type RepeaterHeader = {
    name: string;
    value: string;
    line: number;
};
export type RepeaterHeaderMap = Record<string, string[]>;
export type RepeaterHttp2PseudoHeaders = {
    method: string;
    scheme: RepeaterScheme;
    authority: string;
    path: string;
    protocol: string;
};
export type ParsedRepeaterRequest = {
    method: string;
    path: string;
    version: RepeaterHttpVersion;
    headers: RepeaterHeader[];
};
export type RepeaterRequestDraft = {
    scheme: RepeaterScheme;
    host: string;
    headBlockStr: string;
    bodyStr: string;
    skipServerCertVerify: boolean;
};
export type RepeaterValidationResult = {
    parsedRequest: ParsedRepeaterRequest | null;
    errors: RepeaterValidationIssue[];
    warnings: RepeaterValidationIssue[];
};
export type PreparedRepeaterRequest = {
    version: RepeaterHttpVersion;
    headBlockStr: string;
    method: string;
    path: string;
    host: string;
    port: string;
    headers: RepeaterHeaderMap;
    pseudoHeaders: RepeaterHttp2PseudoHeaders;
};

const requestLinePattern =
    /^([!#$%&'*+\-.^_`|~0-9A-Za-z]+) (\S+) (HTTP\/(?:1\.0|1\.1|2))$/;
const headerPattern = /^(:?[!#$%&'*+\-.^_`|~0-9A-Za-z]+):(.*)$/;
const authorityPattern =
    /^(?:\[[0-9A-Fa-f:.%]+\]|[A-Za-z0-9.-]+)(?::\d{1,5})?$/;
const connectionSpecificHeaders = new Set([
    "connection",
    "proxy-connection",
    "keep-alive",
    "upgrade",
]);

function createIssue(
    level: RepeaterValidationLevel,
    line: number | null,
    message: string,
): RepeaterValidationIssue {
    return {
        id: `${level}-${line ?? "global"}-${message}`,
        level,
        line,
        message,
    };
}

function normalizeLineEndings(value: string): string {
    return value.replace(/\r\n/g, "\n");
}

function splitHeadBlockLines(rawHeadBlock: string): string[] {
    if (!rawHeadBlock) return [""];
    return normalizeLineEndings(rawHeadBlock).split("\n");
}

function trimTrailingBlankLines(lines: string[]): string[] {
    const trimmedLines = [...lines];

    while (trimmedLines.length > 0 && trimmedLines.at(-1) === "") {
        trimmedLines.pop();
    }

    return trimmedLines;
}

function trimTrailingHeadBlock(rawHeadBlock: string): string {
    return trimTrailingBlankLines(splitHeadBlockLines(rawHeadBlock)).join("\n");
}

function finalizeHeadBlockForTransport(rawHeadBlock: string): string {
    const trimmedLines = trimTrailingBlankLines(splitHeadBlockLines(rawHeadBlock));
    return `${trimmedLines.join("\r\n")}\r\n\r\n`;
}

function getBodyByteLength(body: string): number {
    return new TextEncoder().encode(body).length;
}

function getDefaultPortForScheme(scheme: RepeaterScheme): string {
    return scheme === "https" ? "443" : "80";
}

function isValidPortValue(value: string): boolean {
    if (!/^\d{1,5}$/.test(value)) return false;
    const numericPort = Number.parseInt(value, 10);
    return numericPort >= 1 && numericPort <= 65535;
}

function getAuthorityFromRequest(request: RequestView): string {
    if (!request.host) return "";
    if (!request.port || request.port === getDefaultPortForScheme(request.scheme === "https" ? "https" : "http")) {
        return request.host;
    }

    return `${request.host}:${request.port}`;
}

function getStartLine(request: RequestView): string {
    const normalizedVersion =
        request.version === "HTTP/1.0" ||
        request.version === "HTTP/1.1" ||
        request.version === "HTTP/2"
            ? request.version
            : "HTTP/1.1";

    const path = request.path?.trim() || "/";
    const method = request.method?.trim() || "GET";

    return `${method} ${path} ${normalizedVersion}`;
}

function normalizeSeedHeadBlock(request: RequestView): string {
    const normalizedHeadBlock = request.headBlockStr ?? "";
    const lines = splitHeadBlockLines(normalizedHeadBlock);
    const trimmedLines = [...lines];

    while (trimmedLines.length > 1 && trimmedLines.at(-1) === "") {
        trimmedLines.pop();
    }

    if (trimmedLines.length === 0 || trimmedLines[0].trim().length === 0) {
        return getStartLine(request);
    }

    return trimmedLines.join("\n");
}

function removeHeaderFromHeadBlock(
    rawHeadBlock: string,
    headerName: string,
): string {
    const trimmedLines = trimTrailingBlankLines(splitHeadBlockLines(rawHeadBlock));
    if (trimmedLines.length <= 1) {
        return trimmedLines.join("\n");
    }

    const normalizedHeaderName = headerName.toLowerCase();
    const nextLines = [
        trimmedLines[0],
        ...trimmedLines.slice(1).filter((line) => {
            const headerMatch = line.match(headerPattern);
            if (!headerMatch) return true;

            return headerMatch[1].toLowerCase() !== normalizedHeaderName;
        }),
    ];

    return nextLines.join("\n");
}

function getPrimaryContentType(value: string): string {
    return value.split(";", 1)[0]?.trim().toLowerCase() ?? "";
}

function isJsonContentType(value: string): boolean {
    const primaryContentType = getPrimaryContentType(value);

    return (
        primaryContentType === "application/json" ||
        primaryContentType === "text/json" ||
        primaryContentType.endsWith("+json")
    );
}

function normalizeSeedRequest(
    request: RequestView,
): Pick<RepeaterRequestDraft, "headBlockStr" | "bodyStr"> {
    const headBlockStr = normalizeSeedHeadBlock(request);
    const bodyStr = request.bodyStr ?? "";
    if (!bodyStr) {
        return {
            headBlockStr,
            bodyStr: "",
        };
    }

    const contentType =
        extractHeaderValues(parseHeaderLinesFromHeadBlock(headBlockStr), "content-type")[0]
            ?.value ?? "";

    if (!isJsonContentType(contentType)) {
        return {
            headBlockStr,
            bodyStr,
        };
    }

    try {
        return {
            headBlockStr: removeHeaderFromHeadBlock(headBlockStr, "content-length"),
            bodyStr: JSON.stringify(JSON.parse(bodyStr), null, 2),
        };
    } catch {
        return {
            headBlockStr,
            bodyStr,
        };
    }
}

function extractHeaderValues(
    headers: RepeaterHeader[],
    name: string,
): RepeaterHeader[] {
    const normalizedName = name.toLowerCase();
    return headers.filter((header) => header.name.toLowerCase() === normalizedName);
}

function parseContentLength(
    headers: RepeaterHeader[],
    issues: RepeaterValidationIssue[],
): number | null {
    const values = extractHeaderValues(headers, "content-length");
    if (values.length === 0) return null;

    const parsedValues = values.map((header) => {
        if (!/^\d+$/.test(header.value.trim())) {
            issues.push(
                createIssue(
                    "error",
                    header.line,
                    "Content-Length must be a positive integer.",
                ),
            );
            return null;
        }

        return Number.parseInt(header.value.trim(), 10);
    });

    if (parsedValues.includes(null)) {
        return null;
    }

    const uniqueValues = new Set(parsedValues);
    if (uniqueValues.size > 1) {
        issues.push(
            createIssue(
                "error",
                values[0]?.line ?? null,
                "Multiple Content-Length headers have different values.",
            ),
        );
        return null;
    }

    return parsedValues[0] ?? null;
}

function parseTransferEncoding(headers: RepeaterHeader[]): string[] {
    const values = extractHeaderValues(headers, "transfer-encoding");
    return values.flatMap((header) =>
        header.value
            .split(",")
            .map((token) => token.trim().toLowerCase())
            .filter(Boolean),
    );
}

function splitAuthorityParts(
    authorityInput: string,
    scheme: RepeaterScheme,
): { host: string; port: string; authority: string } {
    const defaultPort = getDefaultPortForScheme(scheme);
    const sanitizedAuthority = sanitizeRepeaterAuthorityInput(authorityInput).trim();
    const ipv6Match = sanitizedAuthority.match(
        /^\[([0-9A-Fa-f:.%]+)\](?::(\d{1,5}))?$/,
    );

    if (ipv6Match) {
        const host = ipv6Match[1];
        const explicitPort = ipv6Match[2] ?? "";
        const port = explicitPort || defaultPort;
        return {
            host,
            port,
            authority: explicitPort ? `[${host}]:${explicitPort}` : `[${host}]`,
        };
    }

    const standardMatch = sanitizedAuthority.match(/^([A-Za-z0-9.-]+)(?::(\d{1,5}))?$/);
    if (standardMatch) {
        const host = standardMatch[1];
        const explicitPort = standardMatch[2] ?? "";
        const port = explicitPort || defaultPort;
        return {
            host,
            port,
            authority: explicitPort ? `${host}:${explicitPort}` : host,
        };
    }

    return {
        host: sanitizedAuthority,
        port: defaultPort,
        authority: sanitizedAuthority,
    };
}

function parseHeaderLinesFromHeadBlock(rawHeadBlock: string): RepeaterHeader[] {
    const lines = trimTrailingBlankLines(splitHeadBlockLines(rawHeadBlock));
    const headers: RepeaterHeader[] = [];

    for (let index = 1; index < lines.length; index += 1) {
        const line = lines[index];
        const headerMatch = line.match(headerPattern);
        if (!headerMatch) continue;

        headers.push({
            name: headerMatch[1],
            value: headerMatch[2].replace(/^\s*/, ""),
            line: index + 1,
        });
    }

    return headers;
}

function buildHeaderMap(headers: RepeaterHeader[]): RepeaterHeaderMap {
    return headers.reduce<RepeaterHeaderMap>((map, header) => {
        if (header.name.startsWith(":")) {
            return map;
        }

        const normalizedName = header.name.toLowerCase();
        const currentValues = map[normalizedName] ?? [];
        return {
            ...map,
            [normalizedName]: [...currentValues, header.value],
        };
    }, {});
}

function getPseudoHeaderValue(
    headers: RepeaterHeader[],
    name: string,
): string {
    return extractHeaderValues(headers, name)[0]?.value.trim() ?? "";
}

function buildPseudoHeaders(
    draft: RepeaterRequestDraft,
    parsedRequest: ParsedRepeaterRequest | null,
    headers: RepeaterHeader[],
): RepeaterHttp2PseudoHeaders {
    return {
        method:
            getPseudoHeaderValue(headers, ":method") ||
            parsedRequest?.method ||
            "",
        scheme:
            (getPseudoHeaderValue(headers, ":scheme") as RepeaterScheme) ||
            draft.scheme,
        authority:
            getPseudoHeaderValue(headers, ":authority") || draft.host.trim(),
        path:
            getPseudoHeaderValue(headers, ":path") ||
            parsedRequest?.path ||
            "/",
        protocol: getPseudoHeaderValue(headers, ":protocol"),
    };
}

export function sanitizeRepeaterAuthorityInput(value: string): string {
    const normalizedValue = value.trim();
    const withoutScheme = normalizedValue.replace(/^[A-Za-z][A-Za-z0-9+.-]*:\/\//, "");
    const authorityOnly = withoutScheme.split(/[/?#\\]/, 1)[0] ?? "";
    return authorityOnly.replace(/[^A-Za-z0-9.\-:[\]%]/g, "");
}

export function buildRepeaterUrlPreview(
    scheme: RepeaterScheme,
    host: string,
    path: string,
): string {
    if (!host.trim()) return "";
    const normalizedPath = path.trim().length > 0 ? path.trim() : "/";
    return `${scheme}://${host.trim()}${normalizedPath}`;
}

export function prepareRepeaterRequestForSend(
    draft: RepeaterRequestDraft,
    parsedRequest: ParsedRepeaterRequest | null,
): PreparedRepeaterRequest {
    const version = parsedRequest?.version ?? "HTTP/1.1";
    const authorityParts = splitAuthorityParts(draft.host, draft.scheme);
    const normalizedHeadBlock = trimTrailingHeadBlock(draft.headBlockStr);
    const hasBody = draft.bodyStr.length > 0;
    const contentLengthHeaders = parsedRequest
        ? extractHeaderValues(parsedRequest.headers, "content-length")
        : [];
    const normalizedHeadBlockForPayload =
        !hasBody || contentLengthHeaders.length > 0
            ? normalizedHeadBlock
            : [
                  ...trimTrailingBlankLines(splitHeadBlockLines(draft.headBlockStr)),
                  `Content-Length: ${getBodyByteLength(draft.bodyStr)}`,
              ].join("\n");
    const preparedHeaders = parseHeaderLinesFromHeadBlock(
        normalizedHeadBlockForPayload,
    );

    return {
        version,
        headBlockStr: finalizeHeadBlockForTransport(normalizedHeadBlockForPayload),
        method: parsedRequest?.method ?? "",
        path: parsedRequest?.path ?? "/",
        host: authorityParts.host,
        port: authorityParts.port,
        headers: buildHeaderMap(preparedHeaders),
        pseudoHeaders: buildPseudoHeaders(
            {
                ...draft,
                host: authorityParts.authority,
            },
            parsedRequest,
            preparedHeaders,
        ),
    };
}

export function buildRepeaterTabLabel(
    sequenceNumber: number,
    parsedRequest: ParsedRepeaterRequest | null,
): string {
    if (!parsedRequest) {
        return `#${sequenceNumber} New`;
    }

    const normalizedPath =
        parsedRequest.path.length > 28
            ? `${parsedRequest.path.slice(0, 25)}...`
            : parsedRequest.path;

    return `#${sequenceNumber} ${parsedRequest.method} ${normalizedPath}`;
}

export function getRepeaterRequestPath(
    parsedRequest: ParsedRepeaterRequest | null,
): string {
    return parsedRequest?.path || "/";
}

export function seedRepeaterRequestDraftFromRequest(
    request: RequestView,
): RepeaterRequestDraft {
    const normalizedSeedRequest = normalizeSeedRequest(request);

    return {
        scheme: request.scheme === "https" ? "https" : "http",
        host: getAuthorityFromRequest(request),
        headBlockStr: normalizedSeedRequest.headBlockStr,
        bodyStr: normalizedSeedRequest.bodyStr,
        skipServerCertVerify: false,
    };
}

export function seedRepeaterRequestDraft(
    entry: HistoryEntry,
): RepeaterRequestDraft | null {
    if (!entry.request) return null;

    return seedRepeaterRequestDraftFromRequest(entry.request);
}

export function createEmptyRepeaterRequestDraft(): RepeaterRequestDraft {
    return {
        scheme: "https",
        host: "",
        headBlockStr: "GET / HTTP/1.1",
        bodyStr: "",
        skipServerCertVerify: false,
    };
}

export function validateRepeaterRequest(
    draft: RepeaterRequestDraft,
): RepeaterValidationResult {
    const issues: RepeaterValidationIssue[] = [];
    const warnings: RepeaterValidationIssue[] = [];
    const headLines = splitHeadBlockLines(draft.headBlockStr);
    const trimmedHeadLines = [...headLines];
    let trailingBlankLines = 0;

    while (trimmedHeadLines.length > 0 && trimmedHeadLines.at(-1) === "") {
        trimmedHeadLines.pop();
        trailingBlankLines += 1;
    }

    if (trailingBlankLines > 1) {
        issues.push(
            createIssue(
                "error",
                headLines.length,
                "The Head Block can only end with a single separating blank line.",
            ),
        );
    }

    if (trimmedHeadLines.length === 0 || trimmedHeadLines[0].trim().length === 0) {
        issues.push(
            createIssue("error", 1, "The request line in the Head Block is required."),
        );
        return {
            parsedRequest: null,
            errors: issues,
            warnings,
        };
    }

    const requestLineMatch = trimmedHeadLines[0].match(requestLinePattern);
    if (!requestLineMatch) {
        issues.push(
            createIssue(
                "error",
                1,
                "The request line must use the format METHOD SP PATH SP HTTP/1.1 or HTTP/2, with exact spaces.",
            ),
        );
    }

    const parsedHeaders: RepeaterHeader[] = [];
    for (let index = 1; index < trimmedHeadLines.length; index += 1) {
        const lineNumber = index + 1;
        const rawLine = trimmedHeadLines[index];

        if (rawLine.trim().length === 0) {
            issues.push(
                createIssue(
                    "error",
                    lineNumber,
                    "There cannot be blank lines inside the Head Block.",
                ),
            );
            continue;
        }

        const headerMatch = rawLine.match(headerPattern);
        if (!headerMatch) {
            issues.push(
                createIssue(
                    "error",
                    lineNumber,
                    "Invalid header. It must use the form Name: value.",
                ),
            );
            continue;
        }

        parsedHeaders.push({
            name: headerMatch[1],
            value: headerMatch[2].replace(/^\s*/, ""),
            line: lineNumber,
        });
    }

    const host = draft.host.trim();
    if (host.length === 0) {
        issues.push(createIssue("error", null, "Host is required."));
    } else {
        if (/\s/.test(host)) {
            issues.push(
                createIssue("error", null, "Host cannot contain spaces."),
            );
        }

        if (host.includes("://") || /[/?#\\]/.test(host)) {
            issues.push(
                createIssue(
                    "error",
                    null,
                    "Enter only host or host:port, without scheme, slashes, query, or path.",
                ),
            );
        }

        if (!authorityPattern.test(host)) {
            issues.push(
                createIssue(
                    "error",
                    null,
                    "The host / authority contains invalid characters or format.",
                ),
            );
        }

        const ipv6Match = host.match(/^\[[0-9A-Fa-f:.%]+\](?::(\d{1,5}))?$/);
        const standardMatch = host.match(/^[A-Za-z0-9.-]+(?::(\d{1,5}))?$/);
        const explicitPort = ipv6Match?.[1] ?? standardMatch?.[1] ?? "";
        if (explicitPort && !isValidPortValue(explicitPort)) {
            issues.push(
                createIssue(
                    "error",
                    null,
                    "The host / authority port must be between 1 and 65535.",
                ),
            );
        }
    }

    if (!requestLineMatch) {
        return {
            parsedRequest: null,
            errors: issues,
            warnings,
        };
    }

    const parsedRequest: ParsedRepeaterRequest = {
        method: requestLineMatch[1],
        path: requestLineMatch[2],
        version: requestLineMatch[3] as ParsedRepeaterRequest["version"],
        headers: parsedHeaders,
    };

    const hostHeader = extractHeaderValues(parsedHeaders, "host")[0] ?? null;
    if (!hostHeader) {
        warnings.push(
            createIssue(
                "warning",
                null,
                "You did not include a Host header in the Head Block.",
            ),
        );
    } else if (host && hostHeader.value.trim() !== host) {
        warnings.push(
            createIssue(
                "warning",
                hostHeader.line,
                "The Host header does not match the external Repeater host.",
            ),
        );
    }

    const contentLength = parseContentLength(parsedHeaders, issues);
    const transferEncodingTokens = parseTransferEncoding(parsedHeaders);
    const transferEncodingLine =
        extractHeaderValues(parsedHeaders, "transfer-encoding")[0]?.line ?? null;
    const bodyByteLength = getBodyByteLength(draft.bodyStr);
    const hasBody = draft.bodyStr.length > 0;

    if (parsedRequest.version === "HTTP/2") {
        const invalidConnectionHeader = parsedHeaders.find((header) =>
            connectionSpecificHeaders.has(header.name.toLowerCase()),
        );

        if (invalidConnectionHeader) {
            warnings.push(
                createIssue(
                    "warning",
                    invalidConnectionHeader.line,
                    "This header is connection-specific and does not fit well with HTTP/2.",
                ),
            );
        }
    }

    if (transferEncodingTokens.length > 0) {
        if (parsedRequest.version === "HTTP/2" && hasBody) {
            issues.push(
                createIssue(
                    "error",
                    transferEncodingLine,
                    "HTTP/2 does not allow Transfer-Encoding, and Repeater needs an explicit Content-Length to send to Go.",
                ),
            );
        } else if (parsedRequest.version === "HTTP/2") {
            issues.push(
                createIssue(
                    "error",
                    transferEncodingLine,
                    "HTTP/2 does not allow Transfer-Encoding in this request format.",
                ),
            );
        } else if (hasBody) {
            issues.push(
                createIssue(
                    "error",
                    transferEncodingLine,
                    "If there is a body, Repeater must send an explicit Content-Length to Go. Remove Transfer-Encoding before resending.",
                ),
            );
        } else {
            warnings.push(
                createIssue(
                    "warning",
                    transferEncodingLine,
                    "Transfer-Encoding is present but the body is empty.",
                ),
            );
        }
    }

    if (transferEncodingTokens.length > 0 && contentLength !== null) {
        issues.push(
            createIssue(
                "error",
                extractHeaderValues(parsedHeaders, "content-length")[0]?.line ?? null,
                "Do not mix Content-Length with Transfer-Encoding.",
            ),
        );
    }

    if (contentLength !== null && contentLength !== bodyByteLength) {
        issues.push(
            createIssue(
                "error",
                extractHeaderValues(parsedHeaders, "content-length")[0]?.line ?? null,
                `Content-Length says ${contentLength} bytes, but the current body is ${bodyByteLength} bytes long.`,
            ),
        );
    }

    if (!hasBody && contentLength !== null && contentLength > 0) {
        issues.push(
            createIssue(
                "error",
                extractHeaderValues(parsedHeaders, "content-length")[0]?.line ?? null,
                "You set Content-Length but the body is empty.",
            ),
        );
    }

    if (hasBody && contentLength === null && transferEncodingTokens.length === 0) {
        warnings.push(
            createIssue(
                "warning",
                null,
                `The request has a body but no Content-Length. It will be added automatically as ${bodyByteLength} when sent to Go.`,
            ),
        );
    }

    return {
        parsedRequest,
        errors: issues,
        warnings,
    };
}
