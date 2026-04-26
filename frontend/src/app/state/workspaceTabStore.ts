import { writable } from "svelte/store";

export type WorkspaceTabId =
    | "configureProxy"
    | "httpHistory"
    | "savedRequests"
    | "interception"
    | "repeater"
    | "errorLog";

const { subscribe, set } = writable<WorkspaceTabId>("configureProxy");

export function setActiveWorkspaceTab(tabId: WorkspaceTabId) {
    set(tabId);
}

export const workspaceTabStore = {
    subscribe,
};
