export type PrettyFormEntry = {
    id: string;
    key: string;
    value: string;
};

export type PrettyMultipartHeader = {
    id: string;
    key: string;
    value: string;
};

export type PrettyMultipartPart = {
    id: string;
    name: string;
    filename: string;
    contentType: string;
    headers: PrettyMultipartHeader[];
    bodyText: string;
    displayBody: string;
    bodyKind: "text" | "binary" | "empty";
};

export type HtmlPrettyAttribute = {
    id: string;
    name: string;
    value: string | null;
};

export type HtmlPrettyLine = {
    id: string;
    type: "tag" | "text" | "comment";
    indent: number;
    raw: string;
    tagName?: string;
    closing?: boolean;
    selfClosing?: boolean;
    attributes?: HtmlPrettyAttribute[];
    text?: string;
};

const voidHtmlTags = new Set([
    "area",
    "base",
    "br",
    "col",
    "embed",
    "hr",
    "img",
    "input",
    "link",
    "meta",
    "param",
    "source",
    "track",
    "wbr",
]);

export function getHeaderValue(
    rawHeaders: string,
    headerName: string,
): string | null {
    const expectedHeader = headerName.trim().toLowerCase();

    for (const line of rawHeaders.split(/\r?\n/)) {
        const separatorIndex = line.indexOf(":");
        if (separatorIndex <= 0) continue;

        const currentHeader = line.slice(0, separatorIndex).trim().toLowerCase();
        if (currentHeader !== expectedHeader) continue;

        return line.slice(separatorIndex + 1).trim();
    }

    return null;
}

export function normalizeContentType(contentType: string | null): string {
    return contentType?.split(";")[0]?.trim().toLowerCase() ?? "";
}

export function isJsonContentType(contentType: string): boolean {
    return (
        contentType === "application/json" ||
        contentType === "text/json" ||
        contentType.endsWith("+json")
    );
}

export function isHtmlContentType(contentType: string): boolean {
    return (
        contentType === "text/html" ||
        contentType === "application/xhtml+xml"
    );
}

function isXmlLikeContentType(contentType: string): boolean {
    return (
        contentType === "application/xml" ||
        contentType === "text/xml" ||
        contentType.endsWith("+xml")
    );
}

function isTextLikeContentType(contentType: string): boolean {
    if (!contentType) return true;
    if (contentType.startsWith("text/")) return true;
    if (isJsonContentType(contentType)) return true;
    if (isHtmlContentType(contentType)) return true;
    if (isXmlLikeContentType(contentType)) return true;

    return (
        contentType === "application/javascript" ||
        contentType === "application/x-javascript" ||
        contentType === "application/x-www-form-urlencoded"
    );
}

export function parseFormUrlEncodedBody(rawBody: string): PrettyFormEntry[] {
    const params = new URLSearchParams(rawBody);

    return Array.from(params.entries()).map(([key, value], index) => ({
        id: `${index}-${key}`,
        key,
        value,
    }));
}

export function extractMultipartBoundary(
    contentTypeHeader: string | null,
): string | null {
    if (!contentTypeHeader) return null;

    const match =
        contentTypeHeader.match(/boundary="([^"]+)"/i) ??
        contentTypeHeader.match(/boundary=([^;]+)/i);

    return match?.[1]?.trim() ?? null;
}

function parseMultipartHeaders(rawHeaderBlock: string): PrettyMultipartHeader[] {
    return rawHeaderBlock
        .split(/\r?\n/)
        .map((line, index) => {
            const separatorIndex = line.indexOf(":");
            if (separatorIndex <= 0) {
                return null;
            }

            return {
                id: `${index}-${line.slice(0, separatorIndex).trim()}`,
                key: line.slice(0, separatorIndex).trim(),
                value: line.slice(separatorIndex + 1).trim(),
            };
        })
        .filter((header): header is PrettyMultipartHeader => header !== null);
}

function getMultipartHeaderValue(
    headers: PrettyMultipartHeader[],
    headerName: string,
): string {
    const expectedHeader = headerName.trim().toLowerCase();

    const match = headers.find(
        (header) => header.key.trim().toLowerCase() === expectedHeader,
    );

    return match?.value ?? "";
}

function parseContentDisposition(value: string): {
    name: string;
    filename: string;
} {
    const name =
        value.match(/(?:^|;)\s*name="([^"]*)"/i)?.[1] ??
        value.match(/(?:^|;)\s*name=([^;]+)/i)?.[1] ??
        "";
    const filename =
        value.match(/(?:^|;)\s*filename="([^"]*)"/i)?.[1] ??
        value.match(/(?:^|;)\s*filename=([^;]+)/i)?.[1] ??
        "";

    return {
        name: name.trim(),
        filename: filename.trim(),
    };
}

function resolveMultipartBodyDisplay(
    contentType: string,
    bodyText: string,
    hasFilename: boolean,
): {
    displayBody: string;
    bodyKind: "text" | "binary" | "empty";
} {
    if (bodyText.trim().length === 0) {
        return {
            displayBody: "(no content)",
            bodyKind: "empty",
        };
    }

    if (isTextLikeContentType(contentType) || (!contentType && !hasFilename)) {
        return {
            displayBody: bodyText,
            bodyKind: "text",
        };
    }

    return {
        displayBody: `[binary content omitted, ${bodyText.length} characters]`,
        bodyKind: "binary",
    };
}

export function parseMultipartBody(
    contentTypeHeader: string | null,
    rawBody: string,
): PrettyMultipartPart[] | null {
    const boundary = extractMultipartBoundary(contentTypeHeader);
    if (!boundary) return null;

    const delimiter = `--${boundary}`;
    const sections = rawBody.split(delimiter);
    const parts: PrettyMultipartPart[] = [];

    for (const [index, originalSection] of sections.entries()) {
        let section = originalSection;
        if (!section) continue;

        section = section.replace(/^\r?\n/, "").replace(/\r?\n$/, "");
        if (!section || section === "--") continue;

        if (section.endsWith("--")) {
            section = section.slice(0, -2).replace(/\r?\n$/, "");
        }

        const headerBodySeparator = section.search(/\r?\n\r?\n/);
        if (headerBodySeparator < 0) continue;

        const rawHeaderBlock = section.slice(0, headerBodySeparator).trim();
        const rawPartBody = section
            .slice(headerBodySeparator)
            .replace(/^\r?\n\r?\n/, "")
            .replace(/\r?\n$/, "");

        const headers = parseMultipartHeaders(rawHeaderBlock);
        const contentDisposition = getMultipartHeaderValue(
            headers,
            "content-disposition",
        );
        const { name, filename } = parseContentDisposition(contentDisposition);
        const contentType = normalizeContentType(
            getMultipartHeaderValue(headers, "content-type"),
        );
        const { displayBody, bodyKind } = resolveMultipartBodyDisplay(
            contentType,
            rawPartBody,
            filename.length > 0,
        );

        parts.push({
            id: `part-${index}`,
            name,
            filename,
            contentType,
            headers,
            bodyText: rawPartBody,
            displayBody,
            bodyKind,
        });
    }

    return parts;
}

export function buildMultipartPrettySearchText(
    parts: PrettyMultipartPart[],
): string {
    return parts
        .map((part, index) => {
            const metaLines = [
                `Part ${index + 1}`,
                part.name ? `Name: ${part.name}` : "",
                part.filename ? `Filename: ${part.filename}` : "",
                part.contentType ? `Content-Type: ${part.contentType}` : "",
            ].filter(Boolean);

            const headerLines = part.headers.map(
                (header) => `${header.key}: ${header.value}`,
            );

            return [...metaLines, ...headerLines, part.displayBody].join("\n");
        })
        .join("\n\n");
}

function parseHtmlAttributes(rawTag: string): HtmlPrettyAttribute[] {
    const withoutBrackets = rawTag
        .replace(/^<\s*\/?/, "")
        .replace(/\/?\s*>$/, "")
        .trim();
    const firstSpace = withoutBrackets.search(/\s/);
    if (firstSpace < 0) return [];

    const attributeSource = withoutBrackets.slice(firstSpace).trim();
    const attributes: HtmlPrettyAttribute[] = [];
    const attributePattern =
        /([^\s=/>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+)))?/g;

    let match: RegExpExecArray | null = attributePattern.exec(attributeSource);
    let index = 0;

    while (match) {
        attributes.push({
            id: `attr-${index}-${match[1]}`,
            name: match[1],
            value: match[2] ?? match[3] ?? match[4] ?? null,
        });

        index += 1;
        match = attributePattern.exec(attributeSource);
    }

    return attributes;
}

export function formatHtmlPretty(rawHtml: string): HtmlPrettyLine[] {
    const tokens =
        rawHtml.match(/<!--[\s\S]*?-->|<\/?[^>]+>|[^<]+/g) ?? [];
    const lines: HtmlPrettyLine[] = [];
    let indent = 0;

    for (const token of tokens) {
        const normalizedToken = token.replace(/\r/g, "");
        if (!normalizedToken) continue;

        if (normalizedToken.startsWith("<!--")) {
            lines.push({
                id: `line-${lines.length}`,
                type: "comment",
                indent,
                raw: normalizedToken.trim(),
                text: normalizedToken.trim(),
            });
            continue;
        }

        if (normalizedToken.startsWith("</")) {
            indent = Math.max(0, indent - 1);
            const tagName =
                normalizedToken.match(/^<\/\s*([^\s>]+)/)?.[1] ?? "";

            lines.push({
                id: `line-${lines.length}`,
                type: "tag",
                indent,
                raw: normalizedToken.trim(),
                tagName,
                closing: true,
                selfClosing: false,
                attributes: [],
            });
            continue;
        }

        if (normalizedToken.startsWith("<")) {
            const raw = normalizedToken.trim();
            const tagName = raw.match(/^<\s*([^\s/>]+)/)?.[1] ?? "";
            const selfClosing =
                raw.endsWith("/>") || voidHtmlTags.has(tagName.toLowerCase());

            lines.push({
                id: `line-${lines.length}`,
                type: "tag",
                indent,
                raw,
                tagName,
                closing: false,
                selfClosing,
                attributes: parseHtmlAttributes(raw),
            });

            if (!selfClosing) {
                indent += 1;
            }
            continue;
        }

        const normalizedText = normalizedToken.replace(/\s+/g, " ").trim();
        if (!normalizedText) continue;

        lines.push({
            id: `line-${lines.length}`,
            type: "text",
            indent,
            raw: normalizedText,
            text: normalizedText,
        });
    }

    return lines;
}

export function buildHtmlPrettySearchText(lines: HtmlPrettyLine[]): string {
    return lines
        .map((line) => `${"  ".repeat(line.indent)}${line.raw}`)
        .join("\n");
}
