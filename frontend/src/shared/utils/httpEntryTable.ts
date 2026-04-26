import type {
    SortDirection,
    SortKey,
} from "@/features/http-history/state/httpHistoryStore";

export type TableColumnConfig<TKey extends string> = {
    key: TKey;
    label: string;
    width: number;
    minWidth: number;
};

export type HttpRequestRecord = {
    host: string;
    scheme?: string;
    port: string;
    headBlockStr?: string;
    bodyStr?: string;
    version: string;
    method: string;
    path: string;
};

export type HttpResponseRecord = {
    host: string;
    scheme?: string;
    port: string;
    headBlockStr?: string;
    bodyStr?: string;
    version: string;
    method?: string;
    path?: string;
    statusCode: number | null;
};

export type HttpEntryRecord = {
    request: HttpRequestRecord | null;
    response: HttpResponseRecord | null;
};

export function isNaturalTimeSort(
    sortKey: SortKey,
    sortDirection: SortDirection,
): boolean {
    return sortKey === "time" && sortDirection === "asc";
}

export const httpEntryColumns: TableColumnConfig<SortKey>[] = [
    { key: "time", label: "Time", width: 110, minWidth: 90 },
    { key: "host", label: "Host", width: 320, minWidth: 220 },
    { key: "path", label: "Path", width: 260, minWidth: 180 },
    { key: "port", label: "Port", width: 90, minWidth: 70 },
    { key: "method", label: "Method", width: 110, minWidth: 90 },
    { key: "version", label: "Version", width: 120, minWidth: 90 },
    { key: "statusCode", label: "Status", width: 120, minWidth: 90 },
];

export function buildColumnWidthMap<TKey extends string>(
    columns: readonly TableColumnConfig<TKey>[],
): Record<TKey, number> {
    return Object.fromEntries(
        columns.map((column) => [column.key, column.width]),
    ) as Record<TKey, number>;
}

export function buildColumnTemplate<TKey extends string>(
    columns: readonly TableColumnConfig<TKey>[],
    widths: Record<TKey, number>,
): string {
    return columns.map((column) => `${widths[column.key]}px`).join(" ");
}

export function formatHttpHost(
    scheme: string,
    host: string,
): string {
    if (!host) return "-";
    if (!scheme) return host;

    return `${scheme}://${host}`;
}

function formatUrlAuthority(
    host: string,
    port: string,
    scheme: string,
): string {
    const trimmedHost = host.trim();
    if (!trimmedHost) return "";

    const formattedHost =
        trimmedHost.includes(":") && !trimmedHost.startsWith("[")
            ? `[${trimmedHost}]`
            : trimmedHost;
    const defaultPort =
        scheme === "https" ? "443" : scheme === "http" ? "80" : "";
    const trimmedPort = port.trim();

    if (!trimmedPort || trimmedPort === defaultPort) {
        return formattedHost;
    }

    return `${formattedHost}:${trimmedPort}`;
}

function normalizeUrlPath(path: string): string {
    const trimmedPath = path.trim();
    if (!trimmedPath) {
        return "/";
    }

    if (
        trimmedPath.startsWith("/") ||
        trimmedPath.startsWith("?") ||
        trimmedPath.startsWith("#")
    ) {
        return trimmedPath;
    }

    return `/${trimmedPath}`;
}

export function getEntryHost(entry: HttpEntryRecord): string {
    return formatHttpHost(
        entry.request?.scheme || entry.response?.scheme || "",
        entry.request?.host || entry.response?.host || "",
    );
}

export function getEntryPath(entry: HttpEntryRecord): string {
    return entry.request?.path || entry.response?.path || "-";
}

export function buildRequestUrl(
    request: HttpRequestRecord | null,
): string {
    const scheme = request?.scheme || "";
    const host = request?.host || "";
    const port = request?.port || "";
    const path = request?.path || "/";

    if (!scheme || !host) {
        return "";
    }

    return `${scheme}://${formatUrlAuthority(host, port, scheme)}${normalizeUrlPath(path)}`;
}

export function buildEntryUrl(entry: HttpEntryRecord): string {
    if (entry.request) {
        return buildRequestUrl(entry.request);
    }

    const response = entry.response;
    if (!response) {
        return "";
    }

    return buildRequestUrl({
        host: response.host,
        scheme: response.scheme,
        port: response.port,
        version: response.version,
        method: response.method ?? "",
        path: response.path ?? "/",
        headBlockStr: response.headBlockStr,
        bodyStr: response.bodyStr,
    });
}

export function getEntryPort(entry: HttpEntryRecord): string {
    return entry.request?.port || entry.response?.port || "-";
}

export function getEntryMethod(entry: HttpEntryRecord): string {
    return entry.request?.method || entry.response?.method || "-";
}

export function getEntryVersion(entry: HttpEntryRecord): string {
    return entry.request?.version || entry.response?.version || "-";
}

export function getEntryStatusCode(entry: HttpEntryRecord): number | null {
    return entry.response?.statusCode ?? null;
}

export function getStatusLabelFromCode(statusCode: number | null): string {
    if (statusCode === null) return "Pending";
    return String(statusCode);
}

export function getEntryStatusLabel(entry: HttpEntryRecord): string {
    return getStatusLabelFromCode(getEntryStatusCode(entry));
}

export function getStatusToneFromCode(statusCode: number | null): string {
    if (statusCode === null) return "pending";
    if (statusCode >= 200 && statusCode < 300) return "success";
    if (statusCode >= 300 && statusCode < 400) return "redirect";
    if (statusCode >= 400 && statusCode < 500) return "warning";
    if (statusCode >= 500) return "danger";

    return "neutral";
}

export function getEntryStatusTone(entry: HttpEntryRecord): string {
    return getStatusToneFromCode(getEntryStatusCode(entry));
}

export function buildRequestLine(request: HttpRequestRecord | null): string {
    if (!request) return "Request pending";

    const parts = [request.method, request.path, request.version]
        .map((value) => value.trim())
        .filter(Boolean);

    return parts.length > 0 ? parts.join(" ") : "Request with no start line";
}

export function buildResponseLine(response: HttpResponseRecord | null): string {
    if (!response) return "Waiting for response";

    const parts = [
        response.version,
        response.statusCode === null ? "" : String(response.statusCode),
    ]
        .map((value) => value.trim())
        .filter(Boolean);

    return parts.length > 0 ? parts.join(" ") : "Response with no start line";
}

function compareText(
    left: string,
    right: string,
): number {
    return left.localeCompare(right, undefined, {
        numeric: true,
        sensitivity: "base",
    });
}

function compareNumber(
    left: number,
    right: number,
): number {
    if (left === right) return 0;
    return left > right ? 1 : -1;
}

export function compareHttpEntriesByColumn<T extends HttpEntryRecord>(
    left: T,
    right: T,
    sortKey: SortKey,
    sortDirection: SortDirection,
    getTimeValue: (entry: T) => number,
    getSequence: (entry: T) => number,
): number {
    const factor = sortDirection === "asc" ? 1 : -1;
    let comparison = 0;

    switch (sortKey) {
        case "time":
            comparison = compareNumber(
                getTimeValue(left),
                getTimeValue(right),
            );
            break;
        case "host":
            comparison = compareText(getEntryHost(left), getEntryHost(right));
            break;
        case "path":
            comparison = compareText(getEntryPath(left), getEntryPath(right));
            break;
        case "port":
            comparison = compareText(getEntryPort(left), getEntryPort(right));
            break;
        case "method":
            comparison = compareText(getEntryMethod(left), getEntryMethod(right));
            break;
        case "version":
            comparison = compareText(
                getEntryVersion(left),
                getEntryVersion(right),
            );
            break;
        case "statusCode":
            comparison = compareNumber(
                getEntryStatusCode(left) ?? -1,
                getEntryStatusCode(right) ?? -1,
            );
            break;
    }

    if (comparison !== 0) {
        return comparison * factor;
    }

    return compareNumber(getSequence(left), getSequence(right));
}

export function sortHttpEntriesByColumn<T extends HttpEntryRecord>(
    entries: readonly T[],
    sortKey: SortKey,
    sortDirection: SortDirection,
    getTimeValue: (entry: T) => number,
    getSequence: (entry: T) => number,
): T[] {
    if (entries.length <= 1 || isNaturalTimeSort(sortKey, sortDirection)) {
        return entries as T[];
    }

    return [...entries].sort((left, right) =>
        compareHttpEntriesByColumn(
            left,
            right,
            sortKey,
            sortDirection,
            getTimeValue,
            getSequence,
        ),
    );
}
