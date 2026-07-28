<script lang="ts">
    import { createEventDispatcher } from "svelte";

    export let value = "";
    export let placeholder = "";
    export let progress = "0/0";
    export let compact = false;
    export let singleLine = false;
    export let submitDisabled = false;
    export let navigateDisabled = false;
    export let previousLabel = "Go to previous match";
    export let nextLabel = "Go to next match";
    export let submitLabel = "Search";

    const dispatch = createEventDispatcher<{
        submit: void;
        previous: void;
        next: void;
    }>();

    function emitSubmit() {
        dispatch("submit");
    }
</script>

<form
    class="searchForm"
    class:compact
    class:singleLine
    on:submit|preventDefault={emitSubmit}
>
    <input
        class="searchInput"
        class:compact
        type="search"
        bind:value
        {placeholder}
        disabled={submitDisabled}
    />
    <button
        type="button"
        class="navButton"
        class:compact
        on:click={() => dispatch("previous")}
        disabled={navigateDisabled}
        aria-label={previousLabel}
    >
        &larr;
    </button>
    <span class="searchStatus" class:compact>{progress}</span>
    <button
        type="button"
        class="navButton"
        class:compact
        on:click={() => dispatch("next")}
        disabled={navigateDisabled}
        aria-label={nextLabel}
    >
        &rarr;
    </button>
    <button
        type="submit"
        class="searchButton"
        class:compact
        disabled={submitDisabled}
    >
        {submitLabel}
    </button>
</form>

<style>
    .searchForm {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: 8px;
        width: 100%;
    }

    .searchInput {
        flex: 1 1 220px;
        min-width: 0;
        min-height: 38px;
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

    .searchInput::placeholder {
        color: var(--muted);
    }

    .searchInput:focus {
        border-color: rgba(var(--accent-rgb), 0.42);
        box-shadow: 0 0 0 4px rgba(var(--accent-rgb), 0.18);
    }

    .searchInput.compact {
        min-height: 36px;
        font-size: 12px;
    }

    .searchButton {
        appearance: none;
        min-width: 88px;
        min-height: 38px;
        padding: 0 14px;
        border-radius: 10px;
        border: 1px solid rgba(var(--accent-rgb), 0.72);
        background: var(--accent);
        color: #eff6ff;
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        cursor: pointer;
        transition:
            transform 140ms ease,
            border-color 140ms ease,
            background 140ms ease;
    }

    .searchButton.compact {
        min-width: 78px;
        min-height: 34px;
    }

    .navButton {
        appearance: none;
        min-width: 38px;
        min-height: 38px;
        padding: 0;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--text);
        font-size: 15px;
        font-weight: 700;
        cursor: pointer;
        transition:
            transform 140ms ease,
            border-color 140ms ease,
            background 140ms ease,
            color 140ms ease;
    }

    .navButton.compact {
        min-width: 34px;
        min-height: 34px;
        border-radius: 8px;
        font-size: 14px;
    }

    .navButton:hover:not(:disabled) {
        transform: translateY(-1px);
        border-color: var(--accent);
        background: rgba(var(--accent-rgb), 0.08);
    }

    .searchButton:hover:not(:disabled) {
        transform: translateY(-1px);
        border-color: var(--accent-strong);
        background: var(--accent-strong);
    }

    .searchButton:disabled,
    .navButton:disabled,
    .searchInput:disabled {
        cursor: not-allowed;
        opacity: 0.6;
    }

    .searchStatus {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 92px;
        min-height: 38px;
        padding: 0 12px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--muted-strong);
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        white-space: nowrap;
    }

    .searchStatus.compact {
        min-width: 74px;
        min-height: 34px;
        padding: 0 10px;
        border-radius: 8px;
        font-size: 10px;
    }

    @media (max-width: 760px) {
        .searchForm.compact {
            align-items: stretch;
        }
    }

    @media (min-width: 761px) {
        .searchForm.singleLine {
            flex-wrap: nowrap;
        }
    }
</style>
