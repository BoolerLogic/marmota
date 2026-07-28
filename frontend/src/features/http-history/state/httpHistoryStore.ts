import { writable } from "svelte/store";
import { EventsOn } from "../../../../wailsjs/runtime/runtime.js";
import {
    ClearHistoryEntries,
    GetHistoryFilterMatchesForEntries,
    RemoveActiveHistoryFilter,
    RemoveHistoryEntry,
    UpsertActiveHistoryFilter,
} from "../../../../wailsjs/go/main/App.js";
import { bridge } from "../../../../wailsjs/go/models";
import {
    clearHistoryDetailCache,
    invalidateHistoryEntryDetail,
    removeHistoryEntryDetail,
    type RequestView,
    type ResponseView,
} from "./historyDetailCache";
import type {
    HistoryFilterCondition,
    HistoryFilterOperator,
} from "../utils/historyFilters";

export type SortKey =
    | "time"
    | "host"
    | "path"
    | "port"
    | "method"
    | "version"
    | "statusCode";

export type SortDirection = "asc" | "desc";

export type RequestSummary = {
    id: string;
    host: string;
    scheme: string;
    port: string;
    headBlockStr: string;
    bodyStr: string;
    version: string;
    method: string;
    path: string;
    receivedAtMs: number;
};

export type ResponseSummary = {
    id: string;
    host: string;
    scheme: string;
    port: string;
    headBlockStr: string;
    bodyStr: string;
    version: string;
    method: string;
    path: string;
    statusCode: number | null;
    receivedAtMs: number;
    unsupportedContentEncodings: string[];
    contentDecodingFailed: boolean;
};

export type HistoryEntry = {
    id: string;
    request: RequestSummary | null;
    response: ResponseSummary | null;
    firstSeenAtMs: number;
    requestArrivedAtMs: number | null;
    requestTimeLabel: string;
    sequence: number;
    read: boolean;
};

type HttpHistoryState = {
    entries: HistoryEntry[];
    entriesById: Map<string, HistoryEntry>;
    selectedId: string | null;
    unreadCount: number;
    filterEntryIdsByKey: Map<string, string[]>;
    syncingFilterKeys: Set<string>;
};

type HistoryFilterTabConfig = {
    id: string;
    filterVersion: number;
    conditions: HistoryFilterCondition[];
    operator: HistoryFilterOperator;
};

type HistoryRequestSummaryPayload = {
    id: number | string;
    host?: string;
    port?: string;
    version?: string;
    method?: string;
    path?: string;
    scheme?: string;
    receivedAtMs?: number;
    filterMatches?: bridge.HistoryFilterMatch[];
    evaluatedFilters?: bridge.HistoryFilterMatch[];
};

type HistoryResponseSummaryPayload = {
    id: number | string;
    host?: string;
    port?: string;
    version?: string;
    statusCode?: number;
    receivedAtMs?: number;
    filterMatches?: bridge.HistoryFilterMatch[];
    evaluatedFilters?: bridge.HistoryFilterMatch[];
    unsupportedContentEncodings?: string[];
    contentDecodingFailed?: boolean;
};

const localTimeFormatter = new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
});

const state: HttpHistoryState = {
    entries: [],
    entriesById: new Map<string, HistoryEntry>(),
    selectedId: null,
    unreadCount: 0,
    filterEntryIdsByKey: new Map<string, string[]>(),
    syncingFilterKeys: new Set<string>(),
};

const { subscribe, set } = writable<HttpHistoryState>(state);

let captureStarted = false;
let sequenceCounter = 0;
let httpHistoryPanelActive = false;

const filterKeysByEntryId = new Map<string, Set<string>>();
const pendingEntryIdsByFilterKey = new Map<string, Set<string>>();
const activeFilterVersionsById = new Map<string, number>();
type FilterUpsertOperation = {
    filterId: string;
    filterKey: string;
    mutationToken: symbol;
    pendingEntryIds: Set<string>;
};
const pendingFilterUpsertsById = new Map<string, FilterUpsertOperation>();
const latestFilterMutationTokensById = new Map<string, symbol>();

function publishHistoryState() {
    set(state);
}

function formatLocalTime(timestampMs: number): string {
    return localTimeFormatter.format(new Date(timestampMs));
}

function normalizeFilterMatches(
    matches: bridge.HistoryFilterMatch[] | null | undefined,
): bridge.HistoryFilterMatch[] {
    if (!Array.isArray(matches) || matches.length === 0) {
        return [];
    }

    return matches.flatMap((match) => {
        if (
            !match ||
            typeof match.filterId !== "string" ||
            !Number.isFinite(match.version)
        ) {
            return [];
        }

        return [
            {
                filterId: match.filterId,
                version: Math.max(0, Math.trunc(match.version)),
            },
        ];
    });
}

function normalizeRequestSummary(
    payload: HistoryRequestSummaryPayload,
): RequestSummary {
    return {
        id: String(payload.id),
        host: payload.host ?? "",
        scheme: payload.scheme ?? "",
        port: payload.port ?? "",
        headBlockStr: "",
        bodyStr: "",
        version: payload.version ?? "",
        method: payload.method ?? "",
        path: payload.path ?? "",
        receivedAtMs: payload.receivedAtMs ?? Date.now(),
    };
}

function normalizeResponseSummary(
    payload: HistoryResponseSummaryPayload,
): ResponseSummary {
    return {
        id: String(payload.id),
        host: payload.host ?? "",
        scheme: "",
        port: payload.port ?? "",
        headBlockStr: "",
        bodyStr: "",
        version: payload.version ?? "",
        method: "",
        path: "",
        statusCode: payload.statusCode ?? null,
        receivedAtMs: payload.receivedAtMs ?? Date.now(),
        unsupportedContentEncodings: normalizeStringArray(
            payload.unsupportedContentEncodings,
        ),
        contentDecodingFailed: payload.contentDecodingFailed ?? false,
    };
}

function normalizeStringArray(
    values: string[] | null | undefined,
): string[] {
    if (!Array.isArray(values)) {
        return [];
    }

    return Array.from(
        new Set(
            values
                .filter((value): value is string => typeof value === "string")
                .map((value) => value.trim().toLowerCase())
                .filter(Boolean),
        ),
    );
}

export function getHistoryFilterKey(filterId: string, version: number): string {
    return `${filterId}::${version}`;
}

function setFilterSyncing(filterKey: string, syncing: boolean) {
    if (syncing) {
        state.syncingFilterKeys.add(filterKey);
    } else {
        state.syncingFilterKeys.delete(filterKey);
    }
}

function isLatestFilterUpsert(operation: FilterUpsertOperation): boolean {
    return (
        latestFilterMutationTokensById.get(operation.filterId) ===
        operation.mutationToken
    );
}

function releaseFilterUpsertSync(operation: FilterUpsertOperation): boolean {
    if (
        pendingEntryIdsByFilterKey.get(operation.filterKey) !==
        operation.pendingEntryIds
    ) {
        return false;
    }

    pendingEntryIdsByFilterKey.delete(operation.filterKey);
    setFilterSyncing(operation.filterKey, false);
    return true;
}

function moveFilterUpsertSync(
    operation: FilterUpsertOperation,
    nextFilterKey: string,
) {
    if (operation.filterKey === nextFilterKey) {
        return;
    }

    releaseFilterUpsertSync(operation);
    operation.filterKey = nextFilterKey;
    pendingEntryIdsByFilterKey.set(
        operation.filterKey,
        operation.pendingEntryIds,
    );
    setFilterSyncing(operation.filterKey, true);
}

function supersedePendingFilterUpsert(filterId: string) {
    const pendingOperation = pendingFilterUpsertsById.get(filterId);
    if (!pendingOperation) {
        return;
    }

    pendingFilterUpsertsById.delete(filterId);
    releaseFilterUpsertSync(pendingOperation);
}

function removeEntryIdFromArray(entryIds: string[], entryId: string): boolean {
    const currentIndex = entryIds.indexOf(entryId);
    if (currentIndex === -1) {
        return false;
    }

    entryIds.splice(currentIndex, 1);
    return true;
}

function addEntryToFilterMembership(filterKey: string, entryId: string) {
    const entry = state.entriesById.get(entryId);
    if (!entry) {
        return;
    }

    const currentEntryIds = state.filterEntryIdsByKey.get(filterKey);
    if (currentEntryIds) {
        if (currentEntryIds.includes(entryId)) {
            return;
        }

        const insertionIndex = currentEntryIds.findIndex((currentEntryId) => {
            const currentEntry = state.entriesById.get(currentEntryId);
            return currentEntry ? currentEntry.sequence > entry.sequence : false;
        });

        if (insertionIndex === -1) {
            currentEntryIds.push(entryId);
        } else {
            currentEntryIds.splice(insertionIndex, 0, entryId);
        }
    } else {
        state.filterEntryIdsByKey.set(filterKey, [entryId]);
    }
}

function removeEntryFromFilterMembership(filterKey: string, entryId: string) {
    const currentEntryIds = state.filterEntryIdsByKey.get(filterKey);
    if (!currentEntryIds) {
        return;
    }

    if (!removeEntryIdFromArray(currentEntryIds, entryId)) {
        return;
    }

    if (currentEntryIds.length === 0) {
        state.filterEntryIdsByKey.delete(filterKey);
    }
}

function removeAllFilterMembershipsForFilterId(
    filterId: string,
    keepFilterKey?: string,
) {
    const filterKeyPrefix = `${filterId}::`;
    const filterKeysToRemove = Array.from(state.filterEntryIdsByKey.keys()).filter(
        (filterKey) =>
            filterKey.startsWith(filterKeyPrefix) && filterKey !== keepFilterKey,
    );

    for (const filterKey of filterKeysToRemove) {
        const entryIds = state.filterEntryIdsByKey.get(filterKey) ?? [];

        for (const entryId of entryIds) {
            const currentFilterKeys = filterKeysByEntryId.get(entryId);
            if (!currentFilterKeys) {
                continue;
            }

            currentFilterKeys.delete(filterKey);
            if (currentFilterKeys.size === 0) {
                filterKeysByEntryId.delete(entryId);
            }
        }

        state.filterEntryIdsByKey.delete(filterKey);
    }
}

function removeAllFilterMembershipsForEntryId(entryId: string) {
    const currentFilterKeys = filterKeysByEntryId.get(entryId);
    if (!currentFilterKeys) {
        return;
    }

    for (const filterKey of currentFilterKeys) {
        removeEntryFromFilterMembership(filterKey, entryId);
    }

    filterKeysByEntryId.delete(entryId);
}

function replaceFilterMembership(filterKey: string, entryIds: string[]) {
    const dedupedEntryIds: string[] = [];
    const seenEntryIds = new Set<string>();

    for (const entryId of entryIds) {
        if (!state.entriesById.has(entryId) || seenEntryIds.has(entryId)) {
            continue;
        }

        seenEntryIds.add(entryId);
        dedupedEntryIds.push(entryId);
    }
    const dedupedEntryIdSet = new Set(dedupedEntryIds);
    const naturallyOrderedEntryIds = state.entries
        .map((entry) => entry.id)
        .filter((entryId) => dedupedEntryIdSet.has(entryId));

    const previousEntryIds = state.filterEntryIdsByKey.get(filterKey) ?? [];
    for (const entryId of previousEntryIds) {
        const currentFilterKeys = filterKeysByEntryId.get(entryId);
        if (!currentFilterKeys) {
            continue;
        }

        currentFilterKeys.delete(filterKey);
        if (currentFilterKeys.size === 0) {
            filterKeysByEntryId.delete(entryId);
        }
    }

    if (naturallyOrderedEntryIds.length === 0) {
        state.filterEntryIdsByKey.delete(filterKey);
        return;
    }

    state.filterEntryIdsByKey.set(filterKey, naturallyOrderedEntryIds);

    for (const entryId of naturallyOrderedEntryIds) {
        const currentFilterKeys = filterKeysByEntryId.get(entryId) ?? new Set<string>();
        currentFilterKeys.add(filterKey);
        filterKeysByEntryId.set(entryId, currentFilterKeys);
    }
}

function applyEntryFilterMatches(
    entryId: string,
    matches: bridge.HistoryFilterMatch[] | null | undefined,
) {
    const nextFilterKeys = new Set(
        normalizeFilterMatches(matches).map((match) =>
            getHistoryFilterKey(match.filterId, match.version),
        ),
    );
    const previousFilterKeys = filterKeysByEntryId.get(entryId) ?? new Set<string>();

    for (const filterKey of previousFilterKeys) {
        if (!nextFilterKeys.has(filterKey)) {
            removeEntryFromFilterMembership(filterKey, entryId);
        }
    }

    for (const filterKey of nextFilterKeys) {
        if (!previousFilterKeys.has(filterKey)) {
            addEntryToFilterMembership(filterKey, entryId);
        }
    }

    if (nextFilterKeys.size === 0) {
        filterKeysByEntryId.delete(entryId);
        return;
    }

    filterKeysByEntryId.set(entryId, nextFilterKeys);
}

function applyEventFilterEvaluation(
    entryId: string,
    matches: bridge.HistoryFilterMatch[] | null | undefined,
    evaluatedFilters:
        | bridge.HistoryFilterMatch[]
        | null
        | undefined,
) {
    // Compatibility with summaries emitted by an older backend.
    if (!Array.isArray(evaluatedFilters)) {
        applyEntryFilterMatches(entryId, matches);
        return;
    }

    const matchingFilterKeys = new Set(
        normalizeFilterMatches(matches).map((match) =>
            getHistoryFilterKey(match.filterId, match.version),
        ),
    );
    const currentFilterKeys = new Set(
        filterKeysByEntryId.get(entryId) ?? [],
    );

    for (const evaluatedFilter of normalizeFilterMatches(
        evaluatedFilters,
    )) {
        const activeVersion = activeFilterVersionsById.get(
            evaluatedFilter.filterId,
        );

        // A delayed event from an older filter version is not authoritative
        // for the version currently displayed by the tab.
        if (
            activeVersion === undefined ||
            activeVersion !== evaluatedFilter.version
        ) {
            continue;
        }

        const filterKey = getHistoryFilterKey(
            evaluatedFilter.filterId,
            evaluatedFilter.version,
        );
        if (matchingFilterKeys.has(filterKey)) {
            addEntryToFilterMembership(filterKey, entryId);
            currentFilterKeys.add(filterKey);
        } else {
            removeEntryFromFilterMembership(filterKey, entryId);
            currentFilterKeys.delete(filterKey);
        }
    }

    if (currentFilterKeys.size === 0) {
        filterKeysByEntryId.delete(entryId);
    } else {
        filterKeysByEntryId.set(entryId, currentFilterKeys);
    }
}

function applyHistoryFilterMatchesBatch(
    results: bridge.HistoryEntryFilterMatches[],
    targetFilterId?: string,
    targetVersion?: number,
) {
    for (const result of results) {
        const entryId = String(result.entryId);
        if (!state.entriesById.has(entryId)) {
            continue;
        }

        if (!targetFilterId) {
            applyEntryFilterMatches(entryId, result.matches);
            continue;
        }

        const nextMatches = normalizeFilterMatches(result.matches).filter(
            (match) =>
                match.filterId === targetFilterId &&
                (targetVersion === undefined || match.version === targetVersion),
        );
        const targetFilterKey = getHistoryFilterKey(
            targetFilterId,
            targetVersion ?? 0,
        );
        const previousFilterKeys = filterKeysByEntryId.get(entryId) ?? new Set<string>();
        const hasTargetFilterKey = previousFilterKeys.has(targetFilterKey);

        if (nextMatches.length > 0 && !hasTargetFilterKey) {
            addEntryToFilterMembership(targetFilterKey, entryId);
            const nextFilterKeys = new Set(previousFilterKeys);
            nextFilterKeys.add(targetFilterKey);
            filterKeysByEntryId.set(entryId, nextFilterKeys);
            continue;
        }

        if (nextMatches.length === 0 && hasTargetFilterKey) {
            removeEntryFromFilterMembership(targetFilterKey, entryId);
            const nextFilterKeys = new Set(previousFilterKeys);
            nextFilterKeys.delete(targetFilterKey);
            if (nextFilterKeys.size === 0) {
                filterKeysByEntryId.delete(entryId);
            } else {
                filterKeysByEntryId.set(entryId, nextFilterKeys);
            }
        }
    }
}

function markEntryDirtyForPendingFilterSyncs(entryId: string) {
    for (const pendingEntryIds of pendingEntryIdsByFilterKey.values()) {
        pendingEntryIds.add(entryId);
    }
}

function createHistoryEntry(
    id: string,
    firstSeenAtMs: number,
    request: RequestSummary | null,
    response: ResponseSummary | null,
): HistoryEntry {
    sequenceCounter += 1;

    const requestArrivedAtMs = request?.receivedAtMs ?? null;
    const nextEntry: HistoryEntry = {
        id,
        request,
        response,
        firstSeenAtMs,
        requestArrivedAtMs,
        requestTimeLabel:
            requestArrivedAtMs === null
                ? "--:--:--"
                : formatLocalTime(requestArrivedAtMs),
        sequence: sequenceCounter,
        read: httpHistoryPanelActive,
    };

    state.entriesById.set(id, nextEntry);
    state.entries.push(nextEntry);
    if (!nextEntry.read) {
        state.unreadCount += 1;
    }
    if (!state.selectedId) {
        state.selectedId = id;
    }

    return nextEntry;
}

function moveEntryToNaturalOrderEnd(entryId: string) {
    const currentIndex = state.entries.findIndex((entry) => entry.id === entryId);
    if (currentIndex === -1 || currentIndex === state.entries.length - 1) {
        return;
    }

    const [entry] = state.entries.splice(currentIndex, 1);
    state.entries.push(entry);
}

function clearPendingFilterSyncEntryId(entryId: string) {
    for (const pendingEntryIds of pendingEntryIdsByFilterKey.values()) {
        pendingEntryIds.delete(entryId);
    }
}

function removeHistoryEntryFromState(entryId: string): boolean {
    const currentEntry = state.entriesById.get(entryId);
    if (!currentEntry) {
        return false;
    }

    const currentIndex = state.entries.findIndex((entry) => entry.id === entryId);
    const fallbackSelectedId =
        state.entries[currentIndex + 1]?.id ??
        state.entries[currentIndex - 1]?.id ??
        null;

    if (!currentEntry.read) {
        state.unreadCount = Math.max(0, state.unreadCount - 1);
    }

    if (currentIndex !== -1) {
        state.entries.splice(currentIndex, 1);
    }

    removeAllFilterMembershipsForEntryId(entryId);
    clearPendingFilterSyncEntryId(entryId);
    state.entriesById.delete(entryId);
    removeHistoryEntryDetail(entryId);

    if (state.selectedId === entryId) {
        state.selectedId = fallbackSelectedId;
    }

    return true;
}

function clearHistoryEntriesFromState() {
    state.entries.length = 0;
    state.entriesById.clear();
    state.filterEntryIdsByKey.clear();
    state.selectedId = null;
    state.unreadCount = 0;
    sequenceCounter = 0;
    filterKeysByEntryId.clear();
    clearHistoryDetailCache();

    for (const pendingEntryIds of pendingEntryIdsByFilterKey.values()) {
        pendingEntryIds.clear();
    }
}

function markEntryRead(entry: HistoryEntry): boolean {
    if (entry.read) {
        return false;
    }

    entry.read = true;
    state.unreadCount = Math.max(0, state.unreadCount - 1);
    return true;
}

function applyRequestSummary(payload: HistoryRequestSummaryPayload) {
    const request = normalizeRequestSummary(payload);
    const currentEntry = state.entriesById.get(request.id);

    if (!currentEntry) {
        createHistoryEntry(
            request.id,
            request.receivedAtMs,
            request,
            null,
        );
        applyEventFilterEvaluation(
            request.id,
            payload.filterMatches,
            payload.evaluatedFilters,
        );
        markEntryDirtyForPendingFilterSyncs(request.id);
        return;
    }

    currentEntry.request = request;
    if (currentEntry.requestArrivedAtMs === null) {
        currentEntry.requestArrivedAtMs = request.receivedAtMs;
        currentEntry.requestTimeLabel = formatLocalTime(request.receivedAtMs);
        moveEntryToNaturalOrderEnd(currentEntry.id);
    }
    if (httpHistoryPanelActive) {
        markEntryRead(currentEntry);
    }

    applyEventFilterEvaluation(
        request.id,
        payload.filterMatches,
        payload.evaluatedFilters,
    );
    invalidateHistoryEntryDetail(request.id);
    markEntryDirtyForPendingFilterSyncs(request.id);
}

function applyResponseSummary(payload: HistoryResponseSummaryPayload) {
    const response = normalizeResponseSummary(payload);
    const currentEntry = state.entriesById.get(response.id);

    if (!currentEntry) {
        createHistoryEntry(
            response.id,
            response.receivedAtMs,
            null,
            response,
        );
        applyEventFilterEvaluation(
            response.id,
            payload.filterMatches,
            payload.evaluatedFilters,
        );
        markEntryDirtyForPendingFilterSyncs(response.id);
        return;
    }

    currentEntry.response = response;
    if (httpHistoryPanelActive) {
        markEntryRead(currentEntry);
    }

    applyEventFilterEvaluation(
        response.id,
        payload.filterMatches,
        payload.evaluatedFilters,
    );
    invalidateHistoryEntryDetail(response.id);
    markEntryDirtyForPendingFilterSyncs(response.id);
}

export function ensureHttpHistoryCapture() {
    if (captureStarted) {
        return;
    }

    captureStarted = true;
    EventsOn("request", (payload: HistoryRequestSummaryPayload) => {
        applyRequestSummary(payload);
        publishHistoryState();
    });
    EventsOn("response", (payload: HistoryResponseSummaryPayload) => {
        applyResponseSummary(payload);
        publishHistoryState();
    });
}

export function selectHistoryEntry(id: string) {
    if (state.selectedId === id) {
        return;
    }

    state.selectedId = id;
    publishHistoryState();
}

export function markAllHistoryEntriesRead() {
    let changed = false;

    for (const entry of state.entries) {
        changed = markEntryRead(entry) || changed;
    }

    if (changed) {
        publishHistoryState();
    }
}

export function setHttpHistoryPanelActive(active: boolean) {
    if (httpHistoryPanelActive === active) {
        return;
    }

    httpHistoryPanelActive = active;
    if (active) {
        markAllHistoryEntriesRead();
    }
}

async function reconcilePendingEntryIdsForFilter(
    operation: FilterUpsertOperation,
    filterId: string,
    version: number,
) {
    while (
        isLatestFilterUpsert(operation) &&
        operation.pendingEntryIds.size > 0
    ) {
        const entryIdsSnapshot = Array.from(operation.pendingEntryIds);
        operation.pendingEntryIds.clear();

        const results = await GetHistoryFilterMatchesForEntries(
            bridge.GetHistoryFilterMatchesForEntriesParams.createFrom({
                entryIds: entryIdsSnapshot.map(toBackendHistoryId),
            }),
        );

        if (!isLatestFilterUpsert(operation)) {
            return;
        }

        applyHistoryFilterMatchesBatch(results, filterId, version);
        publishHistoryState();
    }
}

export async function upsertHistoryFilterTab(
    tab: HistoryFilterTabConfig,
): Promise<{ filterId: string; version: number; matchingIds: string[] }> {
    const requestedFilterKey = getHistoryFilterKey(tab.id, tab.filterVersion);
    const previousActiveVersion = activeFilterVersionsById.get(tab.id);
    const operation: FilterUpsertOperation = {
        filterId: tab.id,
        filterKey: requestedFilterKey,
        mutationToken: Symbol(`upsert:${requestedFilterKey}`),
        pendingEntryIds: new Set<string>(),
    };

    supersedePendingFilterUpsert(tab.id);
    pendingFilterUpsertsById.set(tab.id, operation);
    latestFilterMutationTokensById.set(tab.id, operation.mutationToken);
    activeFilterVersionsById.set(tab.id, tab.filterVersion);
    pendingEntryIdsByFilterKey.set(
        requestedFilterKey,
        operation.pendingEntryIds,
    );
    setFilterSyncing(requestedFilterKey, true);
    publishHistoryState();

    try {
        const result = await UpsertActiveHistoryFilter(
            bridge.UpsertActiveHistoryFilterParams.createFrom({
                filterId: tab.id,
                version: tab.filterVersion,
                conditions: tab.conditions,
                operator: tab.operator,
            }),
        );
        const normalizedResult = {
            filterId: result.filterId,
            version: result.version,
            matchingIds: result.matchingIds.map((id) => String(id)),
        };

        if (!isLatestFilterUpsert(operation)) {
            return normalizedResult;
        }

        const resolvedFilterKey = getHistoryFilterKey(
            result.filterId,
            result.version,
        );
        if (
            result.filterId !== tab.id &&
            activeFilterVersionsById.get(tab.id) === tab.filterVersion
        ) {
            activeFilterVersionsById.delete(tab.id);
        }
        activeFilterVersionsById.set(result.filterId, result.version);

        if (resolvedFilterKey !== requestedFilterKey) {
            moveFilterUpsertSync(operation, resolvedFilterKey);
        }

        removeAllFilterMembershipsForFilterId(result.filterId, resolvedFilterKey);
        replaceFilterMembership(
            resolvedFilterKey,
            normalizedResult.matchingIds,
        );
        publishHistoryState();

        await reconcilePendingEntryIdsForFilter(
            operation,
            result.filterId,
            result.version,
        );

        return normalizedResult;
    } catch (error) {
        if (
            isLatestFilterUpsert(operation) &&
            activeFilterVersionsById.get(tab.id) === tab.filterVersion
        ) {
            if (previousActiveVersion === undefined) {
                activeFilterVersionsById.delete(tab.id);
            } else {
                activeFilterVersionsById.set(tab.id, previousActiveVersion);
            }
        }
        throw error;
    } finally {
        const wasLatestOperation = isLatestFilterUpsert(operation);
        const releasedSyncState = releaseFilterUpsertSync(operation);

        if (pendingFilterUpsertsById.get(tab.id) === operation) {
            pendingFilterUpsertsById.delete(tab.id);
        }
        if (wasLatestOperation) {
            latestFilterMutationTokensById.delete(tab.id);
        }
        if (wasLatestOperation || releasedSyncState) {
            publishHistoryState();
        }
    }
}

export async function removeHistoryFilterTab(
    filterId: string,
    version: number,
) {
    const filterKey = getHistoryFilterKey(filterId, version);
    const mutationToken = Symbol(`remove:${filterKey}`);

    supersedePendingFilterUpsert(filterId);
    latestFilterMutationTokensById.set(filterId, mutationToken);

    try {
        await RemoveActiveHistoryFilter(
            bridge.RemoveActiveHistoryFilterParams.createFrom({
                filterId,
                version,
            }),
        );

        if (latestFilterMutationTokensById.get(filterId) !== mutationToken) {
            return;
        }

        if (activeFilterVersionsById.get(filterId) === version) {
            activeFilterVersionsById.delete(filterId);
        }
        pendingEntryIdsByFilterKey.delete(filterKey);
        setFilterSyncing(filterKey, false);
        removeAllFilterMembershipsForFilterId(filterId);
        publishHistoryState();
    } finally {
        if (latestFilterMutationTokensById.get(filterId) === mutationToken) {
            latestFilterMutationTokensById.delete(filterId);
        }
    }
}

export async function syncPersistedHistoryFilters(
    tabs: HistoryFilterTabConfig[],
) {
    for (const tab of tabs) {
        await upsertHistoryFilterTab(tab);
    }
}

export async function removeHistoryEntryById(entryId: string) {
    await RemoveHistoryEntry(toBackendHistoryId(entryId));

    if (removeHistoryEntryFromState(entryId)) {
        publishHistoryState();
    }
}

export async function clearAllHistoryEntries() {
    await ClearHistoryEntries();
    clearHistoryEntriesFromState();
    publishHistoryState();
}

export function clearFrontendHistoryState() {
    clearHistoryEntriesFromState();
    state.syncingFilterKeys.clear();
    pendingEntryIdsByFilterKey.clear();
    publishHistoryState();
}

export type { RequestView, ResponseView };

export const httpHistoryStore = {
    subscribe,
};

function toBackendHistoryId(id: string): number {
    const parsedId = Number(id);
    if (!Number.isFinite(parsedId)) {
        throw new Error(`Could not convert ID "${id}" for the backend.`);
    }

    return parsedId;
}
