<script lang="ts">
    import AppAlertDialog from "@/shared/components/AppAlertDialog.svelte";
    import ProxyStatusCard from "./components/ProxyStatusCard.svelte";
    import type {
        ProxyMode,
        ProxyState,
        UpstreamProxySettings,
    } from "./utils/proxyUi";
    import {
        hasValidUpstreamProxyCredentials,
        isValidIP,
        isValidPort,
        isValidUpstreamProxy,
        isValidUpstreamProxyHost,
        isValidUpstreamProxyPort,
        resolveListenIp,
    } from "./utils/proxyUi";

    export let error: string | null = null;
    export let onStartProxy: (
        mode: ProxyMode,
        specificIp: string,
        port: number,
        skipServerCertVerify: boolean,
        upstreamProxy: UpstreamProxySettings,
    ) => Promise<void> | void;
    export let onCloseProxy: () => Promise<void> | void;
    export let onExportCA: () => Promise<string> | string;
    export let onResetCA: () => Promise<void> | void;
    export let onOpenConfigDirectory: () => Promise<void> | void;

    export let state: ProxyState;
    export let proxyMode: ProxyMode;
    export let specificIp: string;
    export let port: number;
    export let skipServerCertVerify: boolean = false;
    export let upstreamProxyEnabled: boolean = false;
    export let upstreamProxyHost: string = "";
    export let upstreamProxyPort: number = 1080;
    export let upstreamProxyUsername: string = "";
    export let upstreamProxyPassword: string = "";
    export let configDirectory: string = "";
    export let configFilePath: string = "";
    export let configLoadWarning: string = "";
    export let settingsReady: boolean = false;

    export let resolvedIp: string;
    export let portValid: boolean;

    type OperationState =
        | "idle"
        | "starting"
        | "stopping"
        | "exporting"
        | "resetting";

    let operationState: OperationState = "idle";
    let caMessage = "";
    let caMessageTone: "neutral" | "success" | "error" = "neutral";
    let resetConfirmDialogOpen = false;
    let resetSuccessDialogOpen = false;
    let openingConfigDirectory = false;
    let storageActionError = "";

    $: operationBusy = operationState !== "idle";
    $: locked =
        !settingsReady ||
        state === "loading" ||
        state === "running" ||
        operationBusy;
    $: exportBusy = operationState === "exporting";
    $: resetBusy = operationState === "resetting";
    $: startBusy = operationState === "starting";
    $: stopBusy = operationState === "stopping";
    $: resolvedIp = resolveListenIp(proxyMode, specificIp);
    $: ipValid = isValidIP(resolvedIp);
    $: portValid = isValidPort(port);
    $: upstreamProxySettings = {
        enabled: upstreamProxyEnabled,
        host: upstreamProxyHost,
        port: upstreamProxyPort,
        username: upstreamProxyUsername,
        password: upstreamProxyPassword,
    } satisfies UpstreamProxySettings;
    $: upstreamProxyHostValid =
        !upstreamProxyEnabled ||
        isValidUpstreamProxyHost(upstreamProxyHost);
    $: upstreamProxyPortValid =
        !upstreamProxyEnabled ||
        isValidUpstreamProxyPort(upstreamProxyPort);
    $: upstreamProxyCredentialsValid =
        !upstreamProxyEnabled ||
        hasValidUpstreamProxyCredentials(
            upstreamProxyUsername,
            upstreamProxyPassword,
        );
    $: upstreamProxyValid = isValidUpstreamProxy(upstreamProxySettings);
    $: canStart =
        settingsReady &&
        ipValid &&
        portValid &&
        upstreamProxyValid &&
        !operationBusy &&
        state !== "loading" &&
        state !== "running";
    $: canStop = state === "running" && !operationBusy;
    $: canExportCA = !operationBusy && state !== "loading";
    $: canResetCA =
        !operationBusy && state !== "loading" && state !== "running";
    $: startLabel = startBusy
        ? "Starting..."
        : state === "running"
          ? "Proxy running"
          : "Start proxy";
    $: stopLabel = stopBusy ? "Stopping..." : "Stop proxy";

    $: ipHelper =
        proxyMode === "specific"
            ? specificIp.trim().length === 0
                ? "Required Specific IP"
                : ipValid
                  ? "Valid IP"
                  : "Invalid IP (xxx.xxx.xxx.xxx)"
            : "";

    $: portHelper =
        port === null
            ? "Required Port"
            : portValid
              ? "Valid Port"
              : "Invalid Port (0-65535)";
    $: tlsHelper = skipServerCertVerify
        ? "Server TLS verification will be skipped"
        : "The server TLS certificate will be verified";
    $: upstreamHostHelper = upstreamProxyHostValid
        ? "Hostname or IP address"
        : "A valid SOCKS5 hostname or IP is required";
    $: upstreamPortHelper = upstreamProxyPortValid
        ? "Valid SOCKS5 port"
        : "Port must be between 1 and 65535";
    $: upstreamCredentialsHelper = upstreamProxyCredentialsValid
        ? upstreamProxyUsername.length > 0
            ? "Credentials will be supplied if the SOCKS5 server requests authentication"
            : "Leave both fields empty when authentication is not required"
        : "Username and password must either both be filled in or both be empty";

    async function handleStart() {
        if (!canStart) return;
        if (port === null) return;

        operationState = "starting";
        try {
            await onStartProxy(
                proxyMode,
                specificIp,
                port,
                skipServerCertVerify,
                upstreamProxySettings,
            );
        } finally {
            operationState = "idle";
        }
    }

    async function handleStop() {
        if (!canStop) return;

        operationState = "stopping";
        try {
            await onCloseProxy();
        } finally {
            operationState = "idle";
        }
    }

    async function handleOpenConfigDirectory() {
        if (openingConfigDirectory || !configDirectory) return;

        openingConfigDirectory = true;
        storageActionError = "";
        try {
            await onOpenConfigDirectory();
        } catch (error: unknown) {
            storageActionError =
                error instanceof Error
                    ? error.message
                    : "Could not open the Marmota configuration directory.";
        } finally {
            openingConfigDirectory = false;
        }
    }

    function setCaError(e: unknown, fallbackMessage: string) {
        caMessageTone = "error";
        caMessage =
            e instanceof Error ? e.message : String(e || fallbackMessage);
    }

    async function handleExportCA() {
        if (!canExportCA) return;

        operationState = "exporting";
        caMessage = "";
        caMessageTone = "neutral";

        try {
            const savePath = await onExportCA();
            if (savePath.trim().length === 0) {
                caMessageTone = "neutral";
                caMessage = "No file was saved.";
            } else {
                caMessageTone = "success";
                caMessage = `CA certificate saved to: ${savePath}`;
            }
        } catch (e: unknown) {
            setCaError(e, "CA export failed.");
        } finally {
            operationState = "idle";
        }
    }

    function openResetConfirmDialog() {
        if (!canResetCA) return;

        caMessage = "";
        caMessageTone = "neutral";
        resetConfirmDialogOpen = true;
    }

    function closeResetConfirmDialog() {
        if (resetBusy) return;

        resetConfirmDialogOpen = false;
    }

    function handleConfirmBackdropClick(event: MouseEvent) {
        if (event.target === event.currentTarget) {
            closeResetConfirmDialog();
        }
    }

    function handleWindowKeydown(event: KeyboardEvent) {
        if (!resetConfirmDialogOpen || event.key !== "Escape") return;

        event.preventDefault();
        closeResetConfirmDialog();
    }

    async function confirmResetCA() {
        if (!canResetCA) return;

        operationState = "resetting";
        caMessage = "";
        caMessageTone = "neutral";

        try {
            await onResetCA();
            caMessageTone = "success";
            caMessage =
                "The local CA was reset. Trust the new certificate before inspecting HTTPS traffic.";
            resetSuccessDialogOpen = true;
        } catch (e: unknown) {
            setCaError(e, "CA reset failed.");
        } finally {
            resetConfirmDialogOpen = false;
            operationState = "idle";
        }
    }
</script>

<svelte:window on:keydown={handleWindowKeydown} />

<div class="configureView">
    <div class="heroCard">
        <div class="heroCopy">
            <span class="eyebrow">Proxy Control</span>
            <h2>Configure the MITM listener</h2>
            <p>
                Set the listen IP and port from a compact form. The current
                status stays visible on the right.
            </p>
        </div>
        <div class="heroPills">
            <span class="metaPill">Listen IP: {resolvedIp || "-"}</span>
            <span class="metaPill">Port: {portValid ? String(port) : "-"}</span>
            <span class="metaPill"
                >TLS: {skipServerCertVerify ? "Skipped" : "Verified"}</span
            >
            <span class="metaPill">
                Outbound: {upstreamProxyEnabled ? "SOCKS5" : "Direct"}
            </span>
        </div>
    </div>

    <div class="contentGrid">
        <section class="formCard">
            <div class="formHeader">
                <div>
                    <span class="eyebrow">Listen Target</span>
                    <h3>Listen settings</h3>
                </div>
            </div>

            <div class="fieldGrid">
                <label class="fieldCard">
                    <span class="fieldLabel">IP mode</span>
                    <select
                        class="input"
                        bind:value={proxyMode}
                        disabled={locked}
                    >
                        <option value="local">Local</option>
                        <option value="all">All Interfaces</option>
                        <option value="specific">Specific IP</option>
                    </select>
                    <span class="fieldHint">
                        {proxyMode === "local"
                            ? "Listens only on localhost"
                            : proxyMode === "all"
                              ? "Exposes the proxy on all interfaces"
                              : ipHelper}
                    </span>
                </label>

                <label class="fieldCard">
                    <span class="fieldLabel">Port</span>
                    <input
                        class="input"
                        type="number"
                        min="0"
                        max="65535"
                        step="1"
                        bind:value={port}
                        placeholder="e.g. 8080"
                        disabled={locked}
                    />
                    <span class="fieldHint">{portHelper}</span>
                </label>
            </div>

            {#if proxyMode === "specific"}
                <label class="fieldCard singleField">
                    <span class="fieldLabel">Specific IP</span>
                    <input
                        class="input"
                        type="text"
                        bind:value={specificIp}
                        placeholder="e.g. 192.168.1.50"
                        disabled={locked}
                    />
                    <span class="fieldHint">{ipHelper}</span>
                </label>
            {/if}

            <label class="fieldCard singleField">
                <span class="fieldLabel">Certificate Verification</span>
                <div class="toggleRow">
                    <input
                        class="checkInput"
                        type="checkbox"
                        bind:checked={skipServerCertVerify}
                        disabled={locked}
                    />
                    <div class="toggleCopy">
                        <strong>Skip server TLS certificate verification</strong
                        >
                        <span class="fieldHint">{tlsHelper}</span>
                    </div>
                </div>
            </label>

            <section class="upstreamSection" aria-labelledby="upstream-title">
                <div class="upstreamHeader">
                    <div class="upstreamCopy">
                        <span class="eyebrow">Outbound Route</span>
                        <h3 id="upstream-title">SOCKS5 upstream proxy</h3>
                        <p>
                            When enabled, Marmota routes every intercepted
                            outbound connection through this SOCKS5 proxy
                            instead of connecting from this machine directly.
                            TLS interception remains local to Marmota.
                        </p>
                    </div>

                    <label class="enableToggle">
                        <input
                            class="checkInput"
                            type="checkbox"
                            bind:checked={upstreamProxyEnabled}
                            disabled={locked}
                        />
                        <span>{upstreamProxyEnabled ? "Enabled" : "Disabled"}</span>
                    </label>
                </div>

                {#if upstreamProxyEnabled}
                    <div class="upstreamFields">
                        <label class="fieldCard">
                            <span class="fieldLabel">Host</span>
                            <input
                                class="input"
                                class:invalid={!upstreamProxyHostValid}
                                type="text"
                                bind:value={upstreamProxyHost}
                                placeholder="e.g. brd.superproxy.io"
                                autocomplete="off"
                                spellcheck="false"
                                disabled={locked}
                            />
                            <span class="fieldHint">{upstreamHostHelper}</span>
                        </label>

                        <label class="fieldCard">
                            <span class="fieldLabel">Port</span>
                            <input
                                class="input"
                                class:invalid={!upstreamProxyPortValid}
                                type="number"
                                min="1"
                                max="65535"
                                step="1"
                                bind:value={upstreamProxyPort}
                                placeholder="e.g. 1080"
                                disabled={locked}
                            />
                            <span class="fieldHint">{upstreamPortHelper}</span>
                        </label>

                        <label class="fieldCard">
                            <span class="fieldLabel">Username (optional)</span>
                            <input
                                class="input"
                                class:invalid={!upstreamProxyCredentialsValid}
                                type="text"
                                bind:value={upstreamProxyUsername}
                                placeholder="SOCKS5 username"
                                autocomplete="off"
                                spellcheck="false"
                                disabled={locked}
                            />
                        </label>

                        <label class="fieldCard">
                            <span class="fieldLabel">Password (optional)</span>
                            <input
                                class="input"
                                class:invalid={!upstreamProxyCredentialsValid}
                                type="text"
                                bind:value={upstreamProxyPassword}
                                placeholder="SOCKS5 password"
                                autocomplete="off"
                                disabled={locked}
                            />
                        </label>
                    </div>

                    <span
                        class:invalidHint={!upstreamProxyCredentialsValid}
                        class="credentialsHint"
                    >
                        {upstreamCredentialsHelper}
                    </span>

                    {#if proxyMode !== "local"}
                        <div class="networkExposureWarning" role="alert">
                            <strong>Protect this listener</strong>
                            <span>
                                Marmota does not authenticate inbound proxy
                                clients. Binding outside localhost can let other
                                devices use this upstream route. Restrict access
                                with a firewall and a trusted network.
                            </span>
                        </div>
                    {/if}
                {/if}
            </section>

            {#if state === "error"}
                <div class="errorBox" role="alert">
                    <strong>Startup error</strong>
                    <span>{error ?? "unknown"}</span>
                </div>
            {/if}

            <div class="actions">
                <button
                    type="button"
                    class="primaryButton"
                    on:click={handleStart}
                    disabled={!canStart}
                >
                    {startLabel}
                </button>

                <button
                    type="button"
                    class="secondaryButton"
                    on:click={handleStop}
                    disabled={!canStop}
                >
                    {stopLabel}
                </button>
            </div>
        </section>

        <aside class="statusColumn">
            <ProxyStatusCard
                {resolvedIp}
                {portValid}
                {port}
                {state}
                {skipServerCertVerify}
                {upstreamProxyEnabled}
                {upstreamProxyHost}
                {upstreamProxyPort}
            />

            <section class="certificateCard">
                <div class="certificateHeader">
                    <div>
                        <span class="eyebrow">Certificate Authority</span>
                        <h3>Marmota CA</h3>
                    </div>
                    <span class="certificateBadge">Local</span>
                </div>

                <div class="certificateActions">
                    <button
                        type="button"
                        class="certificateButton"
                        on:click={handleExportCA}
                        disabled={!canExportCA}
                    >
                        <span>{exportBusy ? "Exporting..." : "Export CA"}</span>
                    </button>

                    <button
                        type="button"
                        class="certificateButton dangerButton"
                        on:click={openResetConfirmDialog}
                        disabled={!canResetCA}
                        title={state === "running"
                            ? "Stop the proxy before resetting the CA"
                            : "Remove and recreate the local Marmota CA"}
                    >
                        <span>{resetBusy ? "Resetting..." : "Reset CA"}</span>
                    </button>
                </div>

                <span class="certificateHint">
                    Reset creates a new CA. Export and trust it again after reset.
                </span>

                {#if caMessage}
                    <span
                        class:caError={caMessageTone === "error"}
                        class:caSuccess={caMessageTone === "success"}
                        class="caMessage"
                        role={caMessageTone === "error" ? "alert" : undefined}
                        aria-live="polite"
                    >
                        {caMessage}
                    </span>
                {/if}
            </section>
        </aside>
    </div>

    <section class="storageNotice" aria-labelledby="storage-notice-title">
        <div class="storageIcon" aria-hidden="true">
            <svg viewBox="0 0 24 24" focusable="false">
                <path d="M3.5 6.5h6l2 2h9v9.5a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2V6.5Z" />
                <path d="M3.5 10h17" />
            </svg>
        </div>

        <div class="storageCopy">
            <span class="eyebrow">Local storage</span>
            <h3 id="storage-notice-title">
                Marmota configuration and certificate files are stored in
            </h3>
            <button
                type="button"
                class="storagePath"
                on:click={handleOpenConfigDirectory}
                disabled={!configDirectory || openingConfigDirectory}
                title={configDirectory
                    ? `Open ${configDirectory}`
                    : "Loading configuration directory"}
            >
                <span class="storagePathText">
                    {configDirectory || "Loading configuration directory..."}
                </span>
                <span class="storagePathAction">
                    {openingConfigDirectory ? "Opening..." : "Open folder"}
                </span>
            </button>
            {#if configFilePath}
                <span class="storageFileHint">
                    Proxy settings: {configFilePath}. Configured SOCKS5
                    credentials are stored locally in this file.
                </span>
            {/if}
            {#if configLoadWarning}
                <span class="storageWarning" role="alert">
                    {configLoadWarning}
                </span>
            {/if}
            {#if storageActionError}
                <span class="storageWarning" role="alert">
                    {storageActionError}
                </span>
            {/if}
        </div>
    </section>
</div>

{#if resetConfirmDialogOpen}
    <div
        class="confirmOverlay"
        role="presentation"
        tabindex="-1"
        on:click={handleConfirmBackdropClick}
    >
        <div
            class="confirmDialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="reset-ca-confirm-title"
            aria-describedby="reset-ca-confirm-message"
        >
            <div class="confirmCopy">
                <span class="eyebrow">Certificate Authority</span>
                <h3 id="reset-ca-confirm-title">Reset Marmota CA?</h3>
                <p id="reset-ca-confirm-message" class="confirmText">
                    This will remove the current local CA and create a new one.
                    You will need to export and trust the new certificate again
                    before inspecting HTTPS traffic.
                </p>
            </div>

            <div class="confirmActions">
                <button
                    type="button"
                    class="confirmButton secondary"
                    on:click={closeResetConfirmDialog}
                    disabled={resetBusy}
                >
                    Cancel
                </button>
                <button
                    type="button"
                    class="confirmButton danger"
                    on:click={confirmResetCA}
                    disabled={resetBusy}
                >
                    {resetBusy ? "Resetting..." : "Reset CA"}
                </button>
            </div>
        </div>
    </div>
{/if}

{#if resetSuccessDialogOpen}
    <AppAlertDialog
        eyebrow="Certificate Authority"
        title="CA certificate reset"
        message="Marmota created a new local CA certificate. Export and trust the new certificate before inspecting HTTPS traffic."
        buttonLabel="OK"
        on:close={() => (resetSuccessDialogOpen = false)}
    />
{/if}

<style>
    h2,
    h3,
    p {
        margin: 0;
    }

    h2 {
        font-size: clamp(24px, 3vw, 30px);
        line-height: 1.1;
        letter-spacing: -0.03em;
        color: var(--text);
    }

    h3 {
        font-size: 18px;
        line-height: 1.2;
        letter-spacing: -0.02em;
        color: var(--text);
    }

    p {
        color: var(--muted);
        line-height: 1.6;
    }

    .configureView {
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    .heroCard,
    .formCard,
    .storageNotice {
        border-radius: 16px;
        border: 1px solid var(--line);
        background: var(--surface-strong);
        box-shadow: var(--shadow-card);
    }

    .heroCard {
        padding: 18px;
        display: flex;
        justify-content: space-between;
        gap: 16px;
        align-items: flex-start;
    }

    .heroCopy {
        display: flex;
        flex-direction: column;
        gap: 8px;
        max-width: 720px;
    }

    .eyebrow {
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.12em;
        text-transform: uppercase;
        color: var(--muted);
    }

    .heroPills {
        display: flex;
        flex-wrap: wrap;
        gap: 10px;
        justify-content: flex-end;
        align-self: stretch;
        align-items: flex-start;
    }

    .metaPill {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        padding: 8px 12px;
        border-radius: 999px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--muted-strong);
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.04em;
    }

    .storageNotice {
        display: grid;
        grid-template-columns: auto minmax(0, 1fr);
        gap: 14px;
        align-items: start;
        padding: 16px 18px;
    }

    .storageIcon {
        width: 42px;
        height: 42px;
        display: inline-grid;
        place-items: center;
        border-radius: 12px;
        border: 1px solid rgba(var(--accent-rgb), 0.35);
        background: var(--accent-soft);
        color: var(--accent);
    }

    .storageIcon svg {
        width: 21px;
        height: 21px;
        fill: none;
        stroke: currentColor;
        stroke-width: 1.8;
        stroke-linecap: round;
        stroke-linejoin: round;
    }

    .storageCopy {
        min-width: 0;
        display: grid;
        gap: 7px;
    }

    .storagePath {
        appearance: none;
        width: 100%;
        min-width: 0;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
        padding: 10px 12px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--field-bg);
        color: var(--text);
        cursor: pointer;
        text-align: left;
    }

    .storagePath:hover:not(:disabled),
    .storagePath:focus-visible {
        border-color: rgba(var(--accent-rgb), 0.6);
        box-shadow: 0 0 0 4px rgba(var(--accent-rgb), 0.12);
        outline: none;
    }

    .storagePath:disabled {
        cursor: wait;
        opacity: 0.72;
    }

    .storagePathText {
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 12px;
    }

    .storagePathAction {
        flex: 0 0 auto;
        color: var(--accent);
        font-size: 11px;
        font-weight: 800;
    }

    .storageFileHint,
    .storageWarning {
        color: var(--muted);
        font-size: 11px;
        line-height: 1.45;
        overflow-wrap: anywhere;
    }

    .storageWarning {
        color: var(--warning);
    }

    .contentGrid {
        display: grid;
        grid-template-columns: minmax(0, 1.35fr) minmax(280px, 360px);
        gap: 18px;
        align-items: start;
    }

    .formCard {
        padding: 18px;
    }

    .formHeader {
        display: flex;
        justify-content: space-between;
        gap: 14px;
        align-items: center;
        margin-bottom: 16px;
    }

    .fieldGrid {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 14px;
    }

    .fieldCard {
        display: flex;
        flex-direction: column;
        gap: 10px;
        padding: 14px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
    }

    .singleField {
        margin-top: 14px;
        max-width: none;
    }

    .upstreamSection {
        display: grid;
        gap: 16px;
        margin-top: 14px;
        padding: 16px;
        border-radius: 14px;
        border: 1px solid var(--line);
        background:
            linear-gradient(
                135deg,
                rgba(var(--accent-rgb), 0.08),
                transparent 46%
            ),
            var(--surface-muted);
    }

    .upstreamHeader {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        gap: 18px;
    }

    .upstreamCopy {
        display: grid;
        gap: 7px;
        max-width: 680px;
    }

    .upstreamCopy p {
        max-width: 660px;
        font-size: 13px;
        line-height: 1.5;
    }

    .enableToggle {
        flex: 0 0 auto;
        min-height: 38px;
        display: inline-flex;
        align-items: center;
        gap: 9px;
        padding: 0 12px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--field-bg);
        color: var(--text);
        font-size: 12px;
        font-weight: 800;
        cursor: pointer;
    }

    .upstreamFields {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 12px;
    }

    .credentialsHint {
        color: var(--muted);
        font-size: 12px;
        line-height: 1.45;
    }

    .credentialsHint.invalidHint {
        color: var(--warning);
    }

    .networkExposureWarning {
        display: grid;
        gap: 4px;
        padding: 11px 12px;
        border-radius: 10px;
        border: 1px solid var(--warning-line);
        background: var(--warning-soft);
        color: var(--warning);
        font-size: 12px;
        line-height: 1.45;
    }

    .networkExposureWarning strong {
        color: var(--text);
    }

    .fieldLabel {
        color: var(--text);
        font-size: 13px;
        font-weight: 700;
    }

    .fieldHint {
        color: var(--muted);
        font-size: 12px;
    }

    .input {
        width: 100%;
        min-height: 42px;
        padding: 0 14px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--field-bg);
        color: var(--text);
        outline: none;
        transition:
            border-color 140ms ease,
            box-shadow 140ms ease,
            background 140ms ease;
    }

    .input:focus {
        border-color: rgba(var(--accent-rgb), 0.42);
        box-shadow: 0 0 0 4px rgba(var(--accent-rgb), 0.18);
    }

    .input.invalid {
        border-color: var(--warning);
    }

    .input:disabled {
        opacity: 1;
        cursor: not-allowed;
        color: var(--muted);
        background: var(--surface-elevated);
    }

    .toggleRow {
        display: flex;
        gap: 12px;
        align-items: flex-start;
    }

    .checkInput {
        width: 18px;
        height: 18px;
        margin-top: 2px;
        accent-color: var(--accent);
        cursor: pointer;
    }

    .checkInput:disabled {
        cursor: not-allowed;
        opacity: 0.6;
    }

    .toggleCopy {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }

    .toggleCopy strong {
        color: var(--text);
        font-size: 13px;
        font-weight: 700;
        line-height: 1.35;
    }

    .errorBox {
        margin-top: 14px;
        padding: 14px 16px;
        border-radius: 12px;
        border: 1px solid var(--danger-line);
        background: var(--danger-soft);
        display: flex;
        flex-direction: column;
        gap: 6px;
        color: #fecaca;
    }

    .actions {
        margin-top: 22px;
        display: flex;
        gap: 10px;
        flex-wrap: wrap;
    }

    .statusColumn {
        position: sticky;
        top: 0;
        display: grid;
        gap: 14px;
    }

    .primaryButton,
    .secondaryButton {
        appearance: none;
        min-height: 42px;
        padding: 0 16px;
        border-radius: 10px;
        border: 1px solid transparent;
        cursor: pointer;
        font-weight: 700;
        transition:
            transform 140ms ease,
            filter 140ms ease,
            border-color 140ms ease,
            background 140ms ease;
    }

    .primaryButton {
        background: var(--accent);
        border-color: rgba(var(--accent-rgb), 0.72);
        color: #eff6ff;
    }

    .secondaryButton {
        background: var(--surface-muted);
        border-color: var(--line);
        color: var(--text);
    }

    .primaryButton:hover:not(:disabled),
    .secondaryButton:hover:not(:disabled) {
        transform: translateY(-1px);
        filter: none;
    }

    .primaryButton:hover:not(:disabled) {
        background: var(--accent-strong);
        border-color: var(--accent-strong);
    }

    .secondaryButton:hover:not(:disabled) {
        background: var(--surface-elevated);
        border-color: var(--line-strong);
    }

    .primaryButton:disabled,
    .secondaryButton:disabled {
        cursor: not-allowed;
        opacity: 0.55;
    }

    .certificateCard {
        display: grid;
        gap: 14px;
        padding: 16px;
        border-radius: 16px;
        border: 1px solid var(--line);
        background:
            linear-gradient(
                135deg,
                rgba(var(--accent-rgb), 0.1),
                rgba(15, 23, 42, 0.98) 46%,
                rgba(52, 211, 153, 0.08)
            ),
            var(--surface-strong);
        box-shadow: var(--shadow-card);
    }

    .certificateHeader {
        display: flex;
        justify-content: space-between;
        gap: 12px;
        align-items: flex-start;
    }

    .certificateBadge {
        flex: 0 0 auto;
        min-height: 24px;
        padding: 0 8px;
        border-radius: 999px;
        border: 1px solid var(--success-line);
        background: var(--success-soft);
        color: var(--success);
        display: inline-flex;
        align-items: center;
        justify-content: center;
        font-size: 10px;
        font-weight: 800;
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .certificateActions {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 10px;
    }

    .certificateButton {
        appearance: none;
        min-height: 42px;
        padding: 0 12px;
        border-radius: 10px;
        border: 1px solid rgba(var(--accent-rgb), 0.3);
        background: rgba(var(--accent-rgb), 0.12);
        color: #dbeafe;
        cursor: pointer;
        font-size: 12px;
        font-weight: 800;
        transition:
            transform 140ms ease,
            border-color 140ms ease,
            background 140ms ease;
    }

    .certificateButton:hover:not(:disabled) {
        transform: translateY(-1px);
        border-color: rgba(var(--accent-rgb), 0.55);
        background: rgba(var(--accent-rgb), 0.2);
    }

    .certificateButton:disabled {
        cursor: not-allowed;
        opacity: 0.55;
    }

    .dangerButton {
        border-color: var(--danger-line);
        background: rgba(248, 113, 113, 0.1);
        color: #fecaca;
    }

    .dangerButton:hover:not(:disabled) {
        border-color: var(--danger);
        background: var(--danger-soft);
    }

    .certificateHint,
    .caMessage {
        color: var(--muted);
        font-size: 12px;
        line-height: 1.45;
        overflow-wrap: anywhere;
    }

    .caSuccess {
        color: var(--success);
    }

    .caError {
        color: #fecaca;
    }

    .confirmOverlay {
        position: fixed;
        inset: 0;
        z-index: 90;
        display: grid;
        place-items: center;
        padding: 28px;
        background: rgba(2, 6, 23, 0.68);
        backdrop-filter: blur(8px);
    }

    .confirmDialog {
        width: min(480px, 100%);
        display: grid;
        gap: 20px;
        padding: 22px;
        border-radius: 18px;
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
        line-height: 1.55;
    }

    .confirmActions {
        display: flex;
        justify-content: flex-end;
        gap: 10px;
        flex-wrap: wrap;
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
        font-weight: 700;
        cursor: pointer;
        transition:
            border-color 140ms ease,
            background 140ms ease,
            transform 140ms ease;
    }

    .confirmButton:hover:not(:disabled) {
        transform: translateY(-1px);
        border-color: var(--line-strong);
        background: var(--surface-elevated);
    }

    .confirmButton:disabled {
        cursor: not-allowed;
        opacity: 0.55;
    }

    .confirmButton.danger {
        border-color: var(--danger-line);
        background: rgba(248, 113, 113, 0.1);
        color: #fecaca;
    }

    .confirmButton.danger:hover:not(:disabled) {
        border-color: var(--danger);
        background: var(--danger-soft);
    }

    @media (max-width: 980px) {
        .heroCard,
        .formHeader,
        .contentGrid {
            flex-direction: column;
            align-items: stretch;
        }

        .contentGrid {
            display: flex;
        }

        .heroPills {
            justify-content: flex-start;
        }

        .statusColumn {
            position: static;
        }
    }

    @media (max-width: 760px) {
        .confirmOverlay {
            padding: 16px;
        }

        .confirmDialog {
            padding: 18px;
        }

        .confirmActions {
            display: grid;
            grid-template-columns: 1fr;
        }

        .heroCard,
        .formCard,
        .certificateCard {
            border-radius: 14px;
        }

        .fieldGrid {
            grid-template-columns: 1fr;
        }

        .upstreamHeader {
            flex-direction: column;
        }

        .upstreamFields {
            grid-template-columns: 1fr;
        }

        .certificateActions {
            grid-template-columns: 1fr;
        }
    }
</style>
