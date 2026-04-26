import type { HistoryEntry } from "../state/httpHistoryStore";

export type HistoryFilterTarget =
    | "all"
    | "request"
    | "response"
    | "requestHead"
    | "responseHead"
    | "requestBody"
    | "responseBody"
    | "head"
    | "body"
    | "method"
    | "host"
    | "port"
    | "scheme"
    | "path";

export type HistoryFilterOperator = "and" | "or";
export type HistoryFilterMatchMode =
    | "contains"
    | "notContains"
    | "equals"
    | "notEquals"
    | "startsWith"
    | "endsWith";

export type HistoryFilterCondition = {
    id: string;
    query: string;
    target: HistoryFilterTarget;
    matchMode: HistoryFilterMatchMode;
};

export const HISTORY_FILTER_TARGET_VALUES: HistoryFilterTarget[] = [
    "all",
    "request",
    "response",
    "requestHead",
    "responseHead",
    "requestBody",
    "responseBody",
    "method",
    "host",
    "port",
    "scheme",
    "path",
    "head",
    "body",
];
const HISTORY_FILTER_TARGET_LABELS: Record<HistoryFilterTarget, string> = {
    all: "All requests and responses",
    request: "Request only",
    response: "Response only",
    requestHead: "Request head blocks",
    responseHead: "Response head blocks",
    requestBody: "Request bodies",
    responseBody: "Response bodies",
    method: "HTTP method",
    host: "Host / domain",
    port: "Port",
    scheme: "Scheme",
    path: "Path",
    head: "All head blocks",
    body: "All bodies",
};
export const HISTORY_FILTER_TARGET_OPTIONS: Array<{
    value: HistoryFilterTarget;
    label: string;
}> = HISTORY_FILTER_TARGET_VALUES.map((value) => ({
    value,
    label: HISTORY_FILTER_TARGET_LABELS[value],
}));
export const HISTORY_FILTER_MATCH_MODE_VALUES: HistoryFilterMatchMode[] = [
    "contains",
    "notContains",
    "equals",
    "notEquals",
    "startsWith",
    "endsWith",
];
export const HISTORY_FILTER_MATCH_MODE_OPTIONS: Array<{
    value: HistoryFilterMatchMode;
    label: string;
}> = HISTORY_FILTER_MATCH_MODE_VALUES.map((value) => ({
    value,
    label:
        value === "contains"
            ? "Contains"
            : value === "notContains"
              ? "Does not contain"
              : value === "equals"
                ? "Exactly equals"
                : value === "notEquals"
                  ? "Does not exactly equal"
                  : value === "startsWith"
                    ? "Starts with"
                    : "Ends with",
}));

let conditionCounter = 0;
const entryIndexCache = new WeakMap<
    HistoryEntry,
    Record<HistoryFilterTarget, string>
>();

function nextConditionId(): string {
    conditionCounter += 1;
    return `history-filter-condition-${conditionCounter}`;
}

function joinSegments(...values: string[]): string {
    return values.filter((value) => value.trim().length > 0).join("\n");
}

function joinDistinctSegments(...values: string[]): string {
    return [...new Set(values.filter((value) => value.trim().length > 0))].join(
        "\n",
    );
}

function normalizeSearchableText(value: string): string {
    return value.toLocaleLowerCase();
}

function buildEntryIndex(
    entry: HistoryEntry,
): Record<HistoryFilterTarget, string> {
    const cachedEntryIndex = entryIndexCache.get(entry);
    if (cachedEntryIndex) {
        return cachedEntryIndex;
    }

    const requestHead = entry.request?.headBlockStr ?? "";
    const requestBody = entry.request?.bodyStr ?? "";
    const responseHead = entry.response?.headBlockStr ?? "";
    const responseBody = entry.response?.bodyStr ?? "";
    const method = joinDistinctSegments(
        entry.request?.method ?? "",
        entry.response?.method ?? "",
    );
    const host = joinDistinctSegments(
        entry.request?.host ?? "",
        entry.response?.host ?? "",
    );
    const port = joinDistinctSegments(
        entry.request?.port ?? "",
        entry.response?.port ?? "",
    );
    const scheme = joinDistinctSegments(
        entry.request?.scheme ?? "",
        entry.response?.scheme ?? "",
    );
    const path = joinDistinctSegments(
        entry.request?.path ?? "",
        entry.response?.path ?? "",
    );

    const nextEntryIndex = {
        all: normalizeSearchableText(
            joinSegments(requestHead, requestBody, responseHead, responseBody),
        ),
        request: normalizeSearchableText(joinSegments(requestHead, requestBody)),
        response: normalizeSearchableText(
            joinSegments(responseHead, responseBody),
        ),
        requestHead: normalizeSearchableText(requestHead),
        responseHead: normalizeSearchableText(responseHead),
        requestBody: normalizeSearchableText(requestBody),
        responseBody: normalizeSearchableText(responseBody),
        head: normalizeSearchableText(joinSegments(requestHead, responseHead)),
        body: normalizeSearchableText(joinSegments(requestBody, responseBody)),
        method: normalizeSearchableText(method),
        host: normalizeSearchableText(host),
        port: normalizeSearchableText(port),
        scheme: normalizeSearchableText(scheme),
        path: normalizeSearchableText(path),
    };

    entryIndexCache.set(entry, nextEntryIndex);
    return nextEntryIndex;
}

function getHistoryFilterTargetLabel(target: HistoryFilterTarget): string {
    return HISTORY_FILTER_TARGET_LABELS[target];
}

export function createHistoryFilterCondition(
    query = "",
    target: HistoryFilterTarget = "all",
    matchMode: HistoryFilterMatchMode = "contains",
): HistoryFilterCondition {
    return {
        id: nextConditionId(),
        query,
        target,
        matchMode,
    };
}

export function cloneHistoryFilterConditions(
    conditions: HistoryFilterCondition[],
): HistoryFilterCondition[] {
    if (conditions.length === 0) {
        return [createHistoryFilterCondition()];
    }

    return conditions.map((condition) =>
        createHistoryFilterCondition(
            condition.query,
            condition.target,
            condition.matchMode,
        ),
    );
}

function matchesFilterCondition(
    candidate: string,
    condition: HistoryFilterCondition,
): boolean {
    const normalizedQuery = normalizeSearchableText(condition.query);

    switch (condition.matchMode) {
        case "contains":
            return candidate.includes(normalizedQuery);
        case "notContains":
            return !candidate.includes(normalizedQuery);
        case "equals":
            return candidate === normalizedQuery;
        case "notEquals":
            return candidate !== normalizedQuery;
        case "startsWith":
            return candidate.startsWith(normalizedQuery);
        case "endsWith":
            return candidate.endsWith(normalizedQuery);
    }
}

export function sanitizeHistoryFilterConditions(
    conditions: HistoryFilterCondition[],
): HistoryFilterCondition[] {
    return conditions
        .map((condition) => ({
            ...condition,
            query: condition.query.trim(),
        }))
        .filter((condition) => condition.query.length > 0);
}

export function entryMatchesHistoryFilter(
    entry: HistoryEntry,
    conditions: HistoryFilterCondition[],
    operator: HistoryFilterOperator,
): boolean {
    const validConditions = sanitizeHistoryFilterConditions(conditions);
    if (validConditions.length === 0) return true;

    const entryIndex = buildEntryIndex(entry);
    const conditionMatches = validConditions.map((condition) =>
        matchesFilterCondition(entryIndex[condition.target], condition),
    );

    return operator === "or"
        ? conditionMatches.some(Boolean)
        : conditionMatches.every(Boolean);
}

export function getHistoryFilterSummary(
    conditions: HistoryFilterCondition[],
    operator: HistoryFilterOperator,
): string {
    const validConditions = sanitizeHistoryFilterConditions(conditions);

    if (validConditions.length === 0) {
        return "No conditions";
    }

    if (validConditions.length === 1) {
        const [condition] = validConditions;
        const targetLabel = getHistoryFilterTargetLabel(condition.target);
        const matchLabel =
            HISTORY_FILTER_MATCH_MODE_OPTIONS.find(
                (option) => option.value === condition.matchMode,
            )?.label ?? "Contains";

        return `${targetLabel} | ${matchLabel}: "${condition.query}"`;
    }

    return `${validConditions.length} conditions | ${operator.toUpperCase()}`;
}
