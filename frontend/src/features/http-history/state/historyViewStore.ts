import { writable } from "svelte/store";
import type {
    SortDirection,
    SortKey,
} from "./httpHistoryStore";
import type {
    HistoryFilterCondition,
    HistoryFilterMatchMode,
    HistoryFilterOperator,
    HistoryFilterTarget,
} from "../utils/historyFilters";
import {
    HISTORY_FILTER_MATCH_MODE_VALUES,
    HISTORY_FILTER_TARGET_VALUES,
} from "../utils/historyFilters";

export const HISTORY_MAIN_VIEW_TAB_ID = "history-main";
export const DEFAULT_HISTORY_SORT_KEY: SortKey = "time";
export const DEFAULT_HISTORY_SORT_DIRECTION: SortDirection = "asc";
const allowedSortKeys: SortKey[] = [
    "time",
    "host",
    "path",
    "port",
    "method",
    "version",
    "statusCode",
];

export type HistoryViewTab = {
    id: string;
    label: string;
    kind: "main" | "filter";
    filterVersion: number;
    conditions: HistoryFilterCondition[];
    operator: HistoryFilterOperator;
    sortKey: SortKey;
    sortDirection: SortDirection;
};

type HistoryViewState = {
    activeTabId: string;
    mainSortKey: SortKey;
    mainSortDirection: SortDirection;
    filterTabs: HistoryViewTab[];
    nextFilterTabNumber: number;
};

const STORAGE_KEY = "mitmgo.http-history.views";

const initialState: HistoryViewState = {
    activeTabId: HISTORY_MAIN_VIEW_TAB_ID,
    mainSortKey: DEFAULT_HISTORY_SORT_KEY,
    mainSortDirection: DEFAULT_HISTORY_SORT_DIRECTION,
    filterTabs: [],
    nextFilterTabNumber: 1,
};
const allowedMatchModes: HistoryFilterMatchMode[] = HISTORY_FILTER_MATCH_MODE_VALUES;
const allowedTargets: HistoryFilterTarget[] = HISTORY_FILTER_TARGET_VALUES;

function isBrowser(): boolean {
    return typeof window !== "undefined" && typeof localStorage !== "undefined";
}

function sanitizeHistoryViewState(value: unknown): HistoryViewState {
    if (!value || typeof value !== "object") {
        return initialState;
    }

    const candidate = value as Partial<HistoryViewState>;
    const mainSortKey = allowedSortKeys.includes(candidate.mainSortKey as SortKey)
        ? (candidate.mainSortKey as SortKey)
        : DEFAULT_HISTORY_SORT_KEY;
    const mainSortDirection =
        candidate.mainSortDirection === "desc" ? "desc" : "asc";
    const filterTabs = Array.isArray(candidate.filterTabs)
        ? candidate.filterTabs.flatMap((tab) => {
            if (
                !tab ||
                typeof tab !== "object" ||
                typeof tab.id !== "string" ||
                typeof tab.label !== "string" ||
                tab.kind !== "filter" ||
                !Array.isArray(tab.conditions) ||
                (tab.operator !== "and" && tab.operator !== "or")
            ) {
                return [];
            }

            const conditions = tab.conditions.flatMap((condition) => {
                if (
                    !condition ||
                    typeof condition !== "object" ||
                    typeof condition.id !== "string" ||
                    typeof condition.query !== "string" ||
                    typeof condition.target !== "string"
                ) {
                    return [];
                }

                const matchMode = allowedMatchModes.includes(
                    condition.matchMode as HistoryFilterMatchMode,
                )
                    ? (condition.matchMode as HistoryFilterMatchMode)
                    : "contains";
                const target = allowedTargets.includes(
                    condition.target as HistoryFilterTarget,
                )
                    ? (condition.target as HistoryFilterTarget)
                    : "all";

                return [
                    {
                        id: condition.id,
                        query: condition.query,
                        target,
                        matchMode,
                    } satisfies HistoryFilterCondition,
                ];
            });

            return [
                {
                    id: tab.id,
                    label: tab.label,
                    kind: "filter",
                    filterVersion:
                        typeof tab.filterVersion === "number" &&
                            Number.isFinite(tab.filterVersion) &&
                            tab.filterVersion >= 1
                            ? Math.floor(tab.filterVersion)
                            : 1,
                    conditions,
                    operator: tab.operator,
                    sortKey: allowedSortKeys.includes(tab.sortKey as SortKey)
                        ? (tab.sortKey as SortKey)
                        : DEFAULT_HISTORY_SORT_KEY,
                    sortDirection:
                        tab.sortDirection === "desc" ? "desc" : "asc",
                } satisfies HistoryViewTab,
            ];
        })
        : [];
    const requestedActiveTabId =
        typeof candidate.activeTabId === "string"
            ? candidate.activeTabId
            : HISTORY_MAIN_VIEW_TAB_ID;
    const nextFilterTabNumber =
        typeof candidate.nextFilterTabNumber === "number" &&
            Number.isFinite(candidate.nextFilterTabNumber) &&
            candidate.nextFilterTabNumber >= 1
            ? Math.floor(candidate.nextFilterTabNumber)
            : filterTabs.length + 1;

    const activeTabId =
        requestedActiveTabId === HISTORY_MAIN_VIEW_TAB_ID ||
            filterTabs.some((tab) => tab.id === requestedActiveTabId)
            ? requestedActiveTabId
            : HISTORY_MAIN_VIEW_TAB_ID;

    return {
        activeTabId,
        mainSortKey,
        mainSortDirection,
        filterTabs,
        nextFilterTabNumber,
    };
}

function loadHistoryViewState(): HistoryViewState {
    if (!isBrowser()) {
        return initialState;
    }

    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (!raw) {
            return initialState;
        }

        return sanitizeHistoryViewState(JSON.parse(raw));
    } catch {
        return initialState;
    }
}

const { subscribe, update } = writable<HistoryViewState>(loadHistoryViewState());

if (isBrowser()) {
    subscribe((state) => {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    });
}

export function activateHistoryViewTab(tabId: string) {
    update((state) => ({
        ...state,
        activeTabId: tabId,
    }));
}

export function createHistoryViewTab(
    conditions: HistoryFilterCondition[],
    operator: HistoryFilterOperator,
): HistoryViewTab {
    let createdTab: HistoryViewTab = {
        id: "",
        label: "",
        kind: "filter",
        filterVersion: 1,
        conditions: [],
        operator: "and",
        sortKey: DEFAULT_HISTORY_SORT_KEY,
        sortDirection: DEFAULT_HISTORY_SORT_DIRECTION,
    };

    update((state) => {
        const tabNumber = state.nextFilterTabNumber;
        const nextTab: HistoryViewTab = {
            id: `history-filter-${tabNumber}`,
            label: `Filter ${tabNumber}`,
            kind: "filter",
            filterVersion: 1,
            conditions,
            operator,
            sortKey: DEFAULT_HISTORY_SORT_KEY,
            sortDirection: DEFAULT_HISTORY_SORT_DIRECTION,
        };
        createdTab = nextTab;

        return {
            ...state,
            activeTabId: nextTab.id,
            filterTabs: [...state.filterTabs, nextTab],
            nextFilterTabNumber: tabNumber + 1,
        };
    });

    return createdTab;
}

export function updateHistoryViewTab(
    tabId: string,
    conditions: HistoryFilterCondition[],
    operator: HistoryFilterOperator,
): HistoryViewTab | null {
    let updatedTab: HistoryViewTab | null = null;

    update((state) => {
        const nextFilterTabs = state.filterTabs.map((tab) => {
            if (tab.id !== tabId) {
                return tab;
            }

            updatedTab = {
                ...tab,
                filterVersion: tab.filterVersion + 1,
                conditions,
                operator,
            };

            return updatedTab;
        });

        return {
            ...state,
            filterTabs: nextFilterTabs,
        };
    });

    return updatedTab;
}

export function toggleHistoryViewTabSort(tabId: string, key: SortKey) {
    update((state) => {
        if (tabId === HISTORY_MAIN_VIEW_TAB_ID) {
            if (state.mainSortKey === key) {
                return {
                    ...state,
                    mainSortDirection:
                        state.mainSortDirection === "asc" ? "desc" : "asc",
                };
            }

            return {
                ...state,
                mainSortKey: key,
                mainSortDirection: "asc",
            };
        }

        return {
            ...state,
            filterTabs: state.filterTabs.map((tab) => {
                if (tab.id !== tabId) return tab;

                if (tab.sortKey === key) {
                    return {
                        ...tab,
                        sortDirection:
                            tab.sortDirection === "asc" ? "desc" : "asc",
                    };
                }

                return {
                    ...tab,
                    sortKey: key,
                    sortDirection: "asc",
                };
            }),
        };
    });
}

export function closeHistoryViewTab(tabId: string) {
    if (tabId === HISTORY_MAIN_VIEW_TAB_ID) {
        return;
    }

    update((state) => {
        const closingIndex = state.filterTabs.findIndex((tab) => tab.id === tabId);
        if (closingIndex === -1) {
            return state;
        }

        const remainingTabs = state.filterTabs.filter((tab) => tab.id !== tabId);
        const fallbackTabId =
            remainingTabs[Math.max(0, closingIndex - 1)]?.id ??
            HISTORY_MAIN_VIEW_TAB_ID;

        return {
            ...state,
            activeTabId:
                state.activeTabId === tabId ? fallbackTabId : state.activeTabId,
            filterTabs: remainingTabs,
        };
    });
}

export const historyViewStore = {
    subscribe,
};
