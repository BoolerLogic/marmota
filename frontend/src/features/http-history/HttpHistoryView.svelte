<script lang="ts">
    import { onDestroy, onMount, tick } from "svelte";
    import { setActiveWorkspaceTab } from "@/app/state/workspaceTabStore";
    import HistoryFilterDialog from "./components/HistoryFilterDialog.svelte";
    import SaveRequestDialog from "@/features/saved-requests/components/SaveRequestDialog.svelte";
    import AppAlertDialog from "@/shared/components/AppAlertDialog.svelte";
    import RequestCopySubmenu from "@/shared/components/RequestCopySubmenu.svelte";
    import HistoryMessageBlock from "./components/HistoryMessageBlock.svelte";
    import RequestPathSummary from "./components/RequestPathSummary.svelte";
    import HistorySearchForm from "./components/HistorySearchForm.svelte";
    import { openRepeaterTabFromRequestSnapshot } from "@/features/repeater/state/repeaterStore";
    import {
        hasSavedRequestForHistoryEntry,
        saveHistoryEntrySnapshot,
    } from "@/features/saved-requests/state/savedRequestsStore";
    import {
        clearAllHistoryEntries,
        getHistoryFilterKey,
        httpHistoryStore,
        removeHistoryFilterTab,
        removeHistoryEntryById,
        selectHistoryEntry,
        syncPersistedHistoryFilters,
        type HistoryEntry,
        type SortDirection,
        type SortKey,
        upsertHistoryFilterTab,
    } from "./state/httpHistoryStore";
    import {
        getCachedHistoryEntryDetail,
        getHistoryEntryDetailCached,
        type HistoryEntryDetail,
    } from "./state/historyDetailCache";
    import {
        DEFAULT_HISTORY_SORT_DIRECTION,
        DEFAULT_HISTORY_SORT_KEY,
        activateHistoryViewTab,
        closeHistoryViewTab,
        createHistoryViewTab,
        HISTORY_MAIN_VIEW_TAB_ID,
        historyViewStore,
        toggleHistoryViewTabSort,
        updateHistoryViewTab,
        type HistoryViewTab,
    } from "./state/historyViewStore";
    import {
        createHistoryFilterCondition,
        getHistoryFilterSummary,
        type HistoryFilterCondition,
        type HistoryFilterOperator,
    } from "./utils/historyFilters";
    import { countMatches } from "@/shared/utils/textSearch";
    import { copyTextToClipboard } from "@/shared/utils/clipboard";
    import {
        buildRequestUrl,
        buildColumnTemplate,
        buildColumnWidthMap,
        buildRequestLine,
        buildResponseLine,
        getEntryHost,
        getEntryMethod,
        getEntryPath,
        getEntryPort,
        getEntryStatusLabel,
        getEntryStatusTone,
        getEntryVersion,
        httpEntryColumns,
        sortHttpEntriesByColumn,
        type TableColumnConfig,
    } from "@/shared/utils/httpEntryTable";
    import {
        calculateVirtualizedRange,
        DEFAULT_TABLE_OVERSCAN,
    } from "@/shared/utils/virtualizedTable";
    import {
        getAdjacentVirtualizedSelection,
        scrollVirtualizedRowIntoView,
    } from "@/shared/utils/virtualizedSelection";
    import {
        forgetScrollPosition,
        getRememberedScrollPosition,
        rememberScrollPosition,
    } from "@/shared/utils/viewScrollMemory";
    import {
        buildRequestCopyText,
        type RequestCopyTarget,
    } from "@/shared/utils/requestCopy";

    type ColumnConfig = TableColumnConfig<SortKey>;

    const allColumns: ColumnConfig[] = httpEntryColumns;
    const HISTORY_TABLE_ROW_HEIGHT = 40;
    const HISTORY_TABLE_OVERSCAN = DEFAULT_TABLE_OVERSCAN;
    const DETAIL_LOADING_DELAY_MS = 150;

    type ResizeState = {
        key: SortKey;
        startX: number;
        startWidth: number;
    };

    type BlockSearchId =
        | "requestHead"
        | "requestBody"
        | "responseHead"
        | "responseBody";
    type SearchScopeId = "global" | BlockSearchId;

    type BlockSearchState = Record<BlockSearchId, string>;
    type SearchNavigationState = Record<SearchScopeId, number>;
    type FilterModalState = {
        mode: "create" | "edit";
        title: string;
        submitLabel: string;
        conditions: HistoryFilterCondition[];
        operator: HistoryFilterOperator;
    };
    type HistoryContextMenuState = {
        entry: HistoryEntry;
        x: number;
        y: number;
    };
    type SaveRequestDialogState = {
        entryId: string;
    };
    type AlertDialogState = {
        eyebrow: string;
        title: string;
        message: string;
    };
    type ConfirmDialogState =
        | {
              action: "removeEntry";
              entryId: string;
              eyebrow: string;
              title: string;
              message: string;
              confirmLabel: string;
          }
        | {
              action: "clearAll";
              eyebrow: string;
              title: string;
              message: string;
              confirmLabel: string;
          };

    let mainHistoryTab: HistoryViewTab = {
        id: HISTORY_MAIN_VIEW_TAB_ID,
        label: "All",
        kind: "main",
        filterVersion: 0,
        conditions: [],
        operator: "and",
        sortKey: DEFAULT_HISTORY_SORT_KEY,
        sortDirection: DEFAULT_HISTORY_SORT_DIRECTION,
    };

    const initialBlockSearchState: BlockSearchState = {
        requestHead: "",
        requestBody: "",
        responseHead: "",
        responseBody: "",
    };
    const blockSearchIds: BlockSearchId[] = [
        "requestHead",
        "requestBody",
        "responseHead",
        "responseBody",
    ];

    let listViewport: HTMLDivElement | null = null;
    let tableHeadElement: HTMLDivElement | null = null;
    let historyViewportObserver: ResizeObserver | null = null;
    let historyHeadObserver: ResizeObserver | null = null;
    let observedHistoryViewport: HTMLDivElement | null = null;
    let observedHistoryHead: HTMLDivElement | null = null;
    let columnWidths: Record<SortKey, number> = buildColumnWidthMap(allColumns);
    let activeResize: ResizeState | null = null;
    let historyListScrollTop = 0;
    let historyListViewportHeight = 0;
    let historyTableHeadHeight = 0;
    let globalSearchInput = "";
    let appliedGlobalSearch = "";
    let blockSearchInputs: BlockSearchState = { ...initialBlockSearchState };
    let appliedBlockSearches: BlockSearchState = { ...initialBlockSearchState };
    let requestBodyMatchCount = 0;
    let responseBodyMatchCount = 0;
    let requestHeadContainer: HTMLDivElement | null = null;
    let requestBodyContainer: HTMLDivElement | null = null;
    let responseHeadContainer: HTMLDivElement | null = null;
    let responseBodyContainer: HTMLDivElement | null = null;
    let activeSearchScope: SearchScopeId | null = null;
    let activeSearchElement: HTMLElement | null = null;
    let navigationState: SearchNavigationState = {
        global: -1,
        requestHead: -1,
        requestBody: -1,
        responseHead: -1,
        responseBody: -1,
    };
    let selectedEntryId = "";
    let filterModalState: FilterModalState | null = null;
    let contextMenuState: HistoryContextMenuState | null = null;
    let saveRequestDialogState: SaveRequestDialogState | null = null;
    let alertDialogState: AlertDialogState | null = null;
    let confirmDialogState: ConfirmDialogState | null = null;
    let restoredHistoryListScrollKey = "";
    let initializedPersistedFilters = false;
    let selectedEntryDetail: HistoryEntryDetail | null = null;
    let selectedEntryDetailSourceKey = "";
    let nextSelectedEntryDetailSourceKey = "";
    let selectedEntryDetailRequestToken = 0;
    let selectedEntryDetailLoadingTimer: ReturnType<typeof setTimeout> | null =
        null;
    let selectedEntryDetailState:
        | "idle"
        | "waiting"
        | "loading"
        | "ready"
        | "error" = "idle";
    let selectedEntryDetailError = "";

    $: history = $httpHistoryStore;
    $: historyViewState = $historyViewStore;
    $: mainHistoryTab = {
        id: HISTORY_MAIN_VIEW_TAB_ID,
        label: "All",
        kind: "main",
        filterVersion: 0,
        conditions: [],
        operator: "and",
        sortKey: historyViewState.mainSortKey,
        sortDirection: historyViewState.mainSortDirection,
    };
    $: historyTabs = [mainHistoryTab, ...historyViewState.filterTabs];
    $: activeHistoryTab =
        historyTabs.find((tab) => tab.id === historyViewState.activeTabId) ??
        mainHistoryTab;
    $: activeHistoryFilterKey =
        activeHistoryTab.kind === "filter"
            ? getHistoryFilterKey(
                  activeHistoryTab.id,
                  activeHistoryTab.filterVersion,
              )
            : "";
    $: baseVisibleEntries =
        activeHistoryTab.kind === "main"
            ? history.entries
            : (history.filterEntryIdsByKey
                  .get(activeHistoryFilterKey)
                  ?.map((entryId) => history.entriesById.get(entryId))
                  .filter(
                      (entry): entry is HistoryEntry => entry !== undefined,
                  ) ?? []);
    $: visibleEntries = sortHttpEntriesByColumn(
        baseVisibleEntries,
        activeHistoryTab.sortKey,
        activeHistoryTab.sortDirection,
        getRequestTimeValue,
        (entry) => entry.sequence,
    );
    $: selectedEntry =
        visibleEntries.find((entry) => entry.id === history.selectedId) ??
        visibleEntries[0] ??
        null;
    $: completedCount = visibleEntries.filter(
        (entry) => entry.request !== null && entry.response !== null,
    ).length;
    $: pendingCount = visibleEntries.filter(
        (entry) => entry.request !== null && entry.response === null,
    ).length;
    $: activeHistoryTabSummary =
        activeHistoryTab.kind === "main"
            ? "Compact table of captured requests and responses."
            : getHistoryFilterSummary(
                  activeHistoryTab.conditions,
                  activeHistoryTab.operator,
              );
    $: activeHistoryTabSyncing =
        activeHistoryTab.kind === "filter" &&
        history.syncingFilterKeys.has(activeHistoryFilterKey);
    $: filterActionLabel =
        activeHistoryTab.kind === "main" ? "Filter" : "Edit filter";
    $: historyColumns = buildColumnTemplate(allColumns, columnWidths);
    $: historyListScrollKey = `http-history:list:${activeHistoryTab.id}`;
    $: visibleEntryRange = calculateVirtualizedRange({
        itemCount: visibleEntries.length,
        scrollTop: historyListScrollTop,
        viewportHeight: historyListViewportHeight,
        rowHeight: HISTORY_TABLE_ROW_HEIGHT,
        overscan: HISTORY_TABLE_OVERSCAN,
        stickyOffset: historyTableHeadHeight,
    });
    $: virtualVisibleEntries = visibleEntries.slice(
        visibleEntryRange.startIndex,
        visibleEntryRange.endIndex,
    );
    $: requestHeadQuery =
        appliedBlockSearches.requestHead.trim().length > 0
            ? appliedBlockSearches.requestHead.trim()
            : appliedGlobalSearch;
    $: requestBodyQuery =
        appliedBlockSearches.requestBody.trim().length > 0
            ? appliedBlockSearches.requestBody.trim()
            : appliedGlobalSearch;
    $: responseHeadQuery =
        appliedBlockSearches.responseHead.trim().length > 0
            ? appliedBlockSearches.responseHead.trim()
            : appliedGlobalSearch;
    $: responseBodyQuery =
        appliedBlockSearches.responseBody.trim().length > 0
            ? appliedBlockSearches.responseBody.trim()
            : appliedGlobalSearch;
    $: requestHeadText = selectedEntryDetail?.request?.headBlockStr ?? "";
    $: requestBodyText = selectedEntryDetail?.request?.bodyStr ?? "";
    $: responseHeadText = selectedEntryDetail?.response?.headBlockStr ?? "";
    $: responseBodyText = selectedEntryDetail?.response?.bodyStr ?? "";
    $: requestHeadHasContent = hasSearchableContent(requestHeadText);
    $: requestBodyHasContent = hasSearchableContent(requestBodyText);
    $: responseHeadHasContent = hasSearchableContent(responseHeadText);
    $: responseBodyHasContent = hasSearchableContent(responseBodyText);
    $: requestHeadMatchCount = countMatches(requestHeadText, requestHeadQuery);
    $: responseHeadMatchCount = countMatches(
        responseHeadText,
        responseHeadQuery,
    );
    $: globalMatchCount =
        requestHeadMatchCount +
        requestBodyMatchCount +
        responseHeadMatchCount +
        responseBodyMatchCount;
    $: globalSearchProgress = buildSearchProgressLabel(
        appliedGlobalSearch,
        globalMatchCount,
        navigationState.global,
    );
    $: requestHeadSearchProgress = buildSearchProgressLabel(
        requestHeadQuery,
        requestHeadMatchCount,
        navigationState.requestHead,
    );
    $: requestBodySearchProgress = buildSearchProgressLabel(
        requestBodyQuery,
        requestBodyMatchCount,
        navigationState.requestBody,
    );
    $: responseHeadSearchProgress = buildSearchProgressLabel(
        responseHeadQuery,
        responseHeadMatchCount,
        navigationState.responseHead,
    );
    $: responseBodySearchProgress = buildSearchProgressLabel(
        responseBodyQuery,
        responseBodyMatchCount,
        navigationState.responseBody,
    );
    $: if ((selectedEntry?.id ?? "") !== selectedEntryId) {
        selectedEntryId = selectedEntry?.id ?? "";
        resetSearchNavigation();
    }
    $: nextSelectedEntryDetailSourceKey = selectedEntry
        ? `${selectedEntry.id}:${selectedEntry.requestArrivedAtMs ?? 0}:${selectedEntry.response?.receivedAtMs ?? 0}`
        : "";
    $: if (nextSelectedEntryDetailSourceKey !== selectedEntryDetailSourceKey) {
        selectedEntryDetailSourceKey = nextSelectedEntryDetailSourceKey;
        void loadSelectedEntryDetail(selectedEntry?.id ?? null);
    }
    $: if (
        listViewport &&
        historyListScrollKey !== restoredHistoryListScrollKey
    ) {
        restoredHistoryListScrollKey = historyListScrollKey;

        tick().then(() => {
            if (!listViewport) return;
            listViewport.scrollTop =
                getRememberedScrollPosition(historyListScrollKey);
            historyListScrollTop = listViewport.scrollTop;
            updateHistoryViewportMetrics();
        });
    }
    $: if (listViewport) {
        updateHistoryViewportMetrics();
    }
    $: if (tableHeadElement) {
        updateHistoryViewportMetrics();
    }
    $: if (listViewport !== observedHistoryViewport) {
        historyViewportObserver?.disconnect();
        observedHistoryViewport = listViewport;

        if (listViewport && typeof ResizeObserver !== "undefined") {
            historyViewportObserver = new ResizeObserver(() => {
                updateHistoryViewportMetrics();
            });
            historyViewportObserver.observe(listViewport);
        }

        tick().then(() => {
            updateHistoryViewportMetrics();
        });
    }

    onMount(() => {
        if (initializedPersistedFilters) {
            return;
        }

        initializedPersistedFilters = true;
        void syncPersistedHistoryFilters(historyViewState.filterTabs).catch(
            () => {
                openAlertDialog(
                    "Could not restore filters",
                    "The frontend could not register the persisted filtered tabs in the backend.",
                    "HTTP History",
                );
            },
        );
    });
    $: if (tableHeadElement !== observedHistoryHead) {
        historyHeadObserver?.disconnect();
        observedHistoryHead = tableHeadElement;

        if (tableHeadElement && typeof ResizeObserver !== "undefined") {
            historyHeadObserver = new ResizeObserver(() => {
                updateHistoryViewportMetrics();
            });
            historyHeadObserver.observe(tableHeadElement);
        }

        tick().then(() => {
            updateHistoryViewportMetrics();
        });
    }

    function getHost(entry: HistoryEntry): string {
        return getEntryHost(entry);
    }

    function getPath(entry: HistoryEntry): string {
        return getEntryPath(entry);
    }

    function getPort(entry: HistoryEntry): string {
        return getEntryPort(entry);
    }

    function getMethod(entry: HistoryEntry): string {
        return getEntryMethod(entry);
    }

    function getVersion(entry: HistoryEntry): string {
        return getEntryVersion(entry);
    }

    function getStatusLabel(entry: HistoryEntry): string {
        return getEntryStatusLabel(entry);
    }

    function getStatusTone(entry: HistoryEntry): string {
        return getEntryStatusTone(entry);
    }

    function getRequestLine(entry: HistoryEntry): string {
        return buildRequestLine(entry.request);
    }

    function getResponseLine(entry: HistoryEntry): string {
        return buildResponseLine(entry.response);
    }

    function getRequestTimeValue(entry: HistoryEntry): number {
        return entry.requestArrivedAtMs ?? entry.firstSeenAtMs;
    }

    function getSortDirection(key: SortKey): SortDirection | null {
        if (activeHistoryTab.sortKey !== key) return null;
        return activeHistoryTab.sortDirection;
    }

    function toggleActiveHistorySort(key: SortKey) {
        toggleHistoryViewTabSort(activeHistoryTab.id, key);
    }

    function buildFilterModalState(mode: "create" | "edit"): FilterModalState {
        if (mode === "create" || activeHistoryTab.kind === "main") {
            return {
                mode: "create",
                title: "Create filtered view",
                submitLabel: "Create filter",
                conditions: [createHistoryFilterCondition()],
                operator: "and",
            };
        }

        return {
            mode: "edit",
            title: "Edit filter",
            submitLabel: "Save changes",
            conditions: activeHistoryTab.conditions,
            operator: activeHistoryTab.operator,
        };
    }

    function activateHistoryTab(tabId: string) {
        activateHistoryViewTab(tabId);
    }

    async function handleCloseHistoryTab(event: MouseEvent, tabId: string) {
        event.preventDefault();
        event.stopPropagation();

        const closingTab = historyTabs.find((tab) => tab.id === tabId);
        if (closingTab?.kind === "filter") {
            try {
                await removeHistoryFilterTab(
                    closingTab.id,
                    closingTab.filterVersion,
                );
            } catch {
                openAlertDialog(
                    "Could not close the filter",
                    "The backend could not remove the active filter. The tab was left untouched to avoid inconsistencies.",
                    "HTTP History",
                );
                return;
            }
        }

        forgetScrollPosition(`http-history:list:${tabId}`);
        closeHistoryViewTab(tabId);
    }

    function handleHistoryListScroll() {
        if (!listViewport) return;

        historyListScrollTop = listViewport.scrollTop;
        updateHistoryViewportMetrics();
        rememberScrollPosition(historyListScrollKey, listViewport.scrollTop);
    }

    function updateHistoryViewportMetrics() {
        historyListViewportHeight = listViewport?.clientHeight ?? 0;
        historyTableHeadHeight = tableHeadElement?.offsetHeight ?? 0;
    }

    async function focusHistoryRow(entryId: string) {
        await tick();

        const rowElement =
            listViewport?.querySelector<HTMLButtonElement>(
                `[data-entry-id="${entryId}"]`,
            ) ?? null;

        rowElement?.focus();
    }

    async function moveHistorySelection(direction: -1 | 1) {
        const nextSelection = getAdjacentVirtualizedSelection({
            items: visibleEntries,
            selectedId: selectedEntry?.id ?? null,
            getId: (entry) => entry.id,
            direction,
        });
        if (!nextSelection) return;

        scrollVirtualizedRowIntoView({
            viewport: listViewport,
            rowIndex: nextSelection.index,
            rowHeight: HISTORY_TABLE_ROW_HEIGHT,
            stickyOffset: historyTableHeadHeight,
        });
        handleHistoryListScroll();
        selectHistoryEntry(nextSelection.id);
        await focusHistoryRow(nextSelection.id);
    }

    function handleHistoryTableKeydown(event: KeyboardEvent) {
        const target =
            event.target instanceof HTMLElement ? event.target : null;
        if (!target?.closest(".tableRow")) return;
        if (event.altKey || event.ctrlKey || event.metaKey) return;
        if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return;

        event.preventDefault();
        void moveHistorySelection(event.key === "ArrowDown" ? 1 : -1);
    }

    function openFilterModalForActiveTab() {
        if (filterModalState) return;

        filterModalState = buildFilterModalState(
            activeHistoryTab.kind === "main" ? "create" : "edit",
        );
    }

    function closeFilterModal() {
        filterModalState = null;
    }

    async function saveFilterView(
        event: CustomEvent<{
            conditions: HistoryFilterCondition[];
            operator: HistoryFilterOperator;
        }>,
    ) {
        const { conditions, operator } = event.detail;

        if (
            filterModalState?.mode === "edit" &&
            activeHistoryTab.kind === "filter"
        ) {
            const updatedTab = updateHistoryViewTab(
                activeHistoryTab.id,
                conditions,
                operator,
            );
            if (!updatedTab) {
                openAlertDialog(
                    "Could not update the filter",
                    "The filtered tab no longer exists or could not be updated correctly.",
                    "HTTP History",
                );
                return;
            }

            try {
                await upsertHistoryFilterTab(updatedTab);
            } catch {
                openAlertDialog(
                    "Could not update the filter",
                    "The backend did not accept the new filter. The definition was saved in the tab, but it was not synchronized.",
                    "HTTP History",
                );
            }
            filterModalState = null;
            return;
        }

        const createdTab = createHistoryViewTab(conditions, operator);
        try {
            await upsertHistoryFilterTab(createdTab);
        } catch {
            openAlertDialog(
                "Could not create the filter",
                "The tab was created in the frontend, but the backend could not register it yet.",
                "HTTP History",
            );
        }
        filterModalState = null;
    }

    function hasSearchableContent(value: string | null | undefined): boolean {
        return (value ?? "").trim().length > 0;
    }

    function openHistoryContextMenu(event: MouseEvent, entry: HistoryEntry) {
        event.preventDefault();

        selectHistoryEntry(entry.id);
        contextMenuState = {
            entry,
            x: Math.min(event.clientX, window.innerWidth - 240),
            y: Math.min(event.clientY, window.innerHeight - 200),
        };
    }

    function closeHistoryContextMenu() {
        contextMenuState = null;
    }

    function openRemoveEntryDialog() {
        if (!contextMenuState) return;

        confirmDialogState = {
            action: "removeEntry",
            entryId: contextMenuState.entry.id,
            eyebrow: "HTTP History",
            title: "Delete this request",
            message:
                "The selected request will disappear from history and from any filtered tabs where it is currently visible.",
            confirmLabel: "Delete request",
        };
        closeHistoryContextMenu();
    }

    function openClearHistoryDialog() {
        confirmDialogState = {
            action: "clearAll",
            eyebrow: "HTTP History",
            title: "Delete all requests",
            message:
                "The full visible history in HTTP History will be cleared. Filtered tabs will remain, but they will stay empty until new requests arrive.",
            confirmLabel: "Clear history",
        };
        closeHistoryContextMenu();
    }

    function closeConfirmDialog() {
        confirmDialogState = null;
    }

    async function sendContextEntryToRepeater() {
        if (!contextMenuState) return;

        const detail = await getDetailForEntryId(contextMenuState.entry.id);
        if (!detail?.request) {
            return;
        }

        openRepeaterTabFromRequestSnapshot(detail.request, detail.id);
        setActiveWorkspaceTab("repeater");
        closeHistoryContextMenu();
    }

    async function copyContextRequest(
        event: CustomEvent<{ target: RequestCopyTarget }>,
    ) {
        const detail = contextMenuState
            ? await getDetailForEntryId(contextMenuState.entry.id)
            : null;
        const request = detail?.request;
        if (!request) return;

        const copyTarget = event.detail.target;
        const requiresUrl =
            copyTarget !== "js-headers" && copyTarget !== "python-headers";
        const requestUrl = buildRequestUrl(request);

        if (requiresUrl && !requestUrl) {
            openAlertDialog(
                "URL unavailable",
                "This request does not have enough scheme, host, or path information to build a URL or request snippet.",
                "HTTP History",
            );
            closeHistoryContextMenu();
            return;
        }

        try {
            await copyTextToClipboard(
                buildRequestCopyText(request, copyTarget),
            );
        } catch {
            openAlertDialog(
                "Could not copy",
                "The system did not allow the requested content to be copied to the clipboard.",
                "HTTP History",
            );
        }

        closeHistoryContextMenu();
    }

    function openSaveRequestDialog() {
        if (!contextMenuState) return;
        if (hasSavedRequestForHistoryEntry(contextMenuState.entry.id)) {
            openAlertDialog(
                "Request already saved",
                "This request is already saved in Saved Requests and cannot be duplicated.",
                "Saved Requests",
            );
            closeHistoryContextMenu();
            return;
        }

        saveRequestDialogState = {
            entryId: contextMenuState.entry.id,
        };
        closeHistoryContextMenu();
    }

    function closeSaveRequestDialog() {
        saveRequestDialogState = null;
    }

    function openAlertDialog(
        title: string,
        message: string,
        eyebrow = "HTTP History",
    ) {
        alertDialogState = {
            eyebrow,
            title,
            message,
        };
    }

    function closeAlertDialog() {
        alertDialogState = null;
    }

    async function confirmHistoryAction() {
        if (!confirmDialogState) return;

        const currentConfirmState = confirmDialogState;
        confirmDialogState = null;

        try {
            if (currentConfirmState.action === "removeEntry") {
                await removeHistoryEntryById(currentConfirmState.entryId);
                return;
            }

            await clearAllHistoryEntries();
        } catch {
            openAlertDialog(
                "Could not update history",
                currentConfirmState.action === "removeEntry"
                    ? "The backend could not delete the selected request."
                    : "The backend could not clear the full history.",
                "HTTP History",
            );
        }
    }

    async function getDetailForEntryId(
        entryId: string,
    ): Promise<HistoryEntryDetail | null> {
        try {
            return await getHistoryEntryDetailCached(entryId);
        } catch {
            openAlertDialog(
                "Could not load details",
                "The backend did not return the full details for the selected request.",
                "HTTP History",
            );
            return null;
        }
    }

    async function loadSelectedEntryDetail(entryId: string | null) {
        clearSelectedEntryDetailLoadingTimer();
        selectedEntryDetailError = "";

        if (!entryId) {
            selectedEntryDetail = null;
            selectedEntryDetailState = "idle";
            return;
        }

        const cachedDetail = getCachedHistoryEntryDetail(entryId);
        if (cachedDetail) {
            selectedEntryDetail = cachedDetail;
            selectedEntryDetailState = "ready";
            return;
        }

        const requestToken = selectedEntryDetailRequestToken + 1;
        selectedEntryDetailRequestToken = requestToken;
        selectedEntryDetailState = "waiting";
        selectedEntryDetailLoadingTimer = setTimeout(() => {
            if (selectedEntryDetailRequestToken !== requestToken) {
                return;
            }

            selectedEntryDetail = null;
            selectedEntryDetailState = "loading";
        }, DETAIL_LOADING_DELAY_MS);

        try {
            const detail = await getHistoryEntryDetailCached(entryId);
            if (selectedEntryDetailRequestToken !== requestToken) {
                return;
            }

            clearSelectedEntryDetailLoadingTimer();
            selectedEntryDetail = detail;
            selectedEntryDetailState = "ready";
        } catch {
            if (selectedEntryDetailRequestToken !== requestToken) {
                return;
            }

            clearSelectedEntryDetailLoadingTimer();
            selectedEntryDetail = null;
            selectedEntryDetailState = "error";
            selectedEntryDetailError =
                "Could not load the full details for this request.";
        }
    }

    function clearSelectedEntryDetailLoadingTimer() {
        if (selectedEntryDetailLoadingTimer === null) {
            return;
        }

        clearTimeout(selectedEntryDetailLoadingTimer);
        selectedEntryDetailLoadingTimer = null;
    }

    async function saveContextEntryAsSnapshot(
        event: CustomEvent<{ name: string }>,
    ) {
        if (!saveRequestDialogState) return;

        const detail = await getDetailForEntryId(
            saveRequestDialogState.entryId,
        );
        if (!detail?.request) {
            return;
        }

        const saved = saveHistoryEntrySnapshot(detail, event.detail.name);

        if (!saved) {
            openAlertDialog(
                "Request already saved",
                "This request is already saved in Saved Requests and cannot be duplicated.",
                "Saved Requests",
            );
        }

        closeSaveRequestDialog();
    }

    function handleHistoryShortcut(event: KeyboardEvent) {
        if (event.key === "Escape" && confirmDialogState) {
            event.preventDefault();
            closeConfirmDialog();
            return;
        }

        if (event.key === "Escape" && contextMenuState) {
            event.preventDefault();
            closeHistoryContextMenu();
            return;
        }

        const isFindShortcut =
            (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "f";

        if (!isFindShortcut) return;

        event.preventDefault();

        if (filterModalState) return;

        openFilterModalForActiveTab();
    }

    async function applyGlobalSearch() {
        const nextQuery = globalSearchInput.trim();
        const isConsecutive =
            nextQuery.length > 0 &&
            nextQuery === appliedGlobalSearch &&
            activeSearchScope === "global";

        if (nextQuery.length > 0) {
            clearBlockSearches();
        }

        appliedGlobalSearch = nextQuery;
        await tick();

        if (nextQuery.length === 0) {
            resetSearchNavigation();
            return;
        }

        await navigateScopeMatches("global", isConsecutive ? 1 : 0);
    }

    async function applyBlockSearch(blockId: BlockSearchId) {
        const nextQuery = blockSearchInputs[blockId].trim();
        const isConsecutive =
            nextQuery.length > 0 &&
            nextQuery === appliedBlockSearches[blockId].trim() &&
            activeSearchScope === blockId;

        appliedBlockSearches = {
            ...appliedBlockSearches,
            [blockId]: nextQuery,
        };
        await tick();

        if (nextQuery.length === 0) {
            resetScopeNavigation(blockId);
            return;
        }

        await navigateScopeMatches(blockId, isConsecutive ? 1 : 0);
    }

    function buildSearchProgressLabel(
        query: string,
        count: number,
        currentIndex: number,
    ): string {
        if (query.trim().length === 0 || count <= 0) return "0/0";
        if (currentIndex < 0) return `0/${count}`;

        const clampedIndex = Math.max(0, Math.min(currentIndex, count - 1));
        return `${clampedIndex + 1}/${count}`;
    }

    function getScopeQuery(scopeId: SearchScopeId): string {
        switch (scopeId) {
            case "global":
                return appliedGlobalSearch;
            case "requestHead":
                return requestHeadQuery;
            case "requestBody":
                return requestBodyQuery;
            case "responseHead":
                return responseHeadQuery;
            case "responseBody":
                return responseBodyQuery;
        }
    }

    function getContainer(blockId: BlockSearchId): HTMLDivElement | null {
        switch (blockId) {
            case "requestHead":
                return requestHeadContainer;
            case "requestBody":
                return requestBodyContainer;
            case "responseHead":
                return responseHeadContainer;
            case "responseBody":
                return responseBodyContainer;
        }
    }

    function getScopeMatchElements(scopeId: SearchScopeId): HTMLElement[] {
        if (scopeId === "global") {
            return [
                requestHeadContainer,
                requestBodyContainer,
                responseHeadContainer,
                responseBodyContainer,
            ]
                .filter(Boolean)
                .flatMap((container) =>
                    Array.from(
                        container!.querySelectorAll<HTMLElement>(
                            "mark.searchHit",
                        ),
                    ),
                );
        }

        const container = getContainer(scopeId);
        if (!container) return [];

        return Array.from(
            container.querySelectorAll<HTMLElement>("mark.searchHit"),
        );
    }

    function revealMatchAncestors(matchElement: HTMLElement) {
        const jsonNodesToOpen: HTMLElement[] = [];
        let currentElement: HTMLElement | null = matchElement.parentElement;

        while (currentElement) {
            if (currentElement.tagName === "DETAILS") {
                (currentElement as HTMLDetailsElement).open = true;
            }

            if (currentElement.classList.contains("jsonNode")) {
                jsonNodesToOpen.push(currentElement);
            }

            currentElement = currentElement.parentElement;
        }

        for (let index = jsonNodesToOpen.length - 1; index >= 0; index -= 1) {
            const jsonNode = jsonNodesToOpen[index];
            const toggleButton =
                jsonNode.querySelector<HTMLButtonElement>(".jsonToggleButton");

            if (toggleButton?.getAttribute("aria-expanded") === "false") {
                toggleButton.click();
            }
        }
    }

    function clearActiveSearchElement() {
        if (!activeSearchElement) return;

        activeSearchElement.classList.remove("activeSearchHit");
        activeSearchElement.setAttribute("tabindex", "-1");

        if (document.activeElement === activeSearchElement) {
            (activeSearchElement as HTMLElement).blur();
        }

        activeSearchElement = null;
    }

    function scrollMatchIntoView(matchElement: HTMLElement) {
        const interactiveCodeShell = matchElement.closest<HTMLElement>(
            ".codeBlockShell.interactive",
        );

        if (interactiveCodeShell) {
            const renderViewport =
                interactiveCodeShell.querySelector<HTMLElement>(
                    ".codeRenderViewport",
                );
            const inputOverlay =
                interactiveCodeShell.querySelector<HTMLElement>(
                    ".codeInputOverlay",
                );

            if (renderViewport && inputOverlay) {
                const viewportRect = renderViewport.getBoundingClientRect();
                const matchRect = matchElement.getBoundingClientRect();
                const verticalMargin = 28;
                const horizontalMargin = 20;

                let nextTop = inputOverlay.scrollTop;
                let nextLeft = inputOverlay.scrollLeft;

                if (matchRect.top < viewportRect.top + verticalMargin) {
                    nextTop -=
                        viewportRect.top + verticalMargin - matchRect.top;
                } else if (
                    matchRect.bottom >
                    viewportRect.bottom - verticalMargin
                ) {
                    nextTop +=
                        matchRect.bottom -
                        (viewportRect.bottom - verticalMargin);
                }

                if (matchRect.left < viewportRect.left + horizontalMargin) {
                    nextLeft -=
                        viewportRect.left + horizontalMargin - matchRect.left;
                } else if (
                    matchRect.right >
                    viewportRect.right - horizontalMargin
                ) {
                    nextLeft +=
                        matchRect.right -
                        (viewportRect.right - horizontalMargin);
                }

                inputOverlay.scrollTo({
                    top: Math.max(0, nextTop),
                    left: Math.max(0, nextLeft),
                    behavior: "smooth",
                });
                return;
            }
        }

        matchElement.scrollIntoView({
            block: "center",
            inline: "nearest",
            behavior: "smooth",
        });
    }

    function clearBlockSearches() {
        if (activeSearchScope !== "global") {
            clearActiveSearchElement();
            activeSearchScope = null;
        }

        blockSearchInputs = { ...initialBlockSearchState };
        appliedBlockSearches = { ...initialBlockSearchState };

        navigationState = blockSearchIds.reduce(
            (nextState, blockId) => ({
                ...nextState,
                [blockId]: -1,
            }),
            { ...navigationState },
        );
    }

    function resetScopeNavigation(scopeId: SearchScopeId) {
        if (activeSearchScope === scopeId) {
            clearActiveSearchElement();
            activeSearchScope = null;
        }

        navigationState = {
            ...navigationState,
            [scopeId]: -1,
        };
    }

    function resetSearchNavigation() {
        clearActiveSearchElement();
        activeSearchScope = null;
        navigationState = {
            global: -1,
            requestHead: -1,
            requestBody: -1,
            responseHead: -1,
            responseBody: -1,
        };
    }

    async function prepareScopeNavigation(
        scopeId: SearchScopeId,
    ): Promise<boolean> {
        if (getScopeQuery(scopeId).trim().length === 0) {
            return false;
        }

        if (scopeId === "global" && activeSearchScope !== "global") {
            clearBlockSearches();
            await tick();
            activeSearchScope = "global";
            return true;
        }

        if (activeSearchScope !== scopeId) {
            clearActiveSearchElement();
            activeSearchScope = scopeId;
        }

        return true;
    }

    async function navigateSearchMatches(
        scopeId: SearchScopeId,
        step: number,
    ): Promise<void> {
        const canNavigate = await prepareScopeNavigation(scopeId);
        if (!canNavigate) return;

        await navigateScopeMatches(scopeId, step);
    }

    async function navigateScopeMatches(
        scopeId: SearchScopeId,
        step: number,
    ): Promise<void> {
        await tick();

        const matches = getScopeMatchElements(scopeId);
        if (matches.length === 0) {
            resetScopeNavigation(scopeId);
            return;
        }

        const currentIndex =
            activeSearchScope === scopeId ? navigationState[scopeId] : -1;
        const baseIndex = currentIndex >= 0 ? currentIndex : 0;
        const nextIndex =
            currentIndex >= 0
                ? (baseIndex + step + matches.length) % matches.length
                : 0;

        clearActiveSearchElement();

        const nextMatch = matches[nextIndex];
        revealMatchAncestors(nextMatch);
        await tick();

        nextMatch.setAttribute("tabindex", "0");
        nextMatch.classList.add("activeSearchHit");
        nextMatch.focus({ preventScroll: true });
        scrollMatchIntoView(nextMatch);

        requestAnimationFrame(() => {
            if (activeSearchElement === nextMatch) {
                nextMatch.focus({ preventScroll: true });
            }
        });

        activeSearchElement = nextMatch;
        activeSearchScope = scopeId;
        navigationState = {
            ...navigationState,
            [scopeId]: nextIndex,
        };
    }

    function getColumnConfig(key: SortKey): ColumnConfig {
        return allColumns.find((column) => column.key === key) ?? allColumns[0];
    }

    function startColumnResize(event: MouseEvent, key: SortKey) {
        event.preventDefault();
        event.stopPropagation();

        activeResize = {
            key,
            startX: event.clientX,
            startWidth: columnWidths[key],
        };

        window.addEventListener("mousemove", handleColumnResize);
        window.addEventListener("mouseup", stopColumnResize);
        document.body.style.cursor = "col-resize";
        document.body.style.userSelect = "none";
    }

    function handleColumnResize(event: MouseEvent) {
        if (!activeResize) return;

        const config = getColumnConfig(activeResize.key);
        const nextWidth = Math.max(
            config.minWidth,
            activeResize.startWidth + (event.clientX - activeResize.startX),
        );

        columnWidths = {
            ...columnWidths,
            [activeResize.key]: nextWidth,
        };
    }

    function stopColumnResize() {
        if (!activeResize) return;

        activeResize = null;
        window.removeEventListener("mousemove", handleColumnResize);
        window.removeEventListener("mouseup", stopColumnResize);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
    }

    onDestroy(() => {
        historyViewportObserver?.disconnect();
        historyHeadObserver?.disconnect();
        clearSelectedEntryDetailLoadingTimer();
        if (listViewport) {
            rememberScrollPosition(
                historyListScrollKey,
                listViewport.scrollTop,
            );
        }
        stopColumnResize();
    });
</script>

<svelte:window
    on:click={closeHistoryContextMenu}
    on:keydown={handleHistoryShortcut}
    on:resize={closeHistoryContextMenu}
    on:resize={updateHistoryViewportMetrics}
    on:scroll={closeHistoryContextMenu}
/>

<div class="historyView">
    <div class="listCard">
        <div class="historyTabsBar">
            <div
                class="historyTabs"
                role="tablist"
                aria-label="HTTP history views"
            >
                {#each historyTabs as tab (tab.id)}
                    <div
                        class="historyTabItem"
                        class:active={tab.id === activeHistoryTab.id}
                    >
                        <button
                            type="button"
                            class="historyTab"
                            class:active={tab.id === activeHistoryTab.id}
                            on:click={() => activateHistoryTab(tab.id)}
                            title={tab.kind === "main"
                                ? "All captured requests"
                                : getHistoryFilterSummary(
                                      tab.conditions,
                                      tab.operator,
                                  )}
                        >
                            <span class="historyTabLabel">{tab.label}</span>
                            {#if tab.kind === "filter"}
                                <span class="historyTabMeta"
                                    >{tab.conditions.length} cond.</span
                                >
                            {/if}
                        </button>

                        {#if tab.kind === "filter"}
                            <button
                                type="button"
                                class="closeHistoryTabButton"
                                aria-label={`Close ${tab.label}`}
                                title={`Close ${tab.label}`}
                                on:click={(event) =>
                                    handleCloseHistoryTab(event, tab.id)}
                            >
                                ×
                            </button>
                        {/if}
                    </div>
                {/each}
            </div>

            <button
                type="button"
                class="filterActionButton"
                on:click={openFilterModalForActiveTab}
            >
                {filterActionLabel}
            </button>
        </div>

        <div class="cardHeader">
            <div class="headerCopy">
                <span class="eyebrow">Table View</span>
                <h3>HTTP History</h3>
                <p class="headerSub">
                    {activeHistoryTabSummary}
                </p>
            </div>

            <div class="overviewStats" aria-label="History summary">
                <div class="statChip">
                    <span>Total</span>
                    <strong>{visibleEntries.length}</strong>
                </div>
                <div class="statChip">
                    <span>Complete</span>
                    <strong>{completedCount}</strong>
                </div>
                <div class="statChip">
                    <span>Pending</span>
                    <strong>{pendingCount}</strong>
                </div>
            </div>
        </div>

        {#if history.entries.length === 0}
            <div class="emptyState">
                <div class="emptyTitle">No traffic yet</div>
                <div class="emptySub">
                    Requests will appear here in a compact workbench-style row.
                    Headers and body stay hidden until you select one.
                </div>
            </div>
        {:else if activeHistoryTabSyncing && visibleEntries.length === 0}
            <div class="emptyState">
                <div class="emptyTitle">Updating filter</div>
                <div class="emptySub">
                    The backend is recalculating this filtered view using the
                    current history.
                </div>
            </div>
        {:else if visibleEntries.length === 0}
            <div class="emptyState">
                <div class="emptyTitle">This filter has no matches</div>
                <div class="emptySub">
                    Adjust the conditions in this tab or go back to the main
                    view to create another filter.
                </div>
            </div>
        {:else}
            <div
                class="tableViewport"
                bind:this={listViewport}
                on:keydown={handleHistoryTableKeydown}
                on:scroll={handleHistoryListScroll}
            >
                <div
                    class="historyTable"
                    style={`--history-columns: ${historyColumns}; --history-row-height: ${HISTORY_TABLE_ROW_HEIGHT}px;`}
                >
                    <div
                        class="tableHead"
                        role="row"
                        bind:this={tableHeadElement}
                    >
                        {#each allColumns as column (column.key)}
                            <div class="headColumn">
                                <button
                                    type="button"
                                    class="headCell"
                                    class:sorted={activeHistoryTab.sortKey ===
                                        column.key}
                                    on:click={() =>
                                        toggleActiveHistorySort(column.key)}
                                >
                                    <span>{column.label}</span>
                                    <span
                                        class="sortIndicator"
                                        class:active={getSortDirection(
                                            column.key,
                                        ) !== null}
                                        class:asc={getSortDirection(
                                            column.key,
                                        ) === "asc"}
                                        class:desc={getSortDirection(
                                            column.key,
                                        ) === "desc"}
                                        aria-hidden="true"
                                    >
                                        {#if getSortDirection(column.key) === "asc"}
                                            ↑
                                        {:else if getSortDirection(column.key) === "desc"}
                                            ↓
                                        {:else}
                                            ↕
                                        {/if}
                                    </span>
                                </button>
                                <button
                                    type="button"
                                    class="resizeHandle"
                                    aria-label={`Resize ${column.label} column`}
                                    title={`Resize ${column.label}`}
                                    on:mousedown={(event) =>
                                        startColumnResize(event, column.key)}
                                ></button>
                            </div>
                        {/each}
                    </div>

                    <div
                        class="tableBody"
                        style={`height: ${visibleEntryRange.totalHeight}px;`}
                    >
                        <div
                            class="virtualRows"
                            style={`transform: translateY(${visibleEntryRange.offsetTop}px);`}
                        >
                            {#each virtualVisibleEntries as entry (entry.id)}
                                <button
                                    type="button"
                                    class="tableRow"
                                    class:selected={entry.id ===
                                        selectedEntry?.id}
                                    data-entry-id={entry.id}
                                    tabindex={entry.id === selectedEntry?.id
                                        ? 0
                                        : -1}
                                    style={`height: ${HISTORY_TABLE_ROW_HEIGHT}px;`}
                                    on:click={() =>
                                        selectHistoryEntry(entry.id)}
                                    on:contextmenu={(event) =>
                                        openHistoryContextMenu(event, entry)}
                                >
                                    {#each allColumns as column (column.key)}
                                        {#if column.key === "time"}
                                            <div class="cell mono">
                                                {entry.requestTimeLabel}
                                            </div>
                                        {:else if column.key === "host"}
                                            <div
                                                class="cell strong"
                                                title={getHost(entry)}
                                            >
                                                {getHost(entry)}
                                            </div>
                                        {:else if column.key === "path"}
                                            <div
                                                class="cell mono"
                                                title={getPath(entry)}
                                            >
                                                {getPath(entry)}
                                            </div>
                                        {:else if column.key === "port"}
                                            <div class="cell mono">
                                                {getPort(entry)}
                                            </div>
                                        {:else if column.key === "method"}
                                            <div class="cell methodCell">
                                                {getMethod(entry)}
                                            </div>
                                        {:else if column.key === "version"}
                                            <div class="cell mono">
                                                {getVersion(entry)}
                                            </div>
                                        {:else}
                                            <div class="cell statusCell">
                                                <span
                                                    class={`statusBadge ${getStatusTone(entry)}`}
                                                >
                                                    {getStatusLabel(entry)}
                                                </span>
                                            </div>
                                        {/if}
                                    {/each}
                                </button>
                            {/each}
                        </div>
                    </div>
                </div>
            </div>
        {/if}
    </div>

    <div class="detailCard">
        <div class="cardHeader detailTop">
            <div>
                <span class="eyebrow">Detail</span>
                <h3>Selected entry</h3>
            </div>

            <div class="detailHeaderTools">
                <HistorySearchForm
                    bind:value={globalSearchInput}
                    placeholder="Search within request and response"
                    progress={globalSearchProgress}
                    submitDisabled={!selectedEntry}
                    navigateDisabled={!selectedEntry ||
                        appliedGlobalSearch.trim().length === 0 ||
                        globalMatchCount === 0}
                    previousLabel="Go to previous match"
                    nextLabel="Go to next match"
                    on:submit={applyGlobalSearch}
                    on:previous={() => navigateSearchMatches("global", -1)}
                    on:next={() => navigateSearchMatches("global", 1)}
                />

                {#if selectedEntry}
                    <div class="detailBadges">
                        <span class="metaPill"
                            >{selectedEntry.requestTimeLabel}</span
                        >
                        <span class="metaPill">{getHost(selectedEntry)}</span>
                        <span class="metaPill">:{getPort(selectedEntry)}</span>
                        <span class="metaPill">{getMethod(selectedEntry)}</span>
                        <span
                            class={`metaPill ${getStatusTone(selectedEntry)}`}
                        >
                            {getStatusLabel(selectedEntry)}
                        </span>
                    </div>
                {:else}
                    <span class="headerHint">Select a row from history</span>
                {/if}
            </div>
        </div>

        {#if selectedEntry}
            <div class="detailGrid">
                <section class="messageCard">
                    <div class="messageHeader">
                        <span>Request</span>
                        <strong title={getRequestLine(selectedEntry)}
                            >{getRequestLine(selectedEntry)}</strong
                        >
                    </div>

                    <div class="requestMetaPanel">
                        <div class="messageMeta">
                            <span
                                >Host: {selectedEntry.request?.host ||
                                    "-"}</span
                            >
                            <span
                                >Port: {selectedEntry.request?.port ||
                                    "-"}</span
                            >
                        </div>

                        <RequestPathSummary
                            path={selectedEntry.request?.path || "-"}
                        />
                    </div>

                    {#if selectedEntryDetailState === "loading"}
                        <div class="detailSkeleton" aria-hidden="true">
                            <div class="detailSkeletonHeader">
                                <span class="detailSkeletonLine short"></span>
                                <span class="detailSkeletonLine medium"></span>
                            </div>
                            <div class="detailSkeletonBody">
                                <span class="detailSkeletonLine long"></span>
                                <span class="detailSkeletonLine long"></span>
                                <span class="detailSkeletonLine medium"></span>
                                <span class="detailSkeletonLine short"></span>
                            </div>
                        </div>
                    {:else if selectedEntryDetailState === "waiting" && !selectedEntryDetail}
                        <div class="detailSpacer" aria-hidden="true"></div>
                    {:else if selectedEntryDetailState === "error"}
                        <div class="emptyState detailEmpty">
                            <div class="emptyTitle">
                                Could not load the request
                            </div>
                            <div class="emptySub">
                                {selectedEntryDetailError}
                            </div>
                        </div>
                    {:else}
                        <HistoryMessageBlock
                            label="Head Block"
                            kind="head"
                            text={selectedEntryDetail?.request
                                ? selectedEntryDetail.request.headBlockStr
                                : ""}
                            searchQuery={requestHeadQuery}
                            emptyLabel={selectedEntryDetail?.request
                                ? "(empty)"
                                : "No request yet"}
                            bind:searchInput={blockSearchInputs.requestHead}
                            searchProgress={requestHeadSearchProgress}
                            hasContent={requestHeadHasContent}
                            navigateDisabled={requestHeadQuery.trim().length ===
                                0 || requestHeadMatchCount === 0}
                            previousLabel="Go to previous match in the request head block"
                            nextLabel="Go to next match in the request head block"
                            bind:containerElement={requestHeadContainer}
                            on:submit={() => applyBlockSearch("requestHead")}
                            on:previous={() =>
                                navigateSearchMatches("requestHead", -1)}
                            on:next={() =>
                                navigateSearchMatches("requestHead", 1)}
                        />

                        {#if selectedEntryDetail?.request?.truncatedBody}
                            <div class="truncatedBodyNotice">
                                This body was truncated because it is too large.
                            </div>
                        {/if}

                        <HistoryMessageBlock
                            label="Body"
                            kind="body"
                            headBlockStr={selectedEntryDetail?.request
                                ?.headBlockStr ?? ""}
                            bodyStr={selectedEntryDetail?.request?.bodyStr ??
                                ""}
                            searchQuery={requestBodyQuery}
                            emptyLabel={selectedEntryDetail?.request
                                ? ""
                                : "No request yet"}
                            bind:searchInput={blockSearchInputs.requestBody}
                            searchProgress={requestBodySearchProgress}
                            hasContent={requestBodyHasContent}
                            navigateDisabled={requestBodyQuery.trim().length ===
                                0 || requestBodyMatchCount === 0}
                            previousLabel="Go to previous match in the request body"
                            nextLabel="Go to next match in the request body"
                            bind:containerElement={requestBodyContainer}
                            bind:matchCount={requestBodyMatchCount}
                            on:submit={() => applyBlockSearch("requestBody")}
                            on:previous={() =>
                                navigateSearchMatches("requestBody", -1)}
                            on:next={() =>
                                navigateSearchMatches("requestBody", 1)}
                        />
                    {/if}
                </section>

                <section class="messageCard responseCard">
                    <div class="messageHeader">
                        <span>Response</span>
                        <strong title={getResponseLine(selectedEntry)}
                            >{getResponseLine(selectedEntry)}</strong
                        >
                    </div>

                    <div class="messageMeta">
                        <span>Host: {selectedEntry.response?.host || "-"}</span>
                        <span>Port: {selectedEntry.response?.port || "-"}</span>
                    </div>

                    {#if selectedEntryDetailState === "loading"}
                        <div class="detailSkeleton" aria-hidden="true">
                            <div class="detailSkeletonHeader">
                                <span class="detailSkeletonLine short"></span>
                                <span class="detailSkeletonLine medium"></span>
                            </div>
                            <div class="detailSkeletonBody">
                                <span class="detailSkeletonLine long"></span>
                                <span class="detailSkeletonLine medium"></span>
                                <span class="detailSkeletonLine medium"></span>
                                <span class="detailSkeletonLine short"></span>
                            </div>
                        </div>
                    {:else if selectedEntryDetailState === "waiting" && !selectedEntryDetail}
                        <div class="detailSpacer" aria-hidden="true"></div>
                    {:else if selectedEntryDetailState === "error"}
                        <div class="emptyState detailEmpty">
                            <div class="emptyTitle">
                                Could not load the response
                            </div>
                            <div class="emptySub">
                                {selectedEntryDetailError}
                            </div>
                        </div>
                    {:else}
                        <HistoryMessageBlock
                            label="Head Block"
                            kind="head"
                            text={selectedEntryDetail?.response
                                ? selectedEntryDetail.response.headBlockStr
                                : ""}
                            searchQuery={responseHeadQuery}
                            emptyLabel={selectedEntryDetail?.response
                                ? "(empty)"
                                : "No response yet"}
                            bind:searchInput={blockSearchInputs.responseHead}
                            searchProgress={responseHeadSearchProgress}
                            hasContent={responseHeadHasContent}
                            navigateDisabled={responseHeadQuery.trim()
                                .length === 0 || responseHeadMatchCount === 0}
                            previousLabel="Go to previous match in the response head block"
                            nextLabel="Go to next match in the response head block"
                            bind:containerElement={responseHeadContainer}
                            on:submit={() => applyBlockSearch("responseHead")}
                            on:previous={() =>
                                navigateSearchMatches("responseHead", -1)}
                            on:next={() =>
                                navigateSearchMatches("responseHead", 1)}
                        />

                        {#if selectedEntryDetail?.response?.truncatedBody}
                            <div class="truncatedBodyNotice">
                                This body was truncated because it is too large.
                            </div>
                        {/if}

                        <HistoryMessageBlock
                            label="Body"
                            kind="body"
                            headBlockStr={selectedEntryDetail?.response
                                ?.headBlockStr ?? ""}
                            bodyStr={selectedEntryDetail?.response?.bodyStr ??
                                ""}
                            searchQuery={responseBodyQuery}
                            emptyLabel={selectedEntryDetail?.response
                                ? ""
                                : "No response yet"}
                            bind:searchInput={blockSearchInputs.responseBody}
                            searchProgress={responseBodySearchProgress}
                            hasContent={responseBodyHasContent}
                            navigateDisabled={responseBodyQuery.trim()
                                .length === 0 || responseBodyMatchCount === 0}
                            previousLabel="Go to previous match in the response body"
                            nextLabel="Go to next match in the response body"
                            bind:containerElement={responseBodyContainer}
                            bind:matchCount={responseBodyMatchCount}
                            on:submit={() => applyBlockSearch("responseBody")}
                            on:previous={() =>
                                navigateSearchMatches("responseBody", -1)}
                            on:next={() =>
                                navigateSearchMatches("responseBody", 1)}
                        />
                    {/if}
                </section>
            </div>
        {:else}
            <div class="emptyState detailEmpty">
                <div class="emptyTitle">
                    {visibleEntries.length === 0
                        ? "No matches in this view"
                        : "No request selected"}
                </div>
                <div class="emptySub">
                    {#if visibleEntries.length === 0}
                        Edit the current filter or go back to the main tab to
                        review the full traffic.
                    {:else}
                        Details only expand below, keeping the top table clean
                        and compact as requested.
                    {/if}
                </div>
            </div>
        {/if}
    </div>
</div>

{#if contextMenuState}
    <div
        class="historyContextMenu"
        style={`left: ${contextMenuState.x}px; top: ${contextMenuState.y}px;`}
        role="menu"
        tabindex="-1"
        on:click|stopPropagation
        on:keydown|stopPropagation
    >
        <RequestCopySubmenu
            disabled={!contextMenuState.entry.request}
            on:select={copyContextRequest}
        />

        <button
            type="button"
            class="contextMenuItem"
            disabled={!contextMenuState.entry.request}
            on:click={sendContextEntryToRepeater}
        >
            Send to Repeater
        </button>

        <button
            type="button"
            class="contextMenuItem"
            disabled={!contextMenuState.entry.request}
            on:click={openSaveRequestDialog}
        >
            Save to Saved Requests
        </button>

        <button
            type="button"
            class="contextMenuItem contextMenuItemDanger"
            on:click={openRemoveEntryDialog}
        >
            Delete this request
        </button>

        <button
            type="button"
            class="contextMenuItem contextMenuItemDanger"
            on:click={openClearHistoryDialog}
        >
            Delete all requests
        </button>
    </div>
{/if}

{#if filterModalState}
    <HistoryFilterDialog
        title={filterModalState.title}
        submitLabel={filterModalState.submitLabel}
        initialConditions={filterModalState.conditions}
        initialOperator={filterModalState.operator}
        on:cancel={closeFilterModal}
        on:save={saveFilterView}
    />
{/if}

{#if saveRequestDialogState}
    <SaveRequestDialog
        title="Save temporary request"
        submitLabel="Save copy"
        on:cancel={closeSaveRequestDialog}
        on:save={saveContextEntryAsSnapshot}
    />
{/if}

{#if confirmDialogState}
    <div
        class="confirmOverlay"
        role="presentation"
        tabindex="-1"
        on:click={closeConfirmDialog}
        on:keydown={(event) => {
            if (event.key === "Escape") {
                closeConfirmDialog();
            }
        }}
    >
        <div
            class="confirmDialog"
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="history-confirm-title"
            aria-describedby="history-confirm-message"
            tabindex="-1"
            on:click|stopPropagation
            on:keydown|stopPropagation
        >
            <div class="confirmCopy">
                <span class="eyebrow">{confirmDialogState.eyebrow}</span>
                <h3 id="history-confirm-title">{confirmDialogState.title}</h3>
                <p id="history-confirm-message" class="confirmText">
                    {confirmDialogState.message}
                </p>
            </div>

            <div class="confirmActions">
                <button
                    type="button"
                    class="confirmButton secondary"
                    on:click={closeConfirmDialog}
                >
                    Cancel
                </button>
                <button
                    type="button"
                    class="confirmButton danger"
                    on:click={confirmHistoryAction}
                >
                    {confirmDialogState.confirmLabel}
                </button>
            </div>
        </div>
    </div>
{/if}

{#if alertDialogState}
    <AppAlertDialog
        eyebrow={alertDialogState.eyebrow}
        title={alertDialogState.title}
        message={alertDialogState.message}
        buttonLabel="Entendido"
        on:close={closeAlertDialog}
    />
{/if}

<style>
    h3 {
        margin: 0;
    }

    h3 {
        font-size: 22px;
        line-height: 1.1;
        letter-spacing: -0.03em;
        color: var(--text);
    }

    .historyView {
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    .historyContextMenu {
        position: fixed;
        z-index: 70;
        min-width: 200px;
        padding: 8px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--surface-strong);
        box-shadow: var(--shadow-card);
    }

    .contextMenuItem {
        appearance: none;
        width: 100%;
        min-height: 38px;
        padding: 0 12px;
        border: 1px solid transparent;
        border-radius: 8px;
        background: transparent;
        color: var(--text);
        text-align: left;
        font-size: 12px;
        font-weight: 700;
        cursor: pointer;
        transition:
            border-color 140ms ease,
            background 140ms ease;
    }

    .contextMenuItem:hover:not(:disabled) {
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.08);
    }

    .contextMenuItemDanger {
        color: #fca5a5;
    }

    .contextMenuItemDanger:hover:not(:disabled) {
        border-color: var(--danger-line);
        background: var(--danger-soft);
    }

    .contextMenuItem:disabled {
        cursor: not-allowed;
        opacity: 0.55;
    }

    .confirmOverlay {
        position: fixed;
        inset: 0;
        z-index: 80;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 20px;
        background: rgba(2, 6, 23, 0.56);
        backdrop-filter: blur(2px);
    }

    .confirmDialog {
        width: min(100%, 420px);
        display: grid;
        gap: 16px;
        padding: 18px;
        border-radius: 16px;
        border: 1px solid var(--line);
        background: var(--surface-strong);
        box-shadow: var(--shadow-card);
    }

    .confirmCopy {
        display: grid;
        gap: 8px;
    }

    .confirmText {
        margin: 0;
        color: var(--muted);
        font-size: 13px;
        line-height: 1.5;
    }

    .confirmActions {
        display: flex;
        justify-content: flex-end;
        gap: 10px;
    }

    .confirmButton {
        appearance: none;
        min-height: 38px;
        padding: 0 14px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--text);
        font-size: 12px;
        font-weight: 800;
        cursor: pointer;
        transition:
            border-color 140ms ease,
            background 140ms ease,
            color 140ms ease;
    }

    .confirmButton.secondary:hover {
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.08);
    }

    .confirmButton.danger {
        border-color: var(--danger-line);
        background: var(--danger-soft);
        color: #fecaca;
    }

    .confirmButton.danger:hover {
        background: rgba(248, 113, 113, 0.2);
    }

    .historyTabsBar {
        display: flex;
        justify-content: space-between;
        gap: 12px;
        align-items: center;
        margin-bottom: 14px;
        padding-bottom: 14px;
        border-bottom: 1px solid var(--line);
    }

    .historyTabs {
        display: flex;
        gap: 8px;
        flex: 1 1 auto;
        min-width: 0;
        overflow-x: auto;
        padding-bottom: 2px;
    }

    .historyTabItem {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        flex: 0 0 auto;
        min-width: 0;
        padding: 0 8px 0 12px;
        border: 1px solid var(--line);
        border-radius: 10px;
        background: var(--surface-muted);
        color: var(--muted-strong);
        transition:
            border-color 140ms ease,
            background 140ms ease,
            color 140ms ease,
            transform 140ms ease;
    }

    .historyTab,
    .filterActionButton {
        appearance: none;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--muted-strong);
        transition:
            border-color 140ms ease,
            background 140ms ease,
            color 140ms ease,
            transform 140ms ease;
    }

    .historyTab {
        display: inline-flex;
        align-items: center;
        gap: 8px;
        min-height: 38px;
        padding: 0;
        border: none;
        background: transparent;
        color: inherit;
        cursor: pointer;
        white-space: nowrap;
    }

    .historyTabItem:hover,
    .filterActionButton:hover {
        transform: translateY(-1px);
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.08);
    }

    .historyTabItem.active {
        border-color: rgba(var(--accent-rgb), 0.72);
        background: rgba(var(--accent-rgb), 0.12);
        color: var(--text);
    }

    .historyTabLabel {
        font-size: 12px;
        font-weight: 800;
        letter-spacing: 0.04em;
    }

    .historyTabMeta {
        padding: 4px 8px;
        border-radius: 999px;
        background: rgba(148, 163, 184, 0.1);
        color: var(--muted);
        font-size: 10px;
        font-weight: 800;
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .closeHistoryTabButton {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 22px;
        height: 22px;
        padding: 0;
        border: none;
        border-radius: 999px;
        background: transparent;
        color: var(--muted);
        cursor: pointer;
        font-size: 16px;
        font-weight: 700;
        line-height: 1;
        transition:
            background 140ms ease,
            color 140ms ease,
            transform 140ms ease;
    }

    .closeHistoryTabButton:hover {
        transform: none;
        background: rgba(148, 163, 184, 0.12);
        color: var(--text);
    }

    .historyTabItem.active .closeHistoryTabButton {
        color: var(--text);
    }

    .filterActionButton {
        flex: 0 0 auto;
        min-height: 40px;
        padding: 0 16px;
        border-radius: 10px;
        font-size: 11px;
        font-weight: 800;
        letter-spacing: 0.1em;
        text-transform: uppercase;
        cursor: pointer;
    }

    .overviewCard,
    .listCard,
    .detailCard {
        border-radius: 16px;
        border: 1px solid var(--line);
        background: var(--surface-strong);
        box-shadow: var(--shadow-card);
    }

    .overviewCard {
        padding: 18px;
        display: flex;
        justify-content: space-between;
        gap: 14px;
        align-items: flex-start;
    }

    .overviewCopy {
        display: flex;
        flex-direction: column;
        gap: 10px;
        max-width: 760px;
    }

    .eyebrow {
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.12em;
        text-transform: uppercase;
        color: var(--muted);
    }

    .headerCopy {
        display: flex;
        flex-direction: column;
        gap: 4px;
        min-width: 0;
    }

    .headerSub {
        margin: 0;
        color: var(--muted);
        font-size: 12px;
        line-height: 1.45;
    }

    .overviewStats {
        display: flex;
        flex-wrap: wrap;
        justify-content: flex-end;
        gap: 8px;
        max-width: 100%;
    }

    .statChip {
        min-width: 88px;
        padding: 10px 12px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        display: inline-flex;
        align-items: baseline;
        gap: 8px;
    }

    .statChip span {
        font-size: 11px;
        font-weight: 900;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        color: var(--muted);
    }

    .statChip strong {
        font-size: 18px;
        line-height: 1;
        color: var(--text);
    }

    .listCard,
    .detailCard {
        padding: 16px;
    }

    .cardHeader {
        display: flex;
        justify-content: space-between;
        gap: 14px;
        align-items: flex-start;
        margin-bottom: 12px;
    }

    .headerHint {
        max-width: 420px;
        color: var(--muted);
        font-size: 12px;
        line-height: 1.45;
        text-align: right;
    }

    .emptyState {
        border-radius: 12px;
        border: 1px dashed var(--line-strong);
        background: var(--surface-muted);
        padding: 24px 20px;
    }

    .emptyTitle {
        color: var(--text);
        font-size: 16px;
        font-weight: 900;
    }

    .emptySub {
        margin-top: 8px;
        color: var(--muted);
        line-height: 1.5;
    }

    .tableViewport {
        overflow: auto;
        max-height: 470px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--field-bg);
    }

    .historyTable {
        min-width: 880px;
    }

    .tableHead,
    .tableRow {
        display: grid;
        grid-template-columns: var(--history-columns);
        align-items: center;
        column-gap: 8px;
    }

    .tableHead {
        position: sticky;
        top: 0;
        z-index: 2;
        padding: 9px 12px;
        background: rgba(15, 23, 42, 0.98);
        border-bottom: 1px solid var(--line);
    }

    .headColumn {
        position: relative;
        min-width: 0;
    }

    .headCell {
        appearance: none;
        width: 100%;
        display: inline-flex;
        align-items: center;
        justify-content: flex-start;
        gap: 8px;
        padding: 0 16px 0 0;
        border: none;
        background: transparent;
        color: var(--muted-strong);
        font-size: 11px;
        font-weight: 900;
        letter-spacing: 0.12em;
        text-transform: uppercase;
        cursor: pointer;
    }

    .headCell.sorted {
        color: var(--text);
    }

    .sortIndicator {
        display: none;
        width: 0;
        height: 0;
        flex: 0 0 auto;
        overflow: hidden;
        border-left: 5px solid transparent;
        border-right: 5px solid transparent;
        font-size: 0;
        line-height: 0;
    }

    .sortIndicator.active {
        display: inline-block;
    }

    .sortIndicator.asc {
        border-bottom: 7px solid var(--accent);
        transform: translateY(-1px);
    }

    .sortIndicator.desc {
        border-top: 7px solid var(--accent);
        transform: translateY(1px);
    }

    .resizeHandle {
        position: absolute;
        top: -12px;
        right: -7px;
        width: 14px;
        height: calc(100% + 24px);
        border: none;
        padding: 0;
        background: transparent;
        cursor: col-resize;
    }

    .resizeHandle::before {
        content: "";
        position: absolute;
        top: 14px;
        bottom: 14px;
        left: 50%;
        width: 2px;
        transform: translateX(-50%);
        border-radius: 999px;
        background: rgba(148, 163, 184, 0.22);
        transition: background 140ms ease;
    }

    .resizeHandle:hover::before,
    .resizeHandle:focus-visible::before {
        background: var(--accent);
    }

    .tableBody {
        position: relative;
        min-height: 0;
    }

    .virtualRows {
        position: absolute;
        inset: 0 0 auto 0;
    }

    .tableRow {
        appearance: none;
        width: 100%;
        box-sizing: border-box;
        padding: 0 12px;
        border: none;
        border-bottom: 1px solid rgba(148, 163, 184, 0.08);
        background: transparent;
        color: inherit;
        text-align: left;
        cursor: pointer;
        transition:
            background 140ms ease,
            transform 140ms ease,
            box-shadow 140ms ease;
    }

    .tableRow:hover {
        background: var(--surface-muted);
    }

    .tableRow.selected {
        background: rgba(var(--accent-rgb), 0.14);
        box-shadow:
            inset 4px 0 0 var(--accent),
            inset 0 0 0 1px rgba(var(--accent-rgb), 0.3);
    }

    .cell {
        display: flex;
        align-items: center;
        min-width: 0;
        height: 100%;
        padding: 0 10px 0 0;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        color: var(--text);
        font-size: 12px;
        line-height: 1.25;
    }

    .mono {
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
    }

    .strong {
        font-weight: 900;
    }

    .methodCell {
        color: var(--info);
        font-weight: 700;
        letter-spacing: 0.03em;
    }

    .statusCell {
        overflow: visible;
    }

    .statusBadge,
    .metaPill {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 64px;
        padding: 4px 8px;
        border-radius: 999px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--text);
        font-size: 10px;
        font-weight: 700;
    }

    .statusBadge.pending,
    .metaPill.pending {
        background: var(--info-soft);
        border-color: var(--info-line);
        color: var(--info);
    }

    .statusBadge.success,
    .metaPill.success {
        background: var(--success-soft);
        border-color: var(--success-line);
        color: var(--success);
    }

    .statusBadge.redirect,
    .metaPill.redirect {
        background: var(--info-soft);
        border-color: var(--info-line);
        color: var(--info);
    }

    .statusBadge.warning,
    .metaPill.warning {
        background: var(--warning-soft);
        border-color: var(--warning-line);
        color: var(--warning);
    }

    .statusBadge.danger,
    .metaPill.danger {
        background: var(--danger-soft);
        border-color: var(--danger-line);
        color: var(--danger);
    }

    .statusBadge.neutral,
    .metaPill.neutral {
        background: rgba(148, 163, 184, 0.12);
        border-color: var(--neutral-line);
        color: var(--muted-strong);
    }

    .detailTop {
        align-items: flex-start;
    }

    .detailHeaderTools {
        display: flex;
        flex-direction: column;
        align-items: flex-end;
        gap: 10px;
        min-width: min(100%, 460px);
    }

    .detailBadges {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
        justify-content: flex-end;
    }

    .detailIntro {
        display: grid;
        gap: 10px;
        margin-bottom: 14px;
        padding: 16px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
    }

    .introLine {
        display: flex;
        gap: 12px;
        align-items: flex-start;
    }

    .detailGrid {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 14px;
    }

    .messageCard {
        min-width: 0;
        padding: 16px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.02);
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    .responseCard {
        border-color: var(--line);
    }

    .messageHeader {
        display: grid;
        gap: 12px;
        align-items: start;
        padding-bottom: 12px;
        border-bottom: 1px solid var(--line);
    }

    .messageHeader span {
        color: var(--muted);
        font-size: 11px;
        font-weight: 900;
        letter-spacing: 0.12em;
        text-transform: uppercase;
    }

    .messageHeader strong {
        display: block;
        max-width: 100%;
        min-width: 0;
        color: var(--text);
        text-align: left;
        font-size: 12px;
        line-height: 1.45;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .messageMeta {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
    }

    .requestMetaPanel {
        display: grid;
        gap: 10px;
    }

    .messageMeta span {
        padding: 6px 10px;
        border-radius: 999px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--muted-strong);
        font-size: 11px;
        font-weight: 700;
    }

    .detailEmpty {
        min-height: 200px;
        display: flex;
        flex-direction: column;
        justify-content: center;
    }

    .detailSpacer {
        min-height: 200px;
    }

    .truncatedBodyNotice {
        padding: 10px 12px;
        border-radius: 10px;
        border: 1px solid var(--warning-line);
        background: var(--warning-soft);
        color: var(--warning);
        font-size: 12px;
        line-height: 1.45;
        font-weight: 700;
    }

    .detailSkeleton {
        display: grid;
        gap: 14px;
        min-height: 200px;
        padding: 18px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: linear-gradient(
            90deg,
            rgba(148, 163, 184, 0.04) 0%,
            rgba(148, 163, 184, 0.12) 50%,
            rgba(148, 163, 184, 0.04) 100%
        );
        background-size: 220% 100%;
        animation: detailSkeletonPulse 1.1s ease-in-out infinite;
    }

    .detailSkeletonHeader,
    .detailSkeletonBody {
        display: grid;
        gap: 10px;
    }

    .detailSkeletonLine {
        display: block;
        height: 11px;
        border-radius: 999px;
        background: rgba(226, 232, 240, 0.16);
    }

    .detailSkeletonLine.short {
        width: 30%;
    }

    .detailSkeletonLine.medium {
        width: 58%;
    }

    .detailSkeletonLine.long {
        width: 100%;
    }

    @keyframes detailSkeletonPulse {
        0% {
            background-position: 100% 0;
        }

        100% {
            background-position: -100% 0;
        }
    }

    @media (max-width: 1080px) {
        .historyTabsBar,
        .overviewCard,
        .cardHeader,
        .detailTop {
            flex-direction: column;
            align-items: stretch;
        }

        .detailGrid {
            grid-template-columns: 1fr;
        }

        .overviewStats,
        .headerHint,
        .detailBadges {
            text-align: left;
            justify-content: flex-start;
        }

        .detailHeaderTools {
            align-items: stretch;
        }
    }

    @media (max-width: 760px) {
        .overviewCard,
        .listCard,
        .detailCard {
            border-radius: 14px;
        }

        .historyTab {
            min-height: 38px;
            padding: 0 12px;
        }

        .tableViewport {
            max-height: none;
        }
    }
</style>
