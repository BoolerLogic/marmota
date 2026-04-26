<script lang="ts">
    import { createEventDispatcher, onMount, tick } from "svelte";

    export let title = "Save request";
    export let submitLabel = "Save";
    export let initialName = "";

    const dispatch = createEventDispatcher<{
        cancel: void;
        save: { name: string };
    }>();

    let modalCard: HTMLFormElement | null = null;
    let draftName = initialName;

    onMount(() => {
        tick().then(() =>
            modalCard?.querySelector<HTMLInputElement>('input[type="text"]')?.focus(),
        );
    });

    function closeModal() {
        dispatch("cancel");
    }

    function handleBackdropClick(event: MouseEvent) {
        if (event.target === event.currentTarget) {
            closeModal();
        }
    }

    function handleBackdropKeydown(event: KeyboardEvent) {
        if (event.target !== event.currentTarget) return;

        if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            closeModal();
        }
    }

    function handleWindowKeydown(event: KeyboardEvent) {
        if (event.key === "Escape") {
            event.preventDefault();
            closeModal();
        }
    }

    function submitSave() {
        dispatch("save", {
            name: draftName.trim(),
        });
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
    <form
        bind:this={modalCard}
        class="modalCard"
        on:submit|preventDefault={submitSave}
        role="dialog"
        aria-modal="true"
        aria-labelledby="save-request-title"
    >
        <div class="modalHeader">
            <div class="headerCopy">
                <span class="eyebrow">Saved Requests</span>
                <h3 id="save-request-title">{title}</h3>
                <p class="headerSub">
                    You can leave the name empty. The copy will be kept only
                    during this session.
                </p>
            </div>

            <button
                type="button"
                class="ghostButton"
                on:click={closeModal}
                aria-label="Close save request dialog"
            >
                Close
            </button>
        </div>

        <label class="field">
            <span class="fieldLabel">Optional name</span>
            <input
                class="fieldInput"
                type="text"
                bind:value={draftName}
                placeholder="Example: Initial login"
            />
        </label>

        <div class="modalFooter">
            <span class="footerHint">
                If you leave it blank, it will be saved as an unnamed copy.
            </span>

            <div class="footerActions">
                <button type="button" class="ghostButton" on:click={closeModal}>
                    Cancel
                </button>
                <button type="submit" class="primaryButton">
                    {submitLabel}
                </button>
            </div>
        </div>
    </form>
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
        z-index: 80;
        display: grid;
        place-items: center;
        padding: 28px;
        background: rgba(2, 6, 23, 0.68);
        backdrop-filter: blur(8px);
    }

    .modalCard {
        width: min(560px, 100%);
        display: grid;
        gap: 18px;
        padding: 22px;
        border-radius: 18px;
        border: 1px solid var(--line);
        background: var(--surface-strong);
        box-shadow: var(--shadow-card);
    }

    .modalHeader,
    .modalFooter {
        display: flex;
        justify-content: space-between;
        gap: 16px;
        align-items: flex-start;
    }

    .headerCopy {
        display: flex;
        flex-direction: column;
        gap: 6px;
        min-width: 0;
    }

    .eyebrow,
    .fieldLabel,
    .footerHint {
        color: var(--muted);
    }

    .eyebrow,
    .fieldLabel {
        font-size: 11px;
        font-weight: 800;
        letter-spacing: 0.1em;
        text-transform: uppercase;
    }

    .headerSub {
        margin: 0;
        color: var(--muted);
        font-size: 13px;
        line-height: 1.5;
    }

    .field {
        display: grid;
        gap: 8px;
    }

    .fieldInput {
        min-height: 42px;
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

    .fieldInput:focus {
        border-color: rgba(var(--accent-rgb), 0.42);
        box-shadow: 0 0 0 4px rgba(var(--accent-rgb), 0.14);
    }

    .ghostButton,
    .primaryButton {
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

    .ghostButton:hover,
    .primaryButton:hover {
        transform: translateY(-1px);
    }

    .ghostButton:hover {
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.08);
    }

    .primaryButton {
        border-color: rgba(var(--accent-rgb), 0.72);
        background: var(--accent);
        color: #f8fafc;
    }

    .primaryButton:hover {
        border-color: var(--accent-strong);
        background: var(--accent-strong);
    }

    .footerHint {
        align-self: center;
        font-size: 12px;
        line-height: 1.45;
    }

    .footerActions {
        display: flex;
        gap: 10px;
        flex-wrap: wrap;
        justify-content: flex-end;
    }

    @media (max-width: 760px) {
        .modalOverlay {
            padding: 16px;
        }

        .modalCard {
            padding: 18px;
        }

        .modalHeader,
        .modalFooter {
            flex-direction: column;
            align-items: stretch;
        }

        .footerActions {
            justify-content: stretch;
        }
    }
</style>
