import { writable } from "svelte/store";
import type {
    HistoryEntry,
    RequestView,
} from "@/features/http-history/state/httpHistoryStore";
import {
    sendRepeaterRequest,
    type RepeaterSendResult,
} from "../utils/repeaterBridge";
import {
    buildRepeaterTabLabel,
    createEmptyRepeaterRequestDraft,
    prepareRepeaterRequestForSend,
    seedRepeaterRequestDraft,
    seedRepeaterRequestDraftFromRequest,
    validateRepeaterRequest,
    type RepeaterRequestDraft,
    type RepeaterValidationResult,
} from "../utils/repeaterRequest";

export type RepeaterTabResponse = {
    headBlockStr: string;
    bodyStr: string;
    host: string;
    port: string;
    version: string;
    statusCode: number | null;
    durationMs: number | null;
    unsupportedContentEncodings: string[];
    contentDecodingFailed: boolean;
};

export type RepeaterTab = {
    id: string;
    sequenceNumber: number;
    request: RepeaterRequestDraft;
    validation: RepeaterValidationResult;
    response: RepeaterTabResponse | null;
    requestState: "idle" | "sending" | "success" | "error";
    requestError: string | null;
    sourceEntryId: string | null;
};

type RepeaterState = {
    tabs: RepeaterTab[];
    activeTabId: string | null;
    nextTabNumber: number;
};

const initialState: RepeaterState = {
    tabs: [],
    activeTabId: null,
    nextTabNumber: 1,
};

const { subscribe, update } = writable<RepeaterState>(initialState);

function parseResponseStartLine(
    headBlockStr: string,
): Pick<RepeaterTabResponse, "version" | "statusCode"> {
    const firstLine = headBlockStr.split(/\r?\n/, 1)[0] ?? "";
    const match = firstLine.match(/^(HTTP\/[0-9.]+)\s+(\d{3})/);

    return {
        version: match?.[1] ?? "",
        statusCode: match ? Number.parseInt(match[2], 10) : null,
    };
}

function getStringValue(
    value: unknown,
    fallback = "",
): string {
    return typeof value === "string" ? value : fallback;
}

function getNumberValue(
    value: unknown,
    fallback: number | null = null,
): number | null {
    return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function normalizeStringArray(value: unknown): string[] {
    if (!Array.isArray(value)) return [];

    const normalized = value
        .filter((item): item is string => typeof item === "string")
        .map((item) => item.trim().toLowerCase())
        .filter(Boolean);

    return [...new Set(normalized)];
}

function buildRepeaterResponse(result: RepeaterSendResult): RepeaterTabResponse {
    const headBlockStr = getStringValue(result.headBlockStr);
    const bodyStr = getStringValue(result.bodyStr);
    const parsedStartLine = parseResponseStartLine(headBlockStr);

    return {
        headBlockStr,
        bodyStr,
        host: getStringValue(result.host),
        port: getStringValue(result.port),
        version: getStringValue(result.version, parsedStartLine.version),
        statusCode: getNumberValue(result.statusCode, parsedStartLine.statusCode),
        durationMs: getNumberValue(result.durationMs),
        unsupportedContentEncodings: normalizeStringArray(
            result.unsupportedContentEncodings,
        ),
        contentDecodingFailed: result.contentDecodingFailed ?? false,
    };
}

function hydrateTab(
    partialTab: Omit<RepeaterTab, "validation">,
): RepeaterTab {
    return {
        ...partialTab,
        validation: validateRepeaterRequest(partialTab.request),
    };
}

function buildTab(
    sequenceNumber: number,
    request: RepeaterRequestDraft,
    sourceEntryId: string | null,
): RepeaterTab {
    return hydrateTab({
        id: `repeater-tab-${sequenceNumber}`,
        sequenceNumber,
        request,
        response: null,
        requestState: "idle",
        requestError: null,
        sourceEntryId,
    });
}

function updateTab(
    state: RepeaterState,
    tabId: string,
    updater: (tab: RepeaterTab) => RepeaterTab,
): RepeaterState {
    return {
        ...state,
        tabs: state.tabs.map((tab) => (tab.id === tabId ? updater(tab) : tab)),
    };
}

export function getRepeaterTabTitle(tab: RepeaterTab): string {
    return buildRepeaterTabLabel(
        tab.sequenceNumber,
        tab.validation.parsedRequest,
    );
}

export function createRepeaterTab() {
    update((state) => {
        const tab = buildTab(
            state.nextTabNumber,
            createEmptyRepeaterRequestDraft(),
            null,
        );

        return {
            tabs: [...state.tabs, tab],
            activeTabId: tab.id,
            nextTabNumber: state.nextTabNumber + 1,
        };
    });
}

export function openRepeaterTabFromHistoryEntry(entry: HistoryEntry) {
    const request = seedRepeaterRequestDraft(entry);
    if (!request) return;

    openRepeaterTabFromRequest(request, entry.id);
}

export function openRepeaterTabFromRequestSnapshot(
    request: RequestView,
    sourceEntryId: string | null = null,
) {
    openRepeaterTabFromRequest(
        seedRepeaterRequestDraftFromRequest(request),
        sourceEntryId,
    );
}

function openRepeaterTabFromRequest(
    request: RepeaterRequestDraft,
    sourceEntryId: string | null,
) {
    update((state) => {
        const tab = buildTab(state.nextTabNumber, request, sourceEntryId);

        return {
            tabs: [...state.tabs, tab],
            activeTabId: tab.id,
            nextTabNumber: state.nextTabNumber + 1,
        };
    });
}

export function duplicateRepeaterTab(tabId: string) {
    update((state) => {
        const sourceTab = state.tabs.find((tab) => tab.id === tabId);
        if (!sourceTab) return state;

        const duplicatedTab = buildTab(
            state.nextTabNumber,
            {
                ...sourceTab.request,
            },
            sourceTab.sourceEntryId,
        );

        return {
            tabs: [...state.tabs, duplicatedTab],
            activeTabId: duplicatedTab.id,
            nextTabNumber: state.nextTabNumber + 1,
        };
    });
}

export function closeRepeaterTab(tabId: string) {
    update((state) => {
        const closingIndex = state.tabs.findIndex((tab) => tab.id === tabId);
        if (closingIndex === -1) return state;

        const nextTabs = state.tabs.filter((tab) => tab.id !== tabId);
        const fallbackTabId =
            nextTabs[Math.max(0, closingIndex - 1)]?.id ?? nextTabs[0]?.id ?? null;

        return {
            ...state,
            tabs: nextTabs,
            activeTabId: state.activeTabId === tabId ? fallbackTabId : state.activeTabId,
        };
    });
}

export function setActiveRepeaterTab(tabId: string) {
    update((state) => ({
        ...state,
        activeTabId: tabId,
    }));
}

export function updateRepeaterTabRequest(
    tabId: string,
    patch: Partial<RepeaterRequestDraft>,
) {
    update((state) =>
        updateTab(state, tabId, (tab) =>
            hydrateTab({
                ...tab,
                request: {
                    ...tab.request,
                    ...patch,
                },
                requestState: tab.requestState === "sending" ? "sending" : "idle",
                requestError: null,
            }),
        ),
    );
}

export async function sendRepeaterTab(tabId: string) {
    let currentTab: RepeaterTab | null = null;

    update((state) =>
        updateTab(state, tabId, (tab) => {
            currentTab = hydrateTab({
                ...tab,
                requestState: "sending",
                requestError: null,
            });

            return currentTab;
        }),
    );

    if (!currentTab) return;

    if (currentTab.validation.errors.length > 0) {
        update((state) =>
            updateTab(state, tabId, (tab) => ({
                ...tab,
                requestState: "error",
                requestError:
                    "Fix the validation errors before sending the request.",
            })),
        );
        return;
    }

    if (!currentTab.validation.parsedRequest) {
        update((state) =>
            updateTab(state, tabId, (tab) => ({
                ...tab,
                requestState: "error",
                requestError:
                    "Could not parse the request line before sending the request.",
            })),
        );
        return;
    }

    try {
        const preparedRequest = prepareRepeaterRequestForSend(
            currentTab.request,
            currentTab.validation.parsedRequest,
        );

        const result = await sendRepeaterRequest({
            scheme: currentTab.request.scheme,
            host: preparedRequest.host,
            port: preparedRequest.port,
            method: preparedRequest.method,
            path: preparedRequest.path,
            headBlockStr: preparedRequest.headBlockStr,
            bodyStr: currentTab.request.bodyStr,
            skipServerCertVerify: currentTab.request.skipServerCertVerify,
            version: preparedRequest.version,
            pseudoHeaders: preparedRequest.pseudoHeaders,
            headers: preparedRequest.headers,
        });

        update((state) =>
            updateTab(state, tabId, (tab) => ({
                ...tab,
                requestState: "success",
                requestError: null,
                response: buildRepeaterResponse(result),
            })),
        );
    } catch (error: unknown) {
        update((state) =>
            updateTab(state, tabId, (tab) => ({
                ...tab,
                requestState: "error",
                requestError:
                    error instanceof Error ? error.message : String(error),
            })),
        );
    }
}

export const repeaterStore = {
    subscribe,
};
