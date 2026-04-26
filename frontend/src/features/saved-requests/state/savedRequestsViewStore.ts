import { writable } from "svelte/store";
import type {
    SortDirection,
    SortKey,
} from "@/features/http-history/state/httpHistoryStore";

export type SavedRequestsSortKey = "name" | SortKey;

type SavedRequestsViewState = {
    sortKey: SavedRequestsSortKey;
    sortDirection: SortDirection;
};

const initialState: SavedRequestsViewState = {
    sortKey: "time",
    sortDirection: "asc",
};

const { subscribe, update } = writable<SavedRequestsViewState>(initialState);

export function toggleSavedRequestsSort(key: SavedRequestsSortKey) {
    update((state) => {
        if (state.sortKey === key) {
            return {
                ...state,
                sortDirection: state.sortDirection === "asc" ? "desc" : "asc",
            };
        }

        return {
            ...state,
            sortKey: key,
            sortDirection: "asc",
        };
    });
}

export const savedRequestsViewStore = {
    subscribe,
};
