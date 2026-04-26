import { get, writable } from "svelte/store";
import type {
    HistoryEntryDetail,
    RequestView,
    ResponseView,
} from "@/features/http-history/state/historyDetailCache";

export type SavedRequestEntry = {
    id: string;
    name: string;
    request: RequestView;
    response: ResponseView | null;
    savedAtMs: number;
    savedTimeLabel: string;
    sourceHistoryEntryId: string | null;
    sequence: number;
};

type SavedRequestsState = {
    entries: SavedRequestEntry[];
    selectedId: string | null;
};

const timeFormatter = new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
});

const initialState: SavedRequestsState = {
    entries: [],
    selectedId: null,
};

const { subscribe, update } = writable<SavedRequestsState>(initialState);

let savedRequestSequence = 0;

function formatTime(timestampMs: number): string {
    return timeFormatter.format(new Date(timestampMs));
}

function cloneRequest(request: RequestView): RequestView {
    return {
        ...request,
    };
}

function cloneResponse(response: ResponseView | null): ResponseView | null {
    if (!response) return null;

    return {
        ...response,
    };
}

export function saveHistoryEntrySnapshot(
    entry: HistoryEntryDetail,
    name: string,
): boolean {
    if (!entry.request) return false;
    if (hasSavedRequestForHistoryEntry(entry.id)) return false;

    const savedAtMs = Date.now();
    savedRequestSequence += 1;

    const snapshot: SavedRequestEntry = {
        id: `saved-request-${savedRequestSequence}`,
        name: name.trim(),
        request: cloneRequest(entry.request),
        response: cloneResponse(entry.response),
        savedAtMs,
        savedTimeLabel: formatTime(savedAtMs),
        sourceHistoryEntryId: entry.id,
        sequence: savedRequestSequence,
    };

    update((state) => ({
        ...state,
        entries: [...state.entries, snapshot],
        selectedId: snapshot.id,
    }));

    return true;
}

export function hasSavedRequestForHistoryEntry(entryId: string): boolean {
    return get({ subscribe }).entries.some(
        (entry) => entry.sourceHistoryEntryId === entryId,
    );
}

export function selectSavedRequest(id: string) {
    update((state) => ({
        ...state,
        selectedId: id,
    }));
}

export function removeSavedRequest(id: string) {
    update((state) => {
        const index = state.entries.findIndex((entry) => entry.id === id);
        if (index === -1) return state;

        const nextEntries = [
            ...state.entries.slice(0, index),
            ...state.entries.slice(index + 1),
        ];

        let nextSelectedId = state.selectedId;
        if (state.selectedId === id) {
            nextSelectedId =
                nextEntries[index]?.id ??
                nextEntries[index - 1]?.id ??
                nextEntries[0]?.id ??
                null;
        }

        return {
            ...state,
            entries: nextEntries,
            selectedId: nextSelectedId,
        };
    });
}

export function clearSavedRequests() {
    update(() => ({
        entries: [],
        selectedId: null,
    }));
}

export const savedRequestsStore = {
    subscribe,
};
