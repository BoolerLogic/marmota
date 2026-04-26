<script lang="ts">
    import { createEventDispatcher } from "svelte";

    export let eyebrow = "Notice";
    export let title = "Attention";
    export let message = "";
    export let buttonLabel = "OK";

    const dispatch = createEventDispatcher<{
        close: void;
    }>();

    function closeDialog() {
        dispatch("close");
    }

    function handleBackdropClick(event: MouseEvent) {
        if (event.target === event.currentTarget) {
            closeDialog();
        }
    }

    function handleBackdropKeydown(event: KeyboardEvent) {
        if (event.target !== event.currentTarget) return;

        if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            closeDialog();
        }
    }

    function handleWindowKeydown(event: KeyboardEvent) {
        if (event.key === "Escape") {
            event.preventDefault();
            closeDialog();
        }
    }
</script>

<svelte:window on:keydown={handleWindowKeydown} />

<div
    class="modalOverlay"
    role="presentation"
    tabindex="-1"
    on:click={handleBackdropClick}
    on:keydown={handleBackdropKeydown}
>
    <div
        class="modalCard"
        role="dialog"
        aria-modal="true"
        aria-labelledby="alert-dialog-title"
    >
        <div class="headerCopy">
            <span class="eyebrow">{eyebrow}</span>
            <h3 id="alert-dialog-title">{title}</h3>
            <p class="messageCopy">{message}</p>
        </div>

        <div class="actionsRow">
            <button
                type="button"
                class="primaryButton"
                on:click={closeDialog}
            >
                {buttonLabel}
            </button>
        </div>
    </div>
</div>

<style>
    h3 {
        margin: 0;
        font-size: 22px;
        line-height: 1.1;
        letter-spacing: -0.03em;
        color: var(--text);
    }

    .modalOverlay {
        position: fixed;
        inset: 0;
        z-index: 90;
        display: grid;
        place-items: center;
        padding: 28px;
        background: rgba(2, 6, 23, 0.68);
        backdrop-filter: blur(8px);
    }

    .modalCard {
        width: min(480px, 100%);
        display: grid;
        gap: 20px;
        padding: 22px;
        border-radius: 18px;
        border: 1px solid var(--line);
        background: var(--surface-strong);
        box-shadow: var(--shadow-card);
    }

    .headerCopy {
        display: grid;
        gap: 8px;
    }

    .eyebrow {
        color: var(--muted);
        font-size: 11px;
        font-weight: 800;
        letter-spacing: 0.1em;
        text-transform: uppercase;
    }

    .messageCopy {
        margin: 0;
        color: var(--muted);
        font-size: 13px;
        line-height: 1.55;
    }

    .actionsRow {
        display: flex;
        justify-content: flex-end;
    }

    .primaryButton {
        appearance: none;
        min-height: 38px;
        padding: 0 14px;
        border-radius: 10px;
        border: 1px solid rgba(var(--accent-rgb), 0.72);
        background: var(--accent);
        color: #f8fafc;
        font-size: 12px;
        font-weight: 700;
        cursor: pointer;
        transition:
            border-color 140ms ease,
            background 140ms ease,
            transform 140ms ease;
    }

    .primaryButton:hover {
        transform: translateY(-1px);
        border-color: var(--accent-strong);
        background: var(--accent-strong);
    }

    @media (max-width: 760px) {
        .modalOverlay {
            padding: 16px;
        }

        .modalCard {
            padding: 18px;
        }

        .actionsRow {
            justify-content: stretch;
        }
    }
</style>
