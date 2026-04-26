<script lang="ts">
    import { onDestroy, tick } from "svelte";
    import { setActiveWorkspaceTab } from "@/app/state/workspaceTabStore";
    import type { SortDirection } from "@/features/http-history/state/httpHistoryStore";
    import AppAlertDialog from "@/shared/components/AppAlertDialog.svelte";
    import RequestCopySubmenu from "@/shared/components/RequestCopySubmenu.svelte";
    import HistoryMessageBlock from "@/features/http-history/components/HistoryMessageBlock.svelte";
    import HistorySearchForm from "@/features/http-history/components/HistorySearchForm.svelte";
    import RequestPathSummary from "@/features/http-history/components/RequestPathSummary.svelte";
    import { openRepeaterTabFromRequestSnapshot } from "@/features/repeater/state/repeaterStore";
    import {
        buildRequestUrl,
        buildColumnTemplate,
        buildColumnWidthMap,
        buildRequestLine,
        buildResponseLine,
        compareHttpEntriesByColumn,
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
    import { copyTextToClipboard } from "@/shared/utils/clipboard";
    import {
        buildRequestCopyText,
        type RequestCopyTarget,
    } from "@/shared/utils/requestCopy";
    import {
        calculateVirtualizedRange,
        DEFAULT_TABLE_OVERSCAN,
    } from "@/shared/utils/virtualizedTable";
    import {
        getAdjacentVirtualizedSelection,
        scrollVirtualizedRowIntoView,
    } from "@/shared/utils/virtualizedSelection";
    import {
        getRememberedScrollPosition,
        rememberScrollPosition,
    } from "@/shared/utils/viewScrollMemory";
    import { countMatches } from "@/shared/utils/textSearch";
    import {
        clearSavedRequests,
        removeSavedRequest,
        savedRequestsStore,
        selectSavedRequest,
        type SavedRequestEntry,
    } from "./state/savedRequestsStore";
    import {
        savedRequestsViewStore,
        toggleSavedRequestsSort,
        type SavedRequestsSortKey,
    } from "./state/savedRequestsViewStore";

    type ColumnConfig = TableColumnConfig<SavedRequestsSortKey>;
    type ResizeState = {
        key: SavedRequestsSortKey;
        startX: number;
        startWidth: number;
    };
    type SavedRequestsContextMenuState = {
        entry: SavedRequestEntry;
        x: number;
        y: number;
    };
    type BlockSearchId =
        | "requestHead"
        | "requestBody"
        | "responseHead"
        | "responseBody";
    type AlertDialogState = {
        eyebrow: string;
        title: string;
        message: string;
    };
    type SearchScopeId = "global" | BlockSearchId;
    type BlockSearchState = Record<BlockSearchId, string>;
    type SearchNavigationState = Record<SearchScopeId, number>;

    const allColumns: ColumnConfig[] = [
        { key: "name", label: "Name", width: 180, minWidth: 140 },
        ...httpEntryColumns,
    ];
    const SAVED_REQUESTS_ROW_HEIGHT = 40;
    const SAVED_REQUESTS_OVERSCAN = DEFAULT_TABLE_OVERSCAN;
    const savedRequestsListScrollKey = "saved-requests:list";
    const initialBlockSearchState: BlockSearchState = {
        requestHead: "",
        requestBody: "",
        responseHead: "",
        responseBody: "",
    };

    let tableViewport: HTMLDivElement | null = null;
    let tableHeadElement: HTMLDivElement | null = null;
    let savedRequestsViewportObserver: ResizeObserver | null = null;
    let savedRequestsHeadObserver: ResizeObserver | null = null;
    let observedSavedRequestsViewport: HTMLDivElement | null = null;
    let observedSavedRequestsHead: HTMLDivElement | null = null;
    let columnWidths: Record<SavedRequestsSortKey, number> =
        buildColumnWidthMap(allColumns);
    let activeResize: ResizeState | null = null;
    let savedRequestsScrollTop = 0;
    let savedRequestsViewportHeight = 0;
    let savedRequestsTableHeadHeight = 0;
    let contextMenuState: SavedRequestsContextMenuState | null = null;
    let restoredSavedRequestsScroll = false;
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
    let alertDialogState: AlertDialogState | null = null;

    $: savedRequestsState = $savedRequestsStore;
    $: savedRequestsViewState = $savedRequestsViewStore;
    $: entries = savedRequestsState.entries;
    $: sortedEntries = getSortedSavedRequests(
        entries,
        savedRequestsViewState.sortKey,
        savedRequestsViewState.sortDirection,
    );
    $: selectedEntry =
        sortedEntries.find(
            (entry) => entry.id === savedRequestsState.selectedId,
        ) ??
        sortedEntries[0] ??
        null;
    $: completedCount = entries.filter(
        (entry) => entry.response !== null,
    ).length;
    $: pendingCount = entries.filter((entry) => entry.response === null).length;
    $: tableColumns = buildColumnTemplate(allColumns, columnWidths);
    $: visibleEntryRange = calculateVirtualizedRange({
        itemCount: sortedEntries.length,
        scrollTop: savedRequestsScrollTop,
        viewportHeight: savedRequestsViewportHeight,
        rowHeight: SAVED_REQUESTS_ROW_HEIGHT,
        overscan: SAVED_REQUESTS_OVERSCAN,
        stickyOffset: savedRequestsTableHeadHeight,
    });
    $: virtualSortedEntries = sortedEntries.slice(
        visibleEntryRange.startIndex,
        visibleEntryRange.endIndex,
    );
    $: if (tableViewport && !restoredSavedRequestsScroll) {
        restoredSavedRequestsScroll = true;

        tick().then(() => {
            if (!tableViewport) return;
            tableViewport.scrollTop = getRememberedScrollPosition(
                savedRequestsListScrollKey,
            );
            savedRequestsScrollTop = tableViewport.scrollTop;
            updateSavedRequestsViewportMetrics();
        });
    }
    $: if (tableViewport) {
        updateSavedRequestsViewportMetrics();
    }
    $: if (tableHeadElement) {
        updateSavedRequestsViewportMetrics();
    }
    $: if (tableViewport !== observedSavedRequestsViewport) {
        savedRequestsViewportObserver?.disconnect();
        observedSavedRequestsViewport = tableViewport;

        if (tableViewport && typeof ResizeObserver !== "undefined") {
            savedRequestsViewportObserver = new ResizeObserver(() => {
                updateSavedRequestsViewportMetrics();
            });
            savedRequestsViewportObserver.observe(tableViewport);
        }

        tick().then(() => {
            updateSavedRequestsViewportMetrics();
        });
    }
    $: if (tableHeadElement !== observedSavedRequestsHead) {
        savedRequestsHeadObserver?.disconnect();
        observedSavedRequestsHead = tableHeadElement;

        if (tableHeadElement && typeof ResizeObserver !== "undefined") {
            savedRequestsHeadObserver = new ResizeObserver(() => {
                updateSavedRequestsViewportMetrics();
            });
            savedRequestsHeadObserver.observe(tableHeadElement);
        }

        tick().then(() => {
            updateSavedRequestsViewportMetrics();
        });
    }
    $: requestHeadText = selectedEntry?.request.headBlockStr ?? "";
    $: requestBodyText = selectedEntry?.request.bodyStr ?? "";
    $: responseHeadText = selectedEntry?.response?.headBlockStr ?? "";
    $: responseBodyText = selectedEntry?.response?.bodyStr ?? "";
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

    function getDisplayName(entry: SavedRequestEntry): string {
        return entry.name.trim().length > 0 ? entry.name.trim() : "Untitled";
    }

    function compareText(left: string, right: string): number {
        return left.localeCompare(right, undefined, {
            numeric: true,
            sensitivity: "base",
        });
    }

    function compareNumber(left: number, right: number): number {
        if (left === right) return 0;
        return left > right ? 1 : -1;
    }

    function getSavedRequestTimeValue(entry: SavedRequestEntry): number {
        return entry.savedAtMs;
    }

    function compareSavedRequests(
        left: SavedRequestEntry,
        right: SavedRequestEntry,
        activeSortKey: SavedRequestsSortKey,
        activeSortDirection: SortDirection,
    ): number {
        if (activeSortKey === "name") {
            const comparison = compareText(
                getDisplayName(left),
                getDisplayName(right),
            );

            if (comparison !== 0) {
                return comparison * (activeSortDirection === "asc" ? 1 : -1);
            }

            return compareNumber(left.sequence, right.sequence);
        }

        return compareHttpEntriesByColumn(
            left,
            right,
            activeSortKey,
            activeSortDirection,
            getSavedRequestTimeValue,
            (entry) => entry.sequence,
        );
    }

    function getSortedSavedRequests(
        currentEntries: SavedRequestEntry[],
        activeSortKey: SavedRequestsSortKey,
        activeSortDirection: SortDirection,
    ): SavedRequestEntry[] {
        if (activeSortKey === "name") {
            return [...currentEntries].sort((left, right) =>
                compareSavedRequests(
                    left,
                    right,
                    activeSortKey,
                    activeSortDirection,
                ),
            );
        }

        return sortHttpEntriesByColumn(
            currentEntries,
            activeSortKey,
            activeSortDirection,
            getSavedRequestTimeValue,
            (entry) => entry.sequence,
        );
    }

    function toggleSort(key: SavedRequestsSortKey) {
        toggleSavedRequestsSort(key);
    }

    function getColumnConfig(key: SavedRequestsSortKey): ColumnConfig {
        return allColumns.find((column) => column.key === key) ?? allColumns[0];
    }

    function startColumnResize(event: MouseEvent, key: SavedRequestsSortKey) {
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
        savedRequestsViewportObserver?.disconnect();
        savedRequestsHeadObserver?.disconnect();
        if (tableViewport) {
            rememberScrollPosition(
                savedRequestsListScrollKey,
                tableViewport.scrollTop,
            );
        }
        stopColumnResize();
    });

    function handleSavedRequestsListScroll() {
        if (!tableViewport) return;

        savedRequestsScrollTop = tableViewport.scrollTop;
        updateSavedRequestsViewportMetrics();
        rememberScrollPosition(
            savedRequestsListScrollKey,
            tableViewport.scrollTop,
        );
    }

    function updateSavedRequestsViewportMetrics() {
        savedRequestsViewportHeight = tableViewport?.clientHeight ?? 0;
        savedRequestsTableHeadHeight = tableHeadElement?.offsetHeight ?? 0;
    }

    async function focusSavedRequestRow(entryId: string) {
        await tick();

        const rowElement =
            tableViewport?.querySelector<HTMLButtonElement>(
                `[data-entry-id="${entryId}"]`,
            ) ?? null;

        rowElement?.focus();
    }

    async function moveSavedRequestSelection(direction: -1 | 1) {
        const nextSelection = getAdjacentVirtualizedSelection({
            items: sortedEntries,
            selectedId: selectedEntry?.id ?? null,
            getId: (entry) => entry.id,
            direction,
        });
        if (!nextSelection) return;

        scrollVirtualizedRowIntoView({
            viewport: tableViewport,
            rowIndex: nextSelection.index,
            rowHeight: SAVED_REQUESTS_ROW_HEIGHT,
            stickyOffset: savedRequestsTableHeadHeight,
        });
        handleSavedRequestsListScroll();
        selectSavedRequest(nextSelection.id);
        await focusSavedRequestRow(nextSelection.id);
    }

    function handleSavedRequestsTableKeydown(event: KeyboardEvent) {
        const target =
            event.target instanceof HTMLElement ? event.target : null;
        if (!target?.closest(".tableRow")) return;
        if (event.altKey || event.ctrlKey || event.metaKey) return;
        if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return;

        event.preventDefault();
        void moveSavedRequestSelection(event.key === "ArrowDown" ? 1 : -1);
    }

    function hasSearchableContent(value: string | null | undefined): boolean {
        return (value ?? "").trim().length > 0;
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
            activeSearchElement.blur();
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

    function clearBlockSearches() {
        if (activeSearchScope !== "global") {
            clearActiveSearchElement();
            activeSearchScope = null;
        }

        blockSearchInputs = { ...initialBlockSearchState };
        appliedBlockSearches = { ...initialBlockSearchState };
        navigationState = {
            ...navigationState,
            requestHead: -1,
            requestBody: -1,
            responseHead: -1,
            responseBody: -1,
        };
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

    function openSavedRequestContextMenu(
        event: MouseEvent,
        entry: SavedRequestEntry,
    ) {
        event.preventDefault();
        selectSavedRequest(entry.id);
        contextMenuState = {
            entry,
            x: Math.min(event.clientX, window.innerWidth - 220),
            y: Math.min(event.clientY, window.innerHeight - 180),
        };
    }

    function closeContextMenu() {
        contextMenuState = null;
    }

    function openAlertDialog(
        title: string,
        message: string,
        eyebrow = "Saved Requests",
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

    function sendContextEntryToRepeater() {
        if (!contextMenuState) return;

        openRepeaterTabFromRequestSnapshot(
            contextMenuState.entry.request,
            contextMenuState.entry.id,
        );
        setActiveWorkspaceTab("repeater");
        closeContextMenu();
    }

    async function copyContextRequest(
        event: CustomEvent<{ target: RequestCopyTarget }>,
    ) {
        const request = contextMenuState?.entry.request;
        if (!request) return;

        const copyTarget = event.detail.target;
        const requiresUrl =
            copyTarget !== "js-headers" && copyTarget !== "python-headers";
        const requestUrl = buildRequestUrl(request);

        if (requiresUrl && !requestUrl) {
            openAlertDialog(
                "URL unavailable",
                "This saved request does not have enough scheme, host, or path information to build a URL or request snippet.",
            );
            closeContextMenu();
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
            );
        }

        closeContextMenu();
    }

    function removeContextEntry() {
        if (!contextMenuState) return;

        removeSavedRequest(contextMenuState.entry.id);
        closeContextMenu();
    }

    function clearAllFromContextMenu() {
        clearSavedRequests();
        closeContextMenu();
    }
</script>

<svelte:window
    on:click={closeContextMenu}
    on:resize={closeContextMenu}
    on:resize={updateSavedRequestsViewportMetrics}
    on:scroll={closeContextMenu}
/>

<div class="savedRequestsView">
    <div class="overviewCard">
        <div class="overviewCopy">
            <span class="eyebrow">Table View</span>
            <div class="headerCopy">
                <h3>Saved Requests</h3>
                <p class="headerSub">
                    Frozen snapshots of manually selected traffic. They do not
                    depend on the live history and disappear when the
                    application restarts.
                </p>
            </div>
        </div>

        <div class="overviewTools">
            <div class="overviewStats" aria-label="Saved requests summary">
                <div class="statChip">
                    <span>Total</span>
                    <strong>{entries.length}</strong>
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

            <button
                type="button"
                class="clearButton"
                disabled={entries.length === 0}
                on:click={clearSavedRequests}
            >
                Clear all
            </button>
        </div>
    </div>

    <div class="listCard">
        <div class="cardHeader">
            <div class="headerCopy">
                <span class="eyebrow">Shelf</span>
                <h3>Saved list</h3>
                <p class="headerSub">
                    Same table system as history: sort, resize columns, and
                    right-click a row.
                </p>
            </div>
        </div>

        {#if entries.length > 0}
            <div
                class="tableViewport"
                bind:this={tableViewport}
                on:keydown={handleSavedRequestsTableKeydown}
                on:scroll={handleSavedRequestsListScroll}
            >
                <div
                    class="savedTable"
                    style={`--saved-requests-columns: ${tableColumns}; --saved-requests-row-height: ${SAVED_REQUESTS_ROW_HEIGHT}px;`}
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
                                    class:sorted={savedRequestsViewState.sortKey ===
                                        column.key}
                                    on:click={() => toggleSort(column.key)}
                                >
                                    <span>{column.label}</span>
                                    <span
                                        class="sortIndicator"
                                        class:active={savedRequestsViewState.sortKey ===
                                            column.key}
                                        class:asc={savedRequestsViewState.sortKey ===
                                            column.key &&
                                            savedRequestsViewState.sortDirection ===
                                                "asc"}
                                        class:desc={savedRequestsViewState.sortKey ===
                                            column.key &&
                                            savedRequestsViewState.sortDirection ===
                                                "desc"}
                                        aria-hidden="true"
                                    ></span>
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
                            {#each virtualSortedEntries as entry (entry.id)}
                                <button
                                    type="button"
                                    class="tableRow"
                                    class:selected={entry.id ===
                                        selectedEntry?.id}
                                    data-entry-id={entry.id}
                                    tabindex={entry.id === selectedEntry?.id
                                        ? 0
                                        : -1}
                                    style={`height: ${SAVED_REQUESTS_ROW_HEIGHT}px;`}
                                    on:click={() =>
                                        selectSavedRequest(entry.id)}
                                    on:contextmenu={(event) =>
                                        openSavedRequestContextMenu(
                                            event,
                                            entry,
                                        )}
                                >
                                    {#each allColumns as column (column.key)}
                                        {#if column.key === "name"}
                                            <div
                                                class="cell strong"
                                                title={getDisplayName(entry)}
                                            >
                                                {getDisplayName(entry)}
                                            </div>
                                        {:else if column.key === "time"}
                                            <div class="cell mono">
                                                {entry.savedTimeLabel}
                                            </div>
                                        {:else if column.key === "host"}
                                            <div
                                                class="cell strong"
                                                title={getEntryHost(entry)}
                                            >
                                                {getEntryHost(entry)}
                                            </div>
                                        {:else if column.key === "path"}
                                            <div
                                                class="cell mono"
                                                title={getEntryPath(entry)}
                                            >
                                                {getEntryPath(entry)}
                                            </div>
                                        {:else if column.key === "port"}
                                            <div class="cell mono">
                                                {getEntryPort(entry)}
                                            </div>
                                        {:else if column.key === "method"}
                                            <div class="cell methodCell">
                                                {getEntryMethod(entry)}
                                            </div>
                                        {:else if column.key === "version"}
                                            <div class="cell mono">
                                                {getEntryVersion(entry)}
                                            </div>
                                        {:else}
                                            <div class="cell statusCell">
                                                <span
                                                    class={`statusBadge ${getEntryStatusTone(
                                                        entry,
                                                    )}`}
                                                >
                                                    {getEntryStatusLabel(entry)}
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
        {:else}
            <div class="emptyState">
                <div class="emptyTitle">No saved requests</div>
                <div class="emptySub">
                    Save one from <strong>HTTP History</strong> with a right-click
                    and it will appear here as a frozen copy.
                </div>
            </div>
        {/if}
    </div>

    {#if selectedEntry}
        <div class="detailCard">
            <div class="cardHeader detailTop">
                <div class="headerCopy">
                    <span class="eyebrow">Detail</span>
                    <h3>{getDisplayName(selectedEntry)}</h3>
                    <p class="headerSub">
                        Snapshot saved at {selectedEntry.savedTimeLabel}.
                    </p>
                </div>

                <div class="detailHeaderTools">
                    <HistorySearchForm
                        bind:value={globalSearchInput}
                        placeholder="Search within request and response"
                        progress={globalSearchProgress}
                        submitDisabled={false}
                        navigateDisabled={globalMatchCount <= 0}
                        on:submit={applyGlobalSearch}
                        on:previous={() => navigateSearchMatches("global", -1)}
                        on:next={() => navigateSearchMatches("global", 1)}
                    />

                    <div class="detailBadges">
                        <span class="metaPill"
                            >{selectedEntry.savedTimeLabel}</span
                        >
                        <span class="metaPill"
                            >{getEntryHost(selectedEntry)}</span
                        >
                        <span class="metaPill"
                            >:{selectedEntry.request.port || "-"}</span
                        >
                        <span class="metaPill"
                            >{getEntryMethod(selectedEntry)}</span
                        >
                        <span
                            class={`metaPill ${getEntryStatusTone(selectedEntry)}`}
                        >
                            {getEntryStatusLabel(selectedEntry)}
                        </span>
                    </div>
                </div>
            </div>

            <div class="detailGrid">
                <section class="messageCard">
                    <div class="messageHeader">
                        <span>Request</span>
                        <strong title={buildRequestLine(selectedEntry.request)}
                            >{buildRequestLine(selectedEntry.request)}</strong
                        >
                    </div>

                    <div class="requestMetaPanel">
                        <div class="messageMeta">
                            <span
                                >Host: {selectedEntry.request.host || "-"}</span
                            >
                            <span
                                >Port: {selectedEntry.request.port || "-"}</span
                            >
                        </div>

                        <RequestPathSummary
                            path={selectedEntry.request.path || "-"}
                        />
                    </div>

                    <HistoryMessageBlock
                        label="Head Block"
                        kind="head"
                        text={selectedEntry.request.headBlockStr}
                        emptyLabel="(empty)"
                        searchQuery={requestHeadQuery}
                        bind:searchInput={blockSearchInputs.requestHead}
                        searchProgress={requestHeadSearchProgress}
                        hasContent={requestHeadHasContent}
                        navigateDisabled={requestHeadMatchCount <= 0}
                        bind:containerElement={requestHeadContainer}
                        on:submit={() => applyBlockSearch("requestHead")}
                        on:previous={() =>
                            navigateSearchMatches("requestHead", -1)}
                        on:next={() => navigateSearchMatches("requestHead", 1)}
                    />

                    <div class="bodyBlock">
                        <HistoryMessageBlock
                            label="Body"
                            kind="body"
                            headBlockStr={selectedEntry.request.headBlockStr}
                            bodyStr={selectedEntry.request.bodyStr}
                            emptyLabel=""
                            searchQuery={requestBodyQuery}
                            bind:searchInput={blockSearchInputs.requestBody}
                            searchProgress={requestBodySearchProgress}
                            hasContent={requestBodyHasContent}
                            navigateDisabled={requestBodyMatchCount <= 0}
                            bind:containerElement={requestBodyContainer}
                            bind:matchCount={requestBodyMatchCount}
                            on:submit={() => applyBlockSearch("requestBody")}
                            on:previous={() =>
                                navigateSearchMatches("requestBody", -1)}
                            on:next={() =>
                                navigateSearchMatches("requestBody", 1)}
                        />
                    </div>
                </section>

                <section class="messageCard responseCard">
                    <div class="messageHeader">
                        <span>Response</span>
                        <strong
                            title={buildResponseLine(selectedEntry.response)}
                            >{buildResponseLine(selectedEntry.response)}</strong
                        >
                    </div>

                    <div class="messageMeta">
                        <span>Host: {selectedEntry.response?.host || "-"}</span>
                        <span>Port: {selectedEntry.response?.port || "-"}</span>
                    </div>

                    <HistoryMessageBlock
                        label="Head Block"
                        kind="head"
                        text={selectedEntry.response?.headBlockStr ?? ""}
                        emptyLabel="No saved response"
                        searchQuery={responseHeadQuery}
                        bind:searchInput={blockSearchInputs.responseHead}
                        searchProgress={responseHeadSearchProgress}
                        hasContent={responseHeadHasContent}
                        navigateDisabled={responseHeadMatchCount <= 0}
                        bind:containerElement={responseHeadContainer}
                        on:submit={() => applyBlockSearch("responseHead")}
                        on:previous={() =>
                            navigateSearchMatches("responseHead", -1)}
                        on:next={() => navigateSearchMatches("responseHead", 1)}
                    />

                    <div class="bodyBlock">
                        <HistoryMessageBlock
                            label="Body"
                            kind="body"
                            headBlockStr={selectedEntry.response
                                ?.headBlockStr ?? ""}
                            bodyStr={selectedEntry.response?.bodyStr ?? ""}
                            emptyLabel="No saved response"
                            searchQuery={responseBodyQuery}
                            bind:searchInput={blockSearchInputs.responseBody}
                            searchProgress={responseBodySearchProgress}
                            hasContent={responseBodyHasContent}
                            navigateDisabled={responseBodyMatchCount <= 0}
                            bind:containerElement={responseBodyContainer}
                            bind:matchCount={responseBodyMatchCount}
                            on:submit={() => applyBlockSearch("responseBody")}
                            on:previous={() =>
                                navigateSearchMatches("responseBody", -1)}
                            on:next={() =>
                                navigateSearchMatches("responseBody", 1)}
                        />
                    </div>
                </section>
            </div>
        </div>
    {/if}
</div>

{#if contextMenuState}
    <div
        class="savedRequestsContextMenu"
        style={`left: ${contextMenuState.x}px; top: ${contextMenuState.y}px;`}
        role="menu"
        tabindex="-1"
        on:click|stopPropagation
        on:keydown|stopPropagation
    >
        <RequestCopySubmenu on:select={copyContextRequest} />
        <button
            type="button"
            class="contextMenuItem"
            on:click={sendContextEntryToRepeater}
        >
            Send to Repeater
        </button>
        <button
            type="button"
            class="contextMenuItem"
            on:click={removeContextEntry}
        >
            Delete this
        </button>
        <button
            type="button"
            class="contextMenuItem danger"
            on:click={clearAllFromContextMenu}
        >
            Delete all
        </button>
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
        font-size: 22px;
        line-height: 1.1;
        letter-spacing: -0.03em;
        color: var(--text);
    }

    strong {
        font-weight: 800;
    }

    .savedRequestsView {
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    .savedRequestsContextMenu {
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

    .contextMenuItem:hover {
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.08);
    }

    .contextMenuItem.danger {
        color: var(--danger);
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

    .overviewCopy,
    .headerCopy {
        display: flex;
        flex-direction: column;
        gap: 6px;
        min-width: 0;
    }

    .overviewTools {
        display: flex;
        flex-direction: column;
        align-items: flex-end;
        gap: 10px;
    }

    .eyebrow,
    .messageLabel {
        color: var(--muted);
        font-size: 11px;
        font-weight: 900;
        letter-spacing: 0.12em;
        text-transform: uppercase;
    }

    .headerSub {
        margin: 0;
        color: var(--muted);
        font-size: 12px;
        line-height: 1.45;
    }

    .overviewStats,
    .detailBadges,
    .messageMeta {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
    }

    .statChip,
    .metaPill,
    .messageMeta span {
        padding: 6px 10px;
        border-radius: 999px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
    }

    .statChip {
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

    .metaPill,
    .messageMeta span {
        color: var(--muted-strong);
        font-size: 11px;
        font-weight: 700;
    }

    .metaPill.pending,
    .statusBadge.pending {
        background: var(--info-soft);
        border-color: var(--info-line);
        color: var(--info);
    }

    .metaPill.success,
    .statusBadge.success {
        background: var(--success-soft);
        border-color: var(--success-line);
        color: var(--success);
    }

    .metaPill.redirect,
    .statusBadge.redirect {
        background: var(--info-soft);
        border-color: var(--info-line);
        color: var(--info);
    }

    .metaPill.warning,
    .statusBadge.warning {
        background: var(--warning-soft);
        border-color: var(--warning-line);
        color: var(--warning);
    }

    .metaPill.danger,
    .statusBadge.danger {
        background: var(--danger-soft);
        border-color: var(--danger-line);
        color: var(--danger);
    }

    .metaPill.neutral,
    .statusBadge.neutral {
        background: rgba(148, 163, 184, 0.12);
        border-color: var(--neutral-line);
        color: var(--muted-strong);
    }

    .clearButton {
        appearance: none;
        min-height: 38px;
        padding: 0 14px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--text);
        font-size: 12px;
        font-weight: 700;
        cursor: pointer;
        transition:
            border-color 140ms ease,
            background 140ms ease,
            transform 140ms ease;
    }

    .clearButton:hover:not(:disabled) {
        transform: translateY(-1px);
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.08);
    }

    .clearButton:disabled {
        cursor: not-allowed;
        opacity: 0.55;
        transform: none;
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

    .tableViewport {
        overflow: auto;
        max-height: 470px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--field-bg);
    }

    .savedTable {
        min-width: 1080px;
    }

    .tableHead,
    .tableRow {
        display: grid;
        grid-template-columns: var(--saved-requests-columns);
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

    .statusBadge {
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

    .requestMetaPanel,
    .messageBlock {
        display: grid;
        gap: 10px;
    }

    .bodyBlock {
        min-width: 0;
        min-height: 0;
    }

    .bodyBlock :global(.prettyPanel) {
        max-height: min(56vh, 620px);
        overflow: auto;
        scrollbar-gutter: stable;
    }

    @media (max-width: 1080px) {
        .overviewCard,
        .cardHeader,
        .detailTop {
            flex-direction: column;
            align-items: stretch;
        }

        .overviewTools {
            align-items: stretch;
        }

        .detailHeaderTools {
            align-items: stretch;
        }

        .detailGrid {
            grid-template-columns: 1fr;
        }
    }
</style>
