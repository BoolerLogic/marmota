import { writable } from "svelte/store";
import { EventsOn } from "../../../../wailsjs/runtime/runtime.js";

export type ErrorEntry = {
    id: string;
    message: string;
    createdAtMs: number;
    timeLabel: string;
    read: boolean;
};

type ErrorState = {
    entries: ErrorEntry[];
};

const timeFormatter = new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
});

const initialState: ErrorState = {
    entries: [],
};

const { subscribe, update } = writable<ErrorState>(initialState);

let captureStarted = false;
let errorSequence = 0;
let errorPanelActive = false;

function formatTime(timestampMs: number): string {
    return timeFormatter.format(new Date(timestampMs));
}

function appendError(message: string) {
    const createdAtMs = Date.now();
    errorSequence += 1;

    update((state) => ({
        ...state,
        entries: [
            {
                id: `error-${errorSequence}`,
                message,
                createdAtMs,
                timeLabel: formatTime(createdAtMs),
                read: errorPanelActive,
            },
            ...state.entries,
        ],
    }));
}

export function ensureErrorCapture() {
    if (captureStarted) return;

    captureStarted = true;
    EventsOn("error", (message: string) => {
        appendError(String(message ?? ""));
    });
}

export function markAllErrorsRead() {
    update((state) => ({
        ...state,
        entries: state.entries.map((entry) => ({
            ...entry,
            read: true,
        })),
    }));
}

export function setErrorPanelActive(active: boolean) {
    if (errorPanelActive === active) return;

    errorPanelActive = active;

    if (active) {
        markAllErrorsRead();
    }
}

export function clearErrors() {
    update(() => ({
        entries: [],
    }));
}

export const errorStore = {
    subscribe,
};
