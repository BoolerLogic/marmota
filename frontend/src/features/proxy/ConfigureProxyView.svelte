<script lang="ts">
    import AppAlertDialog from "@/shared/components/AppAlertDialog.svelte";
    import ProxyStatusCard from "./components/ProxyStatusCard.svelte";
    import type { ProxyMode, ProxyState } from "./utils/proxyUi";
    import { isValidIP, isValidPort, resolveListenIp } from "./utils/proxyUi";

    export let error: string | null = null;
    export let onStartProxy: (
        ip: string,
        port: number,
        skipServerCertVerify: boolean,
    ) => Promise<void> | void;
    export let onCloseProxy: () => Promise<void> | void;
    export let onExportCA: () => Promise<string> | string;
    export let onResetCA: () => Promise<void> | void;

    export let state: ProxyState;
    export let proxyMode: ProxyMode;
    export let specificIp: string;
    export let port: number;
    export let skipServerCertVerify: boolean = false;

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

    $: operationBusy = operationState !== "idle";
    $: locked = state === "loading" || state === "running" || operationBusy;
    $: exportBusy = operationState === "exporting";
    $: resetBusy = operationState === "resetting";
    $: startBusy = operationState === "starting";
    $: stopBusy = operationState === "stopping";
    $: resolvedIp = resolveListenIp(proxyMode, specificIp);
    $: ipValid = isValidIP(resolvedIp);
    $: portValid = isValidPort(port);
    $: canStart =
        ipValid &&
        portValid &&
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

    async function handleStart() {
        if (!canStart) return;
        if (port === null) return;

        operationState = "starting";
        try {
            await onStartProxy(resolvedIp, port, skipServerCertVerify);
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
    .formCard {
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

        .certificateActions {
            grid-template-columns: 1fr;
        }
    }
</style>
