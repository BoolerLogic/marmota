<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import {
        REQUEST_COPY_OPTIONS,
        type RequestCopyTarget,
    } from "@/shared/utils/requestCopy";

    export let disabled = false;

    const dispatch = createEventDispatcher<{
        select: {
            target: RequestCopyTarget;
        };
    }>();

    function handleSelect(target: RequestCopyTarget) {
        if (disabled) return;
        dispatch("select", { target });
    }
</script>

<div class="submenuGroup" class:disabled>
    <button
        type="button"
        class="contextMenuItem submenuTrigger"
        disabled={disabled}
    >
        <span>Copy</span>
        <span class="submenuChevron" aria-hidden="true">›</span>
    </button>

    {#if !disabled}
        <div class="submenuPanel" role="menu" aria-label="Copy options">
            {#each REQUEST_COPY_OPTIONS as option (option.id)}
                <button
                    type="button"
                    class="submenuItem"
                    title={option.description}
                    on:click={() => handleSelect(option.id)}
                >
                    {option.label}
                </button>
            {/each}
        </div>
    {/if}
</div>

<style>
    .submenuGroup {
        position: relative;
    }

    .contextMenuItem,
    .submenuItem {
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
            background 140ms ease,
            color 140ms ease;
    }

    .submenuTrigger {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
    }

    .submenuPanel {
        position: absolute;
        top: -8px;
        left: calc(100% - 4px);
        z-index: 1;
        min-width: 280px;
        padding: 8px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--surface-strong);
        box-shadow: var(--shadow-card);
        display: none;
    }

    .submenuGroup:hover .submenuPanel,
    .submenuGroup:focus-within .submenuPanel {
        display: grid;
        gap: 4px;
    }

    .contextMenuItem:hover:not(:disabled),
    .submenuItem:hover:not(:disabled),
    .contextMenuItem:focus-visible:not(:disabled),
    .submenuItem:focus-visible:not(:disabled) {
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.08);
    }

    .submenuChevron {
        color: var(--muted);
        font-size: 14px;
        line-height: 1;
    }

    .disabled .submenuChevron {
        opacity: 0.55;
    }

    .contextMenuItem:disabled {
        cursor: not-allowed;
        opacity: 0.55;
    }
</style>
