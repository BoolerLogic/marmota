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
};

type HistoryResponseSummaryPayload = {
    id: number | string;
    host?: string;
    port?: string;
    version?: string;
    statusCode?: number;
    receivedAtMs?: number;
    filterMatches?: bridge.HistoryFilterMatch[];
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
    };
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
    invalidateHistoryEntryDetail(entryId);

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
        applyEntryFilterMatches(request.id, payload.filterMatches);
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

    applyEntryFilterMatches(request.id, payload.filterMatches);
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
        applyEntryFilterMatches(response.id, payload.filterMatches);
        markEntryDirtyForPendingFilterSyncs(response.id);
        return;
    }

    currentEntry.response = response;
    if (httpHistoryPanelActive) {
        markEntryRead(currentEntry);
    }

    applyEntryFilterMatches(response.id, payload.filterMatches);
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
    filterId: string,
    version: number,
) {
    const filterKey = getHistoryFilterKey(filterId, version);
    const pendingEntryIds = pendingEntryIdsByFilterKey.get(filterKey);
    if (!pendingEntryIds) {
        return;
    }

    while (pendingEntryIds.size > 0) {
        const entryIdsSnapshot = Array.from(pendingEntryIds);
        pendingEntryIds.clear();

        const results = await GetHistoryFilterMatchesForEntries(
            bridge.GetHistoryFilterMatchesForEntriesParams.createFrom({
                entryIds: entryIdsSnapshot.map(toBackendHistoryId),
            }),
        );

        applyHistoryFilterMatchesBatch(results, filterId, version);
        publishHistoryState();
    }
}

export async function upsertHistoryFilterTab(
    tab: HistoryFilterTabConfig,
): Promise<{ filterId: string; version: number; matchingIds: string[] }> {
    const requestedFilterKey = getHistoryFilterKey(tab.id, tab.filterVersion);
    const pendingEntryIds = new Set<string>();

    pendingEntryIdsByFilterKey.set(requestedFilterKey, pendingEntryIds);
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
        const resolvedFilterKey = getHistoryFilterKey(result.filterId, result.version);

        if (resolvedFilterKey !== requestedFilterKey) {
            pendingEntryIdsByFilterKey.delete(requestedFilterKey);
            pendingEntryIdsByFilterKey.set(resolvedFilterKey, pendingEntryIds);
            setFilterSyncing(requestedFilterKey, false);
        }

        removeAllFilterMembershipsForFilterId(result.filterId, resolvedFilterKey);
        replaceFilterMembership(
            resolvedFilterKey,
            result.matchingIds.map((id) => String(id)),
        );
        publishHistoryState();

        await reconcilePendingEntryIdsForFilter(result.filterId, result.version);

        pendingEntryIdsByFilterKey.delete(resolvedFilterKey);
        setFilterSyncing(resolvedFilterKey, false);
        publishHistoryState();

        return {
            filterId: result.filterId,
            version: result.version,
            matchingIds: result.matchingIds.map((id) => String(id)),
        };
    } catch (error) {
        pendingEntryIdsByFilterKey.delete(requestedFilterKey);
        setFilterSyncing(requestedFilterKey, false);
        publishHistoryState();
        throw error;
    }
}

export async function removeHistoryFilterTab(
    filterId: string,
    version: number,
) {
    const filterKey = getHistoryFilterKey(filterId, version);

    await RemoveActiveHistoryFilter(
        bridge.RemoveActiveHistoryFilterParams.createFrom({
            filterId,
            version,
        }),
    );

    pendingEntryIdsByFilterKey.delete(filterKey);
    setFilterSyncing(filterKey, false);
    removeAllFilterMembershipsForFilterId(filterId);
    publishHistoryState();
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
