<script lang="ts">
    import RepeaterCodeEditor from "./components/RepeaterCodeEditor.svelte";
    import RepeaterResponsePanel from "./components/RepeaterResponsePanel.svelte";
    import RepeaterValidationPanel from "./components/RepeaterValidationPanel.svelte";
    import {
        closeRepeaterTab,
        createRepeaterTab,
        duplicateRepeaterTab,
        getRepeaterTabTitle,
        repeaterStore,
        sendRepeaterTab,
        setActiveRepeaterTab,
        updateRepeaterTabRequest,
    } from "./state/repeaterStore";
    import {
        buildRepeaterUrlPreview,
        getRepeaterRequestPath,
        sanitizeRepeaterAuthorityInput,
    } from "./utils/repeaterRequest";

    $: repeaterState = $repeaterStore;
    $: activeTab =
        repeaterState.tabs.find((tab) => tab.id === repeaterState.activeTabId) ??
        repeaterState.tabs[0] ??
        null;
    $: activeParsedRequest = activeTab?.validation.parsedRequest ?? null;
    $: activeUrlPreview = activeTab
        ? buildRepeaterUrlPreview(
              activeTab.request.scheme,
              activeTab.request.host,
              getRepeaterRequestPath(activeParsedRequest),
          )
        : "";
    $: headIssueLines = activeTab
        ? [
              ...activeTab.validation.errors,
              ...activeTab.validation.warnings,
          ]
              .map((issue) => issue.line)
              .filter((line): line is number => line !== null)
        : [];

    function handleCloseTab(event: MouseEvent, tabId: string) {
        event.preventDefault();
        event.stopPropagation();
        closeRepeaterTab(tabId);
    }

    function updateActiveTabHost(host: string) {
        if (!activeTab) return;
        updateRepeaterTabRequest(activeTab.id, {
            host: sanitizeRepeaterAuthorityInput(host),
        });
    }

    function updateActiveTabScheme(useHttps: boolean) {
        if (!activeTab) return;
        updateRepeaterTabRequest(activeTab.id, {
            scheme: useHttps ? "https" : "http",
        });
    }

    function updateActiveTabServerCertificateValidation(
        validateServerCertificate: boolean,
    ) {
        if (!activeTab) return;
        updateRepeaterTabRequest(activeTab.id, {
            skipServerCertVerify: !validateServerCertificate,
        });
    }

    function updateActiveHeadBlock(headBlockStr: string) {
        if (!activeTab) return;
        updateRepeaterTabRequest(activeTab.id, { headBlockStr });
    }

    function updateActiveBody(bodyStr: string) {
        if (!activeTab) return;
        updateRepeaterTabRequest(activeTab.id, { bodyStr });
    }
</script>

<div class="repeaterView">
    <div class="toolbarCard">
        <div class="toolbarTabs" role="tablist" aria-label="Repeater tabs">
            {#each repeaterState.tabs as tab (tab.id)}
                <div class="toolbarTabItem" class:active={tab.id === activeTab?.id}>
                    <button
                        type="button"
                        class="toolbarTab"
                        class:active={tab.id === activeTab?.id}
                        on:click={() => setActiveRepeaterTab(tab.id)}
                        title={getRepeaterTabTitle(tab)}
                    >
                        <span class="toolbarTabLabel">
                            {getRepeaterTabTitle(tab)}
                        </span>
                    </button>

                    <button
                        type="button"
                        class="closeTabButton"
                        aria-label={`Close ${getRepeaterTabTitle(tab)}`}
                        title={`Close ${getRepeaterTabTitle(tab)}`}
                        on:click={(event) => handleCloseTab(event, tab.id)}
                    >
                        ×
                    </button>
                </div>
            {/each}
        </div>

        <button type="button" class="primaryAction" on:click={createRepeaterTab}>
            New tab
        </button>
    </div>

    {#if activeTab}
        <div class="workspaceGrid">
            <section class="requestCard">
                <div class="cardHeader">
                    <div class="headerCopy">
                        <span class="eyebrow">Request</span>
                        <h2>{getRepeaterTabTitle(activeTab)}</h2>
                        <p class="headerSub">
                            Edit the Head Block and Body in raw form. The
                            frontend validates before sending, does not touch
                            your editor, and only fills in Content-Length in the
                            real payload when it is missing.
                        </p>
                    </div>

                    <div class="headerActions">
                        <button
                            type="button"
                            class="secondaryAction"
                            on:click={() => duplicateRepeaterTab(activeTab.id)}
                        >
                            Duplicate
                        </button>
                        <button
                            type="button"
                            class="secondaryAction"
                            on:click={() => closeRepeaterTab(activeTab.id)}
                        >
                            Close
                        </button>
                        <button
                            type="button"
                            class="primaryAction"
                            disabled={activeTab.requestState === "sending"}
                            on:click={() => sendRepeaterTab(activeTab.id)}
                        >
                            {activeTab.requestState === "sending"
                                ? "Sending..."
                                : "Send"}
                        </button>
                    </div>
                </div>

                <div class="requestMetaGrid">
                    <label class="metaField">
                        <span class="fieldLabel">Secure</span>
                        <span class="checkField">
                            <input
                                type="checkbox"
                                checked={activeTab.request.scheme === "https"}
                                on:change={(event) =>
                                    updateActiveTabScheme(
                                        event.currentTarget.checked,
                                    )}
                            />
                            <span>
                                {activeTab.request.scheme === "https"
                                    ? "On"
                                    : "Off"}
                            </span>
                        </span>
                    </label>

                    <label
                        class="metaField"
                        class:disabledField={activeTab.request.scheme !== "https"}
                    >
                        <span class="fieldLabel">
                            Validate server certificate
                        </span>
                        <span class="checkField">
                            <input
                                type="checkbox"
                                checked={activeTab.request.scheme === "https" &&
                                    !activeTab.request.skipServerCertVerify}
                                disabled={activeTab.request.scheme !== "https"}
                                on:change={(event) =>
                                    updateActiveTabServerCertificateValidation(
                                        event.currentTarget.checked,
                                    )}
                            />
                            <span>
                                {activeTab.request.scheme !== "https"
                                    ? "Off"
                                    : !activeTab.request.skipServerCertVerify
                                    ? "On"
                                    : "Off"}
                            </span>
                        </span>
                    </label>

                    <label class="metaField hostField">
                        <span class="fieldLabel">Host / authority</span>
                        <input
                            class="hostInput"
                            type="text"
                            value={activeTab.request.host}
                            placeholder="example.com or example.com:8443"
                            autocapitalize="off"
                            autocomplete="off"
                            autocorrect="off"
                            spellcheck="false"
                            on:input={(event) =>
                                updateActiveTabHost(
                                    event.currentTarget.value,
                                )}
                        />
                    </label>
                </div>

                <div class="urlPreview">
                    <span class="fieldLabel">Derived URL</span>
                    <strong>{activeUrlPreview || "(incomplete)"}</strong>
                </div>

                <RepeaterCodeEditor
                    label="Head Block"
                    value={activeTab.request.headBlockStr}
                    placeholder="GET / HTTP/1.1"
                    issueLines={headIssueLines}
                    minLines={8}
                    ariaLabel="Raw Repeater head block"
                    on:change={(event) =>
                        updateActiveHeadBlock(event.detail.value)}
                />

                <RepeaterCodeEditor
                    label="Body"
                    value={activeTab.request.bodyStr}
                    placeholder="Raw request body"
                    minLines={10}
                    ariaLabel="Raw Repeater body"
                    on:change={(event) => updateActiveBody(event.detail.value)}
                />

                <RepeaterValidationPanel
                    errors={activeTab.validation.errors}
                    warnings={activeTab.validation.warnings}
                />
            </section>

            <section class="responseCard">
                <RepeaterResponsePanel
                    response={activeTab.response}
                    requestState={activeTab.requestState}
                    requestError={activeTab.requestError}
                />
            </section>
        </div>
    {:else}
        <div class="emptyState">
            <div class="emptyTitle">No Repeater tabs</div>
            <div class="emptySub">
                Create an empty tab or send a request from HTTP History with a
                right-click.
            </div>
            <button type="button" class="primaryAction" on:click={createRepeaterTab}>
                Create first tab
            </button>
        </div>
    {/if}
</div>

<style>
    h2 {
        margin: 0;
        font-size: 22px;
        line-height: 1.1;
        letter-spacing: -0.03em;
        color: var(--text);
    }

    .repeaterView {
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    .toolbarCard,
    .requestCard,
    .responseCard,
    .emptyState {
        border-radius: 16px;
        border: 1px solid var(--line);
        background: var(--surface-strong);
        box-shadow: var(--shadow-card);
    }

    .toolbarCard {
        display: flex;
        justify-content: space-between;
        gap: 12px;
        align-items: center;
        padding: 14px 16px;
    }

    .toolbarTabs {
        display: flex;
        gap: 8px;
        flex: 1 1 auto;
        min-width: 0;
        overflow-x: auto;
    }

    .toolbarTabItem {
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

    .toolbarTabItem:hover {
        transform: translateY(-1px);
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.08);
    }

    .toolbarTabItem.active {
        border-color: rgba(var(--accent-rgb), 0.72);
        background: rgba(var(--accent-rgb), 0.12);
        color: var(--text);
    }

    .toolbarTab {
        appearance: none;
        min-height: 38px;
        padding: 0;
        border: none;
        background: transparent;
        color: inherit;
        cursor: pointer;
        white-space: nowrap;
    }

    .toolbarTabLabel {
        font-size: 12px;
        font-weight: 800;
        letter-spacing: 0.04em;
    }

    .closeTabButton {
        appearance: none;
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
            color 140ms ease;
    }

    .closeTabButton:hover {
        background: rgba(148, 163, 184, 0.12);
        color: var(--text);
    }

    .primaryAction,
    .secondaryAction {
        appearance: none;
        min-height: 38px;
        padding: 0 14px;
        border-radius: 10px;
        font-size: 11px;
        font-weight: 800;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        cursor: pointer;
        transition:
            transform 140ms ease,
            border-color 140ms ease,
            background 140ms ease,
            color 140ms ease;
    }

    .primaryAction {
        border: 1px solid rgba(var(--accent-rgb), 0.72);
        background: var(--accent);
        color: #f8fafc;
    }

    .secondaryAction {
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--text);
    }

    .primaryAction:hover:not(:disabled),
    .secondaryAction:hover:not(:disabled) {
        transform: translateY(-1px);
    }

    .secondaryAction:hover:not(:disabled) {
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.08);
    }

    .primaryAction:hover:not(:disabled) {
        border-color: var(--accent-strong);
        background: var(--accent-strong);
    }

    .primaryAction:disabled,
    .secondaryAction:disabled {
        cursor: not-allowed;
        opacity: 0.6;
        transform: none;
    }

    .workspaceGrid {
        display: grid;
        grid-template-columns: minmax(0, 1.15fr) minmax(0, 0.95fr);
        gap: 14px;
    }

    .requestCard,
    .responseCard {
        padding: 16px;
        min-width: 0;
    }

    .requestCard {
        display: grid;
        gap: 14px;
    }

    .cardHeader {
        display: flex;
        justify-content: space-between;
        gap: 14px;
        align-items: flex-start;
    }

    .headerCopy {
        display: grid;
        gap: 6px;
        min-width: 0;
    }

    .eyebrow,
    .fieldLabel {
        color: var(--muted);
        font-size: 11px;
        font-weight: 800;
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .headerSub {
        margin: 0;
        color: var(--muted);
        font-size: 12px;
        line-height: 1.5;
        max-width: 640px;
    }

    .headerActions {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
        justify-content: flex-end;
    }

    .requestMetaGrid {
        display: grid;
        grid-template-columns: 170px 250px minmax(0, 1fr);
        gap: 12px;
    }

    .metaField {
        display: grid;
        gap: 8px;
        min-width: 0;
    }

    .hostField {
        min-width: 0;
    }

    .checkField {
        display: inline-flex;
        align-items: center;
        gap: 10px;
        min-height: 40px;
        padding: 0 12px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--field-bg);
        color: var(--text);
    }

    .disabledField .checkField {
        opacity: 0.65;
    }

    .disabledField .checkField input {
        cursor: not-allowed;
    }

    .hostInput {
        min-height: 40px;
        width: 100%;
        padding: 0 14px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--field-bg);
        color: var(--text);
        outline: none;
        transition:
            border-color 140ms ease,
            box-shadow 140ms ease;
    }

    .hostInput:focus {
        border-color: rgba(var(--accent-rgb), 0.42);
        box-shadow: 0 0 0 4px rgba(var(--accent-rgb), 0.14);
    }

    .urlPreview {
        display: grid;
        gap: 6px;
        padding: 12px 14px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
    }

    .urlPreview strong {
        color: var(--text);
        font-size: 12px;
        line-height: 1.5;
        word-break: break-word;
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
    }

    .emptyState {
        display: grid;
        gap: 10px;
        justify-items: start;
        padding: 24px;
    }

    .emptyTitle {
        color: var(--text);
        font-size: 18px;
        font-weight: 900;
    }

    .emptySub {
        color: var(--muted);
        line-height: 1.55;
        max-width: 620px;
    }

    @media (max-width: 1100px) {
        .workspaceGrid {
            grid-template-columns: 1fr;
        }
    }

    @media (max-width: 760px) {
        .toolbarCard,
        .cardHeader {
            flex-direction: column;
            align-items: stretch;
        }

        .requestMetaGrid {
            grid-template-columns: 1fr;
        }

        .headerActions {
            justify-content: stretch;
        }
    }
</style>
