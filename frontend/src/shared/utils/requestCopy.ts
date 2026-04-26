import type { RequestView } from "@/features/http-history/state/httpHistoryStore";
import { buildRequestUrl } from "./httpEntryTable";

export type RequestCopyTarget =
    | "url"
    | "js-fetch"
    | "js-axios"
    | "python-requests"
    | "python-httpx"
    | "curl"
    | "js-headers"
    | "python-headers";

export type RequestCopyOption = {
    id: RequestCopyTarget;
    label: string;
    description: string;
};

type ParsedRequestHeaders = Array<{
    name: string;
    value: string;
}>;

type CopyRequestContext = {
    url: string;
    method: string;
    headers: ParsedRequestHeaders;
    browserHeadersObject: Record<string, string>;
    scriptHeadersObject: Record<string, string>;
    body: string;
    bodyKind: "none" | "json" | "form" | "raw";
    jsonValue: unknown | null;
    formEntries: Array<[string, string]>;
};

const headerPattern = /^([^:]+):(.*)$/;
const browserRestrictedHeaderNames = new Set([
    "accept-encoding",
    "connection",
    "content-length",
    "cookie",
    "host",
    "origin",
    "referer",
    "user-agent",
]);
const scriptManagedHeaderNames = new Set(["content-length", "host"]);

export const REQUEST_COPY_OPTIONS: RequestCopyOption[] = [
    {
        id: "url",
        label: "URL",
        description: "Copy the full URL with scheme, host, and path.",
    },
    {
        id: "js-fetch",
        label: "JavaScript with fetch",
        description: "Modern snippet for browsers or runtimes with fetch.",
    },
    {
        id: "js-axios",
        label: "JavaScript with axios",
        description: "Request snippet with axios.",
    },
    {
        id: "python-requests",
        label: "Python with requests",
        description: "Request snippet with requests.",
    },
    {
        id: "python-httpx",
        label: "Python with httpx",
        description: "Request snippet with httpx.",
    },
    {
        id: "curl",
        label: "cURL",
        description: "Equivalent curl command.",
    },
    {
        id: "js-headers",
        label: "Headers as JavaScript object",
        description: "Only the headers, ready to paste into JS.",
    },
    {
        id: "python-headers",
        label: "Headers as Python dict",
        description: "Only the headers, ready to paste into Python.",
    },
];

function normalizeLineEndings(value: string): string {
    return value.replace(/\r\n/g, "\n");
}

function trimTrailingBlankLines(lines: string[]): string[] {
    const nextLines = [...lines];

    while (nextLines.length > 0 && nextLines.at(-1)?.trim() === "") {
        nextLines.pop();
    }

    return nextLines;
}

function parseRequestHeaders(headBlockStr: string): ParsedRequestHeaders {
    const lines = trimTrailingBlankLines(normalizeLineEndings(headBlockStr).split("\n"));

    return lines.slice(1).flatMap((line) => {
        const match = line.match(headerPattern);
        if (!match) return [];

        return [
            {
                name: match[1].trim(),
                value: match[2].replace(/^\s*/, ""),
            },
        ];
    });
}

function getPrimaryContentType(headers: ParsedRequestHeaders): string {
    return (
        headers
            .find((header) => header.name.toLowerCase() === "content-type")
            ?.value.split(";", 1)[0]
            ?.trim()
            .toLowerCase() ?? ""
    );
}

function isJsonContentType(contentType: string): boolean {
    return (
        contentType === "application/json" ||
        contentType === "text/json" ||
        contentType.endsWith("+json")
    );
}

function isFormUrlencodedContentType(contentType: string): boolean {
    return contentType === "application/x-www-form-urlencoded";
}

function buildHeadersObject(
    headers: ParsedRequestHeaders,
    blockedHeaderNames: Set<string>,
): Record<string, string> {
    const mergedHeaders = new Map<
        string,
        {
            name: string;
            values: string[];
        }
    >();

    for (const header of headers) {
        const normalizedName = header.name.toLowerCase();
        if (blockedHeaderNames.has(normalizedName)) {
            continue;
        }

        const current = mergedHeaders.get(normalizedName);
        if (!current) {
            mergedHeaders.set(normalizedName, {
                name: header.name,
                values: [header.value],
            });
            continue;
        }

        current.values.push(header.value);
    }

    return Object.fromEntries(
        Array.from(mergedHeaders.values()).map((header) => [
            header.name,
            header.values.join(", "),
        ]),
    );
}

function toPythonLiteral(value: unknown, indentLevel = 0): string {
    const indent = "    ".repeat(indentLevel);
    const childIndent = "    ".repeat(indentLevel + 1);

    if (value === null || value === undefined) return "None";
    if (typeof value === "boolean") return value ? "True" : "False";
    if (typeof value === "number") return Number.isFinite(value) ? String(value) : "None";
    if (typeof value === "string") return JSON.stringify(value);

    if (Array.isArray(value)) {
        if (value.length === 0) return "[]";

        return `[\n${value
            .map((item) => `${childIndent}${toPythonLiteral(item, indentLevel + 1)}`)
            .join(",\n")}\n${indent}]`;
    }

    if (typeof value === "object") {
        const entries = Object.entries(value);
        if (entries.length === 0) return "{}";

        return `{\n${entries
            .map(
                ([key, item]) =>
                    `${childIndent}${JSON.stringify(key)}: ${toPythonLiteral(item, indentLevel + 1)}`,
            )
            .join(",\n")}\n${indent}}`;
    }

    return "None";
}

function escapeShellSingleQuoted(value: string): string {
    return `'${value.replace(/'/g, `'\"'\"'`)}'`;
}

function createCopyRequestContext(request: RequestView): CopyRequestContext {
    const url = buildRequestUrl(request);
    const headers = parseRequestHeaders(request.headBlockStr);
    const body = request.bodyStr ?? "";
    const contentType = getPrimaryContentType(headers);
    const browserHeadersObject = buildHeadersObject(
        headers,
        browserRestrictedHeaderNames,
    );
    const scriptHeadersObject = buildHeadersObject(
        headers,
        scriptManagedHeaderNames,
    );

    if (body.trim().length === 0) {
        return {
            url,
            method: (request.method || "GET").toUpperCase(),
            headers,
            browserHeadersObject,
            scriptHeadersObject,
            body,
            bodyKind: "none",
            jsonValue: null,
            formEntries: [],
        };
    }

    if (isJsonContentType(contentType)) {
        try {
            return {
                url,
                method: (request.method || "GET").toUpperCase(),
                headers,
                browserHeadersObject,
                scriptHeadersObject,
                body,
                bodyKind: "json",
                jsonValue: JSON.parse(body),
                formEntries: [],
            };
        } catch {
            // cae a raw
        }
    }

    if (isFormUrlencodedContentType(contentType)) {
        return {
            url,
            method: (request.method || "GET").toUpperCase(),
            headers,
            browserHeadersObject,
            scriptHeadersObject,
            body,
            bodyKind: "form",
            jsonValue: null,
            formEntries: Array.from(new URLSearchParams(body).entries()),
        };
    }

    return {
        url,
        method: (request.method || "GET").toUpperCase(),
        headers,
        browserHeadersObject,
        scriptHeadersObject,
        body,
        bodyKind: "raw",
        jsonValue: null,
        formEntries: [],
    };
}

function buildJsBodyExpression(context: CopyRequestContext): string | null {
    if (context.bodyKind === "none") {
        return null;
    }

    if (context.bodyKind === "json") {
        return `JSON.stringify(${JSON.stringify(context.jsonValue, null, 2)})`;
    }

    if (context.bodyKind === "form") {
        const formObject = Object.fromEntries(context.formEntries);
        return `new URLSearchParams(${JSON.stringify(formObject, null, 2)})`;
    }

    return JSON.stringify(context.body);
}

function buildPythonBodyArgument(context: CopyRequestContext): {
    argumentName: "json" | "data" | null;
    argumentValue: string | null;
} {
    if (context.bodyKind === "none") {
        return {
            argumentName: null,
            argumentValue: null,
        };
    }

    if (context.bodyKind === "json") {
        return {
            argumentName: "json",
            argumentValue: toPythonLiteral(context.jsonValue),
        };
    }

    if (context.bodyKind === "form") {
        return {
            argumentName: "data",
            argumentValue: toPythonLiteral(Object.fromEntries(context.formEntries)),
        };
    }

    return {
        argumentName: "data",
        argumentValue: JSON.stringify(context.body),
    };
}

function buildCurlSnippet(context: CopyRequestContext): string {
    const lines = [
        `curl -X ${context.method} ${escapeShellSingleQuoted(context.url)}`,
        ...Object.entries(context.scriptHeadersObject).map(
            ([name, value]) =>
                `  -H ${escapeShellSingleQuoted(`${name}: ${value}`)}`,
        ),
    ];

    if (context.bodyKind !== "none") {
        lines.push(`  --data-raw ${escapeShellSingleQuoted(context.body)}`);
    }

    return lines.join(" \\\n");
}

function buildFetchSnippet(context: CopyRequestContext): string {
    const jsBodyExpression = buildJsBodyExpression(context);
    const requestOptions = [
        `method: ${JSON.stringify(context.method)}`,
        `headers: ${JSON.stringify(context.browserHeadersObject, null, 2)}`,
        ...(jsBodyExpression ? [`body: ${jsBodyExpression}`] : []),
    ];

    return `await fetch(${JSON.stringify(context.url)}, {\n${requestOptions
        .map((line) => `  ${line},`)
        .join("\n")}\n});`;
}

function buildAxiosSnippet(context: CopyRequestContext): string {
    const jsBodyExpression = buildJsBodyExpression(context);
    const axiosConfig = [
        `method: ${JSON.stringify(context.method.toLowerCase())}`,
        `url: ${JSON.stringify(context.url)},`,
        `headers: ${JSON.stringify(context.browserHeadersObject, null, 2)},`,
        ...(jsBodyExpression ? [`data: ${jsBodyExpression},`] : []),
        `validateStatus: () => true,`,
    ];

    return `await axios({\n${axiosConfig
        .map((line) => `  ${line}`)
        .join("\n")}\n});`;
}

function buildPythonRequestsSnippet(context: CopyRequestContext): string {
    const pythonBody = buildPythonBodyArgument(context);
    const requestArguments = [
        `headers=${toPythonLiteral(context.scriptHeadersObject)},`,
        ...(pythonBody.argumentName && pythonBody.argumentValue
            ? [`${pythonBody.argumentName}=${pythonBody.argumentValue},`]
            : []),
        `timeout=30,`,
    ];

    return `requests.request(\n    ${JSON.stringify(context.method)},\n    ${JSON.stringify(context.url)},\n${requestArguments
        .map((line) => `    ${line}`)
        .join("\n")}\n)`;
}

function buildPythonHttpxSnippet(context: CopyRequestContext): string {
    const pythonBody = buildPythonBodyArgument(context);
    const requestArguments = [
        `headers=${toPythonLiteral(context.scriptHeadersObject)},`,
        ...(pythonBody.argumentName && pythonBody.argumentValue
            ? [`${pythonBody.argumentName}=${pythonBody.argumentValue},`]
            : []),
        `timeout=30.0,`,
    ];

    return `httpx.request(\n    ${JSON.stringify(context.method)},\n    ${JSON.stringify(context.url)},\n${requestArguments
        .map((line) => `    ${line}`)
        .join("\n")}\n)`;
}

export function buildRequestCopyText(
    request: RequestView,
    target: RequestCopyTarget,
): string {
    const context = createCopyRequestContext(request);

    switch (target) {
        case "url":
            return context.url;
        case "js-fetch":
            return buildFetchSnippet(context);
        case "js-axios":
            return buildAxiosSnippet(context);
        case "python-requests":
            return buildPythonRequestsSnippet(context);
        case "python-httpx":
            return buildPythonHttpxSnippet(context);
        case "curl":
            return buildCurlSnippet(context);
        case "js-headers":
            return JSON.stringify(context.browserHeadersObject, null, 2);
        case "python-headers":
            return toPythonLiteral(context.scriptHeadersObject);
    }
}
