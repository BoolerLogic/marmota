<script lang="ts">
    import { createEventDispatcher, onDestroy } from "svelte";
    import { copyTextToClipboard } from "@/shared/utils/clipboard";

    export let url = "";

    const dispatch = createEventDispatcher<{
        copyerror: void;
    }>();

    let copied = false;
    let resetTimer: ReturnType<typeof setTimeout> | null = null;

    async function copyUrl() {
        if (!url) return;

        try {
            await copyTextToClipboard(url);
            copied = true;
            if (resetTimer !== null) {
                clearTimeout(resetTimer);
            }
            resetTimer = setTimeout(() => {
                copied = false;
                resetTimer = null;
            }, 1600);
        } catch {
            dispatch("copyerror");
        }
    }

    onDestroy(() => {
        if (resetTimer !== null) {
            clearTimeout(resetTimer);
        }
    });
</script>

<div class="urlBar">
    <div class="urlCopy">
        <span class="urlLabel">Request URL</span>
        <span class="urlValue" title={url || "URL unavailable"}>
            {url || "URL unavailable"}
        </span>
    </div>

    <button
        type="button"
        class:copied
        class="copyButton"
        on:click={copyUrl}
        disabled={!url}
        aria-label={copied ? "Request URL copied" : "Copy request URL"}
        title={copied ? "Copied" : "Copy request URL"}
    >
        {#if copied}
            <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                <path d="m5 12 4 4L19 6" />
            </svg>
            <span>Copied</span>
        {:else}
            <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                <rect x="8" y="8" width="11" height="11" rx="2" />
                <path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2" />
            </svg>
            <span>Copy</span>
        {/if}
    </button>
</div>

<style>
    .urlBar {
        min-width: 0;
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 10px 12px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--field-bg);
    }

    .urlCopy {
        min-width: 0;
        flex: 1 1 auto;
        display: grid;
        gap: 4px;
    }

    .urlLabel {
        color: var(--muted);
        font-size: 10px;
        font-weight: 900;
        letter-spacing: 0.12em;
        text-transform: uppercase;
    }

    .urlValue {
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        color: #dbeafe;
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 12px;
        line-height: 1.45;
    }

    .copyButton {
        appearance: none;
        flex: 0 0 auto;
        min-height: 34px;
        display: inline-flex;
        align-items: center;
        gap: 7px;
        padding: 0 11px;
        border-radius: 9px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--text);
        font-size: 11px;
        font-weight: 800;
        cursor: pointer;
        outline: none;
    }

    .copyButton:hover:not(:disabled),
    .copyButton:focus-visible {
        border-color: rgba(var(--accent-rgb), 0.62);
        box-shadow: 0 0 0 4px rgba(var(--accent-rgb), 0.12);
    }

    .copyButton.copied {
        border-color: rgba(52, 211, 153, 0.46);
        color: #86efac;
    }

    .copyButton:disabled {
        opacity: 0.55;
        cursor: not-allowed;
    }

    .copyButton svg {
        width: 16px;
        height: 16px;
        fill: none;
        stroke: currentColor;
        stroke-width: 1.8;
        stroke-linecap: round;
        stroke-linejoin: round;
    }

    @media (max-width: 720px) {
        .urlBar {
            align-items: stretch;
            flex-direction: column;
        }

        .copyButton {
            justify-content: center;
        }
    }
</style>
