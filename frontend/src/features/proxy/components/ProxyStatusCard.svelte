<script lang="ts">
    import type { ProxyState } from "../utils/proxyUi";

    export let resolvedIp: string;
    export let portValid: boolean;
    export let port: number;
    export let state: ProxyState;
    export let skipServerCertVerify: boolean = false;
    export let upstreamProxyEnabled: boolean = false;
    export let upstreamProxyHost: string = "";
    export let upstreamProxyPort: number = 1080;

    $: statusLabel =
        state === "running"
            ? "Listening"
            : state === "loading"
              ? "Loading"
              : state === "error"
                ? "Error"
                : "Not listening";
</script>

<div class="snapshotCard">
    <div class="cardTop">
        <span class="eyebrow">Live Status</span>
        <strong class={`stateBadge ${state}`}>{statusLabel}</strong>
    </div>

    <div class="snapshotRow">
        <span>Listen IP</span>
        <strong>{resolvedIp || "-"}</strong>
    </div>
    <div class="snapshotRow">
        <span>Listen Port</span>
        <strong>{portValid ? String(port) : "-"}</strong>
    </div>
    <div class="snapshotRow">
        <span>Server TLS Certificate Verification</span>
        <strong
            class={`tlsState ${skipServerCertVerify ? "relaxed" : "strict"}`}
            >{skipServerCertVerify ? "Skipped" : "Verified"}</strong
        >
    </div>
    <div class="snapshotRow">
        <span>Outbound Route</span>
        <strong class:upstream={upstreamProxyEnabled}>
            {upstreamProxyEnabled
                ? `SOCKS5 · ${upstreamProxyHost.trim() || "-"}:${upstreamProxyPort}`
                : "Direct connection"}
        </strong>
    </div>
</div>

<style>
    .snapshotCard {
        min-width: 280px;
        padding: 16px;
        border-radius: 16px;
        border: 1px solid var(--line);
        background: var(--surface-strong);
        display: grid;
        gap: 12px;
        box-shadow: var(--shadow-card);
    }

    .cardTop {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
        padding-bottom: 12px;
        border-bottom: 1px solid var(--line);
    }

    .eyebrow {
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.12em;
        text-transform: uppercase;
        color: var(--muted);
    }

    .stateBadge {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        padding: 7px 12px;
        border-radius: 999px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--text);
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .snapshotRow {
        display: flex;
        justify-content: space-between;
        gap: 12px;
        align-items: flex-start;
        color: var(--muted);
        font-size: 13px;
    }

    .snapshotRow strong {
        color: var(--text);
        text-align: right;
        max-width: 60%;
        word-break: break-word;
    }

    .tlsState.strict {
        color: var(--success);
    }

    .tlsState.relaxed {
        color: var(--warning);
    }

    .snapshotRow strong.upstream {
        color: var(--info);
    }

    @media (max-width: 980px) {
        .snapshotCard {
            width: 100%;
        }
    }

    .stateBadge.running {
        color: var(--success);
        background: var(--success-soft);
        border-color: var(--success-line);
    }

    .stateBadge.loading {
        color: var(--info);
        background: var(--info-soft);
        border-color: var(--info-line);
    }

    .stateBadge.error {
        color: var(--danger);
        background: var(--danger-soft);
        border-color: var(--danger-line);
    }

    .stateBadge.idle {
        color: var(--muted-strong);
        background: var(--neutral-soft);
        border-color: var(--neutral-line);
    }
</style>
