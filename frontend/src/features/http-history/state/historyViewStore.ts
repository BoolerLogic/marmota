import { writable } from "svelte/store";
import type {
    SortDirection,
    SortKey,
} from "./httpHistoryStore";
import type {
    HistoryFilterCondition,
    HistoryFilterOperator,
} from "../utils/historyFilters";

export const HISTORY_MAIN_VIEW_TAB_ID = "history-main";
export const DEFAULT_HISTORY_SORT_KEY: SortKey = "time";
export const DEFAULT_HISTORY_SORT_DIRECTION: SortDirection = "asc";
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

const initialState: HistoryViewState = {
    activeTabId: HISTORY_MAIN_VIEW_TAB_ID,
    mainSortKey: DEFAULT_HISTORY_SORT_KEY,
    mainSortDirection: DEFAULT_HISTORY_SORT_DIRECTION,
    filterTabs: [],
    nextFilterTabNumber: 1,
};

const { subscribe, update } = writable<HistoryViewState>(initialState);

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
