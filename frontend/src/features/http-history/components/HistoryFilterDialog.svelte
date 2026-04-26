<script lang="ts">
    import { createEventDispatcher, onMount, tick } from "svelte";
    import {
        HISTORY_FILTER_MATCH_MODE_OPTIONS,
        HISTORY_FILTER_TARGET_OPTIONS,
        cloneHistoryFilterConditions,
        createHistoryFilterCondition,
        sanitizeHistoryFilterConditions,
        type HistoryFilterCondition,
        type HistoryFilterMatchMode,
        type HistoryFilterOperator,
        type HistoryFilterTarget,
    } from "@/features/http-history/utils/historyFilters";

    export let title = "Configure filter";
    export let submitLabel = "Save filter";
    export let initialConditions: HistoryFilterCondition[] = [];
    export let initialOperator: HistoryFilterOperator = "and";

    const dispatch = createEventDispatcher<{
        cancel: void;
        save: {
            conditions: HistoryFilterCondition[];
            operator: HistoryFilterOperator;
        };
    }>();

    let modalCard: HTMLFormElement | null = null;
    let draftConditions = cloneHistoryFilterConditions(initialConditions);
    let draftOperator: HistoryFilterOperator = initialOperator;

    $: validConditionCount = sanitizeHistoryFilterConditions(draftConditions).length;

    onMount(() => {
        tick().then(() =>
            modalCard?.querySelector<HTMLInputElement>('input[type="search"]')?.focus(),
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
        if (event.key === "Escape") {
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

    function addCondition() {
        draftConditions = [...draftConditions, createHistoryFilterCondition()];
    }

    function removeCondition(id: string) {
        if (draftConditions.length <= 1) {
            draftConditions = [createHistoryFilterCondition()];
            return;
        }

        draftConditions = draftConditions.filter((condition) => condition.id !== id);
    }

    function updateConditionQuery(id: string, query: string) {
        draftConditions = draftConditions.map((condition) =>
            condition.id === id ? { ...condition, query } : condition,
        );
    }

    function updateConditionTarget(id: string, target: HistoryFilterTarget) {
        draftConditions = draftConditions.map((condition) =>
            condition.id === id ? { ...condition, target } : condition,
        );
    }

    function updateConditionMatchMode(
        id: string,
        matchMode: HistoryFilterMatchMode,
    ) {
        draftConditions = draftConditions.map((condition) =>
            condition.id === id ? { ...condition, matchMode } : condition,
        );
    }

    function handleConditionQueryInput(id: string, event: Event) {
        updateConditionQuery(id, (event.currentTarget as HTMLInputElement).value);
    }

    function handleConditionTargetChange(id: string, event: Event) {
        updateConditionTarget(
            id,
            (event.currentTarget as HTMLSelectElement).value as HistoryFilterTarget,
        );
    }

    function handleConditionMatchModeChange(id: string, event: Event) {
        updateConditionMatchMode(
            id,
            (event.currentTarget as HTMLSelectElement)
                .value as HistoryFilterMatchMode,
        );
    }

    function submitFilter() {
        const conditions = sanitizeHistoryFilterConditions(draftConditions);
        if (conditions.length === 0) return;

        dispatch("save", {
            conditions,
            operator: draftOperator,
        });
    }
</script>

<svelte:window on:keydown={handleWindowKeydown} />

<div
    class="modalOverlay"
    on:click={handleBackdropClick}
    on:keydown={handleBackdropKeydown}
>
    <form
        bind:this={modalCard}
        class="modalCard"
        on:submit|preventDefault={submitFilter}
        role="dialog"
        aria-modal="true"
        aria-labelledby="history-filter-title"
    >
        <div class="modalHeader">
            <div class="headerCopy">
                <span class="eyebrow">Filter Builder</span>
                <h3 id="history-filter-title">{title}</h3>
                <p class="headerSub">
                    Define one or more conditions to generate this view.
                </p>
            </div>

            <button
                type="button"
                class="ghostButton"
                on:click={closeModal}
                aria-label="Close filter settings"
            >
                Close
            </button>
        </div>

        <div class="modalBody">
            {#each draftConditions as condition, index (condition.id)}
                <div class="conditionCard">
                    <div class="conditionIndex">Condition {index + 1}</div>

                    <div class="conditionGrid">
                        <label class="field">
                            <span class="fieldLabel">Text</span>
                            <input
                                class="fieldInput"
                                type="search"
                                value={condition.query}
                                placeholder="Word or text to search"
                                on:input={(event) =>
                                    handleConditionQueryInput(
                                        condition.id,
                                        event,
                                    )}
                            />
                        </label>

                        <label class="field">
                            <span class="fieldLabel">Comparison</span>
                            <select
                                class="fieldInput fieldSelect"
                                value={condition.matchMode}
                                on:change={(event) =>
                                    handleConditionMatchModeChange(
                                        condition.id,
                                        event,
                                    )}
                            >
                                {#each HISTORY_FILTER_MATCH_MODE_OPTIONS as option (option.value)}
                                    <option value={option.value}>
                                        {option.label}
                                    </option>
                                {/each}
                            </select>
                        </label>

                        <label class="field">
                            <span class="fieldLabel">Search in</span>
                            <select
                                class="fieldInput fieldSelect"
                                value={condition.target}
                                on:change={(event) =>
                                    handleConditionTargetChange(
                                        condition.id,
                                        event,
                                    )}
                            >
                                {#each HISTORY_FILTER_TARGET_OPTIONS as option (option.value)}
                                    <option value={option.value}>
                                        {option.label}
                                    </option>
                                {/each}
                            </select>
                        </label>

                        <button
                            type="button"
                            class="dangerButton"
                            on:click={() => removeCondition(condition.id)}
                        >
                            Remove
                        </button>
                    </div>
                </div>
            {/each}

            <button type="button" class="ghostButton" on:click={addCondition}>
                Add condition
            </button>

            {#if draftConditions.length > 1}
                <div class="operatorCard">
                    <div class="operatorCopy">
                        <span class="fieldLabel">Combine conditions</span>
                        <span class="operatorHint">
                            Choose whether all conditions must match or just one.
                        </span>
                    </div>

                    <div class="operatorGroup" role="group" aria-label="Combination mode">
                        <button
                            type="button"
                            class="operatorButton"
                            class:active={draftOperator === "and"}
                            on:click={() => (draftOperator = "and")}
                        >
                            AND
                        </button>
                        <button
                            type="button"
                            class="operatorButton"
                            class:active={draftOperator === "or"}
                            on:click={() => (draftOperator = "or")}
                        >
                            OR
                        </button>
                    </div>
                </div>
            {/if}
        </div>

        <div class="modalFooter">
            <span class="footerHint">
                {validConditionCount} valid condition{validConditionCount === 1
                    ? ""
                    : "s"}
            </span>

            <div class="footerActions">
                <button type="button" class="ghostButton" on:click={closeModal}>
                    Cancel
                </button>
                <button
                    type="submit"
                    class="primaryButton"
                    disabled={validConditionCount === 0}
                >
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
        width: min(860px, 100%);
        max-height: min(84vh, 920px);
        overflow: auto;
        display: flex;
        flex-direction: column;
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

    .headerCopy,
    .operatorCopy {
        display: flex;
        flex-direction: column;
        gap: 6px;
        min-width: 0;
    }

    .eyebrow,
    .fieldLabel,
    .conditionIndex {
        font-size: 11px;
        font-weight: 800;
        letter-spacing: 0.1em;
        text-transform: uppercase;
    }

    .eyebrow,
    .fieldLabel,
    .conditionIndex,
    .footerHint {
        color: var(--muted);
    }

    .headerSub,
    .operatorHint {
        margin: 0;
        color: var(--muted);
        font-size: 13px;
        line-height: 1.5;
    }

    .modalBody {
        display: grid;
        gap: 14px;
    }

    .conditionCard,
    .operatorCard {
        display: grid;
        gap: 12px;
        padding: 16px;
        border-radius: 14px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
    }

    .conditionGrid {
        display: grid;
        grid-template-columns:
            minmax(0, 1.35fr)
            minmax(180px, 0.9fr)
            minmax(220px, 1fr)
            auto;
        gap: 12px;
        align-items: end;
    }

    .field {
        display: grid;
        gap: 8px;
        min-width: 0;
    }

    .fieldInput {
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

    .fieldInput:focus {
        border-color: rgba(var(--accent-rgb), 0.42);
        box-shadow: 0 0 0 4px rgba(var(--accent-rgb), 0.14);
    }

    .fieldSelect {
        appearance: none;
    }

    .ghostButton,
    .primaryButton,
    .dangerButton,
    .operatorButton {
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
    .primaryButton:hover,
    .dangerButton:hover,
    .operatorButton:hover {
        transform: translateY(-1px);
    }

    .ghostButton:hover,
    .operatorButton:hover {
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.08);
    }

    .primaryButton {
        min-width: 130px;
        border-color: rgba(var(--accent-rgb), 0.72);
        background: var(--accent);
        color: #f8fafc;
    }

    .primaryButton:hover {
        border-color: var(--accent-strong);
        background: var(--accent-strong);
    }

    .primaryButton:disabled {
        cursor: not-allowed;
        opacity: 0.6;
        transform: none;
    }

    .dangerButton {
        border-color: var(--danger-line);
        color: var(--danger);
    }

    .dangerButton:hover {
        background: var(--danger-soft);
    }

    .operatorCard {
        grid-template-columns: minmax(0, 1fr) auto;
        align-items: center;
    }

    .operatorGroup,
    .footerActions {
        display: flex;
        gap: 10px;
        flex-wrap: wrap;
        justify-content: flex-end;
    }

    .operatorButton.active {
        border-color: rgba(var(--accent-rgb), 0.72);
        background: rgba(var(--accent-rgb), 0.14);
        color: var(--accent-strong);
    }

    .footerHint {
        align-self: center;
        font-size: 12px;
        line-height: 1.4;
    }

    @media (max-width: 760px) {
        .modalOverlay {
            padding: 16px;
        }

        .modalCard {
            padding: 18px;
        }

        .modalHeader,
        .modalFooter,
        .operatorCard {
            flex-direction: column;
            align-items: stretch;
        }

        .conditionGrid,
        .operatorCard {
            grid-template-columns: 1fr;
        }

        .footerActions,
        .operatorGroup {
            justify-content: stretch;
        }
    }
</style>
