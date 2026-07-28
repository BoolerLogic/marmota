<script lang="ts">
    import { onMount, tick } from "svelte";
    import {
        CloseProxy,
        ExportCA,
        GetInitialAppState,
        OpenConfigDirectory,
        ResetCA,
        StartProxy,
    } from "../wailsjs/go/main/App.js";
    import { proxy, settings } from "../wailsjs/go/models";
    import {
        BrowserOpenURL,
        EventsOn,
    } from "../wailsjs/runtime/runtime.js";
    import {
        setActiveWorkspaceTab,
        workspaceTabStore,
        type WorkspaceTabId,
    } from "@/app/state/workspaceTabStore";
    import AppTabIcon from "@/shared/navigation/AppTabIcon.svelte";
    import {
        ensureErrorCapture,
        errorStore,
        setErrorPanelActive,
    } from "@/features/errors/state/errorStore";
    import type {
        ProxyMode,
        ProxyState,
        UpstreamProxySettings,
    } from "@/features/proxy/utils/proxyUi";
    import {
        isValidPort,
        resolveListenIp,
    } from "@/features/proxy/utils/proxyUi";
    import {
        ensureHttpHistoryCapture,
        httpHistoryStore,
        setHttpHistoryPanelActive,
    } from "@/features/http-history/state/httpHistoryStore";
    import ConfigureProxyView from "@/features/proxy/ConfigureProxyView.svelte";
    import ErrorLogView from "@/features/errors/ErrorLogView.svelte";
    import HttpHistoryView from "@/features/http-history/HttpHistoryView.svelte";
    import InterceptionView from "@/features/interception/InterceptionView.svelte";
    import RepeaterView from "@/features/repeater/RepeaterView.svelte";
    import SavedRequestsView from "@/features/saved-requests/SavedRequestsView.svelte";
    import marmotaIcon from "@/assets/images/marmota_icon.png";
    import {
        getRememberedScrollPosition,
        rememberScrollPosition,
    } from "@/shared/utils/viewScrollMemory";

    type TabMeta = {
        id: WorkspaceTabId;
        label: string;
        eyebrow: string;
        description: string;
    };
    let proxyState: ProxyState = "idle";
    let proxyError: string | null = null;

    // Keep the proxy form state while switching sections.
    let proxyMode: ProxyMode = "local";
    let specificIp: string = "";
    let port: number = 8080;
    let skipServerCertVerify: boolean = false;
    let upstreamProxyEnabled: boolean = false;
    let upstreamProxyHost: string = "";
    let upstreamProxyPort: number = 1080;
    let upstreamProxyUsername: string = "";
    let upstreamProxyPassword: string = "";
    let configDirectory: string = "";
    let configFilePath: string = "";
    let configLoadWarning: string = "";
    let initialSettingsLoaded = false;
    let panelElement: HTMLElement | null = null;
    let restoredWorkspaceScrollKey = "";
    const projectGithubUrl = "https://github.com/BoolerLogic/Marmota";

    const tabs: TabMeta[] = [
        {
            id: "configureProxy",
            label: "Configure Proxy",
            eyebrow: "Control",
            description:
                "Listener startup, listen address, and current proxy status.",
        },
        {
            id: "httpHistory",
            label: "Traffic Inspector",
            eyebrow: "Capture",
            description:
                "Live list of captured requests and responses, sortable and persistent.",
        },
        {
            id: "savedRequests",
            label: "Saved Requests",
            eyebrow: "Saved",
            description:
                "Manually saved snapshots, separate from the live history.",
        },
        // {
        //     id: "interception",
        //     label: "Interception",
        //     eyebrow: "Rules",
        //     description:
        //         "Reserved area for pause, filter, and mutation rules.",
        // },
        {
            id: "repeater",
            label: "Repeater",
            eyebrow: "Replay",
            description: "Manual resend and testing area for captured traffic.",
        },
        {
            id: "errorLog",
            label: "Errors",
            eyebrow: "Monitor",
            description: "Error log emitted by the backend and pending review.",
        },
    ];

    $: activeTab = $workspaceTabStore;
    $: activeMeta = tabs.find((tab) => tab.id === activeTab) ?? tabs[0];
    $: resolvedIp = resolveListenIp(proxyMode, specificIp);
    $: portValid = isValidPort(port);
    $: errorState = $errorStore;
    $: historyState = $httpHistoryStore;
    $: unreadErrorCount = errorState.entries.filter(
        (entry) => !entry.read,
    ).length;
    $: unreadHistoryCount = historyState.unreadCount;
    $: unreadErrorBadgeLabel =
        unreadErrorCount > 99 ? "99+" : String(unreadErrorCount);
    $: unreadHistoryBadgeLabel =
        unreadHistoryCount > 99 ? "99+" : String(unreadHistoryCount);
    $: workspaceScrollKey = `workspace-panel:${activeTab}`;

    onMount(() => {
        ensureHttpHistoryCapture();
        ensureErrorCapture();

        const unsubscribeProxyStopped = EventsOn("proxy-stopped", () => {
            proxyState = "error";
            proxyError =
                "The proxy listener stopped unexpectedly. See the Error Log for details.";
        });

        void loadInitialAppState();
        return unsubscribeProxyStopped;
    });

    async function loadInitialAppState() {
        try {
            const initialState = await GetInitialAppState();
            const savedConfig = initialState.config;
            const savedMode = savedConfig?.proxyMode;

            if (
                savedMode === "local" ||
                savedMode === "all" ||
                savedMode === "specific"
            ) {
                proxyMode = savedMode;
            }
            specificIp = savedConfig?.specificIp ?? "";
            port =
                typeof savedConfig?.port === "number"
                    ? savedConfig.port
                    : 8080;
            skipServerCertVerify =
                savedConfig?.skipServerCertVerify ?? false;
            upstreamProxyEnabled =
                savedConfig?.upstreamProxy?.enabled ?? false;
            upstreamProxyHost =
                savedConfig?.upstreamProxy?.host ?? "";
            upstreamProxyPort =
                typeof savedConfig?.upstreamProxy?.port === "number"
                    ? savedConfig.upstreamProxy.port
                    : 1080;
            upstreamProxyUsername =
                savedConfig?.upstreamProxy?.username ?? "";
            upstreamProxyPassword =
                savedConfig?.upstreamProxy?.password ?? "";
            configDirectory = initialState.configDirectory ?? "";
            configFilePath = initialState.configFilePath ?? "";
            configLoadWarning = initialState.loadWarning ?? "";
        } catch (error: unknown) {
            configLoadWarning =
                error instanceof Error
                    ? error.message
                    : "Could not load Marmota configuration.";
        } finally {
            initialSettingsLoaded = true;
        }
    }

    $: setErrorPanelActive(activeTab === "errorLog");
    $: setHttpHistoryPanelActive(activeTab === "httpHistory");
    $: if (panelElement && workspaceScrollKey !== restoredWorkspaceScrollKey) {
        restoredWorkspaceScrollKey = workspaceScrollKey;

        tick().then(() => {
            if (!panelElement) return;
            panelElement.scrollTop =
                getRememberedScrollPosition(workspaceScrollKey);
        });
    }

    function handleTabSelect(tabId: WorkspaceTabId) {
        setActiveWorkspaceTab(tabId);
    }

    function handlePanelScroll() {
        if (!panelElement) return;

        rememberScrollPosition(workspaceScrollKey, panelElement.scrollTop);
    }

    function openProjectGithub() {
        if (typeof window !== "undefined" && "runtime" in window) {
            BrowserOpenURL(projectGithubUrl);
            return;
        }

        window.open(projectGithubUrl, "_blank", "noopener,noreferrer");
    }

    async function onStartProxy(
        mode: ProxyMode,
        selectedSpecificIp: string,
        port: number,
        skipServerCertVerify: boolean,
        upstreamProxy: UpstreamProxySettings,
    ) {
        if (!initialSettingsLoaded) {
            throw new Error("Marmota configuration is still loading.");
        }

        proxyState = "loading";
        proxyError = null;

        try {
            await StartProxy(
                settings.ProxyConfig.createFrom({
                    schemaVersion: 1,
                    proxyMode: mode,
                    specificIp: selectedSpecificIp.trim(),
                    port,
                    skipServerCertVerify,
                    upstreamProxy: proxy.UpstreamProxyConfig.createFrom({
                        enabled: upstreamProxy.enabled,
                        host: upstreamProxy.host.trim(),
                        port: upstreamProxy.port,
                        username: upstreamProxy.username,
                        password: upstreamProxy.password,
                    }),
                }),
            );
            configLoadWarning = "";
            proxyState = "running";
        } catch (e: unknown) {
            proxyState = "error";
            proxyError = e instanceof Error ? e.message : String(e);
        }
    }

    async function onCloseProxy() {
        proxyState = "loading";
        proxyError = null;

        try {
            await CloseProxy();
            proxyState = "idle";
        } catch (e: unknown) {
            proxyState = "error";
            proxyError = e instanceof Error ? e.message : String(e);
        }
    }

    async function onExportCA() {
        return ExportCA();
    }

    async function onResetCA() {
        await ResetCA();
    }

    async function onOpenConfigDirectory() {
        await OpenConfigDirectory();
    }
</script>

<main class="app">
    <div class="shell">
        <section class="workspace">
            <header class="brandBar" aria-label="Marmota application brand">
                <div class="brandLockup">
                    <span class="brandMark" aria-hidden="true">
                        <img
                            class="brandIcon"
                            src={marmotaIcon}
                            alt=""
                            loading="eager"
                            decoding="async"
                        />
                    </span>
                    <span class="brandText">
                        <span class="brandName">Marmota</span>
                        <span class="brandTagline">MITM Proxy</span>
                    </span>
                </div>

                <a
                    class="creditLink"
                    href={projectGithubUrl}
                    target="_blank"
                    rel="noreferrer"
                    on:click|preventDefault={openProjectGithub}
                    aria-label="Open Marmota GitHub repository"
                >
                    <span class="creditCopy">
                        <span class="creditEyebrow">Built by</span>
                        <span class="creditName">BoolerLogic</span>
                    </span>
                    <span class="githubChip">GitHub</span>
                    <svg
                        class="externalIcon"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        aria-hidden="true"
                    >
                        <path
                            d="M8 16 16 8"
                            stroke-linecap="round"
                            stroke-width="2"
                        />
                        <path
                            d="M9 8h7v7"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                        />
                    </svg>
                </a>
            </header>

            <nav class="topNav" aria-label="Sections">
                {#each tabs as tab (tab.id)}
                    <button
                        type="button"
                        class="navItem"
                        class:active={tab.id === activeTab}
                        on:click={() => handleTabSelect(tab.id)}
                    >
                        <AppTabIcon tabId={tab.id} />
                        <span class="navLabel">{tab.label}</span>
                        {#if tab.id === "httpHistory" && unreadHistoryCount > 0}
                            <span class="navBadge historyBadge">
                                {unreadHistoryBadgeLabel}
                            </span>
                        {:else if tab.id === "errorLog" && unreadErrorCount > 0}
                            <span class="navBadge errorBadge">
                                {unreadErrorBadgeLabel}
                            </span>
                        {/if}
                    </button>
                {/each}
            </nav>

            <header class="workspaceHeader">
                <div class="workspaceCopy">
                    <div class="workspaceEyebrow">{activeMeta.eyebrow}</div>
                    <h1>{activeMeta.label}</h1>
                    <p>{activeMeta.description}</p>
                </div>
            </header>

            <section
                class="panel"
                bind:this={panelElement}
                on:scroll={handlePanelScroll}
            >
                {#if activeTab === "configureProxy"}
                    <ConfigureProxyView
                        state={proxyState}
                        error={proxyError}
                        {resolvedIp}
                        {portValid}
                        {onStartProxy}
                        {onCloseProxy}
                        {onExportCA}
                        {onResetCA}
                        {onOpenConfigDirectory}
                        {configDirectory}
                        {configFilePath}
                        {configLoadWarning}
                        settingsReady={initialSettingsLoaded}
                        bind:proxyMode
                        bind:specificIp
                        bind:port
                        bind:skipServerCertVerify
                        bind:upstreamProxyEnabled
                        bind:upstreamProxyHost
                        bind:upstreamProxyPort
                        bind:upstreamProxyUsername
                        bind:upstreamProxyPassword
                    />
                {:else if activeTab === "httpHistory"}
                    <HttpHistoryView />
                {:else if activeTab === "savedRequests"}
                    <SavedRequestsView />
                {:else if activeTab === "interception"}
                    <InterceptionView />
                {:else if activeTab === "errorLog"}
                    <ErrorLogView />
                {:else}
                    <RepeaterView />
                {/if}
            </section>
        </section>
    </div>
</main>

<style>
    .app {
        min-height: 100vh;
        padding: 16px;
    }

    .shell {
        width: min(1400px, 100%);
        min-height: calc(100vh - 48px);
        margin: 0 auto;
        display: block;
    }

    .brandBar,
    .topNav,
    .workspaceHeader,
    .panel {
        border: 1px solid var(--line);
        background: var(--surface);
        box-shadow: var(--shadow-soft);
    }

    .workspaceEyebrow {
        font-size: 11px;
        font-weight: 900;
        letter-spacing: 0.16em;
        text-transform: uppercase;
        color: var(--muted);
    }

    .brandBar {
        border-radius: 16px;
        padding: 12px 14px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 14px;
        background: linear-gradient(
                135deg,
                rgba(251, 113, 133, 0.1),
                rgba(15, 23, 42, 0.96) 42%,
                rgba(52, 211, 153, 0.08)
            ),
            var(--surface);
    }

    .brandLockup,
    .creditLink {
        min-width: 0;
        display: inline-flex;
        align-items: center;
    }

    .brandLockup {
        gap: 12px;
    }

    .brandMark {
        width: 48px;
        height: 48px;
        flex: 0 0 auto;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        border-radius: 14px;
        border: 1px solid rgba(255, 255, 255, 0.14);
        background: rgba(2, 6, 23, 0.72);
        box-shadow: 0 8px 18px rgba(2, 6, 23, 0.2);
    }

    .brandIcon {
        width: 44px;
        height: 44px;
        display: block;
        border-radius: 12px;
        object-fit: cover;
    }

    .brandText,
    .creditCopy {
        min-width: 0;
        display: flex;
        flex-direction: column;
    }

    .brandName {
        color: var(--text);
        font-size: 22px;
        font-weight: 900;
        line-height: 1;
    }

    .brandTagline {
        margin-top: 5px;
        color: var(--muted);
        font-size: 12px;
        font-weight: 700;
        line-height: 1.25;
        overflow-wrap: anywhere;
    }

    .creditLink {
        flex: 0 1 auto;
        gap: 10px;
        padding: 9px 10px 9px 12px;
        border-radius: 12px;
        border: 1px solid var(--line);
        color: var(--text);
        background: rgba(2, 6, 23, 0.34);
        text-decoration: none;
        transition:
            border-color 140ms ease,
            background 140ms ease,
            transform 140ms ease;
    }

    .creditLink:hover {
        transform: translateY(-1px);
        border-color: var(--line-strong);
        background: rgba(255, 255, 255, 0.06);
    }

    .creditEyebrow {
        color: var(--muted);
        font-size: 11px;
        font-weight: 800;
        line-height: 1.1;
    }

    .creditName {
        color: var(--text);
        font-size: 13px;
        font-weight: 900;
        line-height: 1.25;
    }

    .githubChip {
        flex: 0 0 auto;
        padding: 4px 7px;
        border-radius: 999px;
        color: #ecfeff;
        background: rgba(var(--accent-rgb), 0.2);
        border: 1px solid rgba(var(--accent-rgb), 0.26);
        font-size: 11px;
        font-weight: 900;
        line-height: 1;
    }

    .externalIcon {
        width: 16px;
        height: 16px;
        flex: 0 0 auto;
        color: var(--muted-strong);
    }

    .topNav {
        border-radius: 16px;
        padding: 8px;
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
        background: var(--surface);
    }

    .navItem {
        appearance: none;
        flex: 1 1 180px;
        min-width: 0;
        border: 1px solid var(--line);
        border-radius: 12px;
        background: var(--surface-muted);
        color: inherit;
        text-align: center;
        padding: 10px 14px;
        display: flex;
        flex-direction: row;
        gap: 10px;
        align-items: center;
        justify-content: center;
        position: relative;
        overflow: visible;
        cursor: pointer;
        transition:
            border-color 140ms ease,
            background 140ms ease,
            transform 140ms ease,
            box-shadow 140ms ease;
    }

    .navItem:hover {
        transform: translateY(-1px);
        border-color: var(--line-strong);
        background: var(--surface-elevated);
    }

    .navItem.active {
        border-color: var(--accent);
        background: rgba(var(--accent-rgb), 0.08);
        box-shadow:
            inset 0 0 0 1px rgba(var(--accent-rgb), 0.16),
            0 4px 12px rgba(2, 6, 23, 0.14);
    }

    .navItem.active :global(.iconShell) {
        border-color: var(--accent);
        background: rgba(var(--accent-rgb), 0.1);
        color: var(--accent);
        box-shadow: none;
    }

    .navLabel {
        color: var(--text);
        font-weight: 700;
        font-size: 13px;
        line-height: 1.2;
        overflow-wrap: anywhere;
    }

    .navBadge {
        position: absolute;
        top: -6px;
        right: -6px;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 22px;
        height: 22px;
        padding: 0 6px;
        border-radius: 999px;
        border: 1px solid rgba(255, 255, 255, 0.14);
        font-size: 10px;
        font-weight: 700;
        box-shadow: 0 0 0 2px var(--surface);
        z-index: 2;
        pointer-events: none;
    }

    .errorBadge {
        background: var(--danger);
        color: #fff5f5;
    }

    .historyBadge {
        background: var(--success);
        color: #f0fdf4;
    }

    .workspace {
        min-width: 0;
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    .workspaceHeader {
        border-radius: 16px;
        padding: 18px 20px;
        display: flex;
        justify-content: space-between;
        gap: 18px;
        align-items: flex-start;
    }

    .workspaceCopy h1 {
        margin: 8px 0 6px 0;
        font-size: clamp(24px, 3.4vw, 32px);
        line-height: 1.1;
        letter-spacing: -0.03em;
        color: var(--text);
    }

    .workspaceCopy p {
        margin: 0;
        color: var(--muted);
        line-height: 1.55;
        max-width: 760px;
    }

    .panel {
        flex: 1;
        min-width: 0;
        border-radius: 16px;
        padding: 18px;
        overflow: auto;
    }

    @media (max-width: 760px) {
        .app {
            padding: 12px;
        }

        .shell {
            min-height: calc(100vh - 24px);
            gap: 14px;
        }

        .workspaceHeader,
        .brandBar,
        .panel {
            border-radius: 14px;
        }

        .brandBar {
            align-items: stretch;
            flex-direction: column;
        }

        .creditLink {
            justify-content: space-between;
        }

        .workspaceHeader {
            padding: 16px;
            flex-direction: column;
        }
        .panel {
            padding: 16px;
        }
    }
</style>
