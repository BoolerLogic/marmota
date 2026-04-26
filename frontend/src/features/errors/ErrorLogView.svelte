<script lang="ts">
    import { clearErrors, errorStore } from "./state/errorStore";

    $: errorState = $errorStore;
</script>

<div class="errorView">
    <div class="heroCard">
        <div class="heroCopy">
            <span class="eyebrow">Diagnostics</span>
            <h2>Backend Error Log</h2>
            <p>
                Visual log of errors received from Go through the
                <code>error</code> event.
            </p>
        </div>

        <div class="heroActions">
            <div class="metaPill">Total: {errorState.entries.length}</div>
            <button
                type="button"
                class="clearButton"
                disabled={errorState.entries.length === 0}
                on:click={clearErrors}
            >
                Clear all
            </button>
        </div>
    </div>

    <section class="listCard">
        {#if errorState.entries.length === 0}
            <div class="emptyState">
                <div class="emptyTitle">No errors yet</div>
                <div class="emptySub">
                    When the backend emits <code>error</code> events, they will
                    appear here.
                </div>
            </div>
        {:else}
            <div class="errorList">
                {#each errorState.entries as entry (entry.id)}
                    <article class="errorItem">
                        <div class="errorMeta">
                            <span class="errorTime">{entry.timeLabel}</span>
                        </div>

                        <pre class="errorMessage">{entry.message}</pre>
                    </article>
                {/each}
            </div>
        {/if}
    </section>
</div>

<style>
    h2,
    p {
        margin: 0;
    }

    h2 {
        font-size: clamp(24px, 3vw, 30px);
        line-height: 1.1;
        letter-spacing: -0.03em;
        color: var(--text);
    }

    p {
        color: var(--muted);
        line-height: 1.6;
    }

    code {
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 0.95em;
    }

    .errorView {
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    .heroCard,
    .listCard {
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
        max-width: 760px;
    }

    .eyebrow {
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.12em;
        text-transform: uppercase;
        color: var(--muted);
    }

    .heroActions {
        display: flex;
        flex-wrap: wrap;
        gap: 10px;
        align-items: center;
        justify-content: flex-end;
    }

    .metaPill {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-height: 40px;
        padding: 0 12px;
        border-radius: 999px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--muted-strong);
        font-size: 12px;
        font-weight: 700;
    }

    .clearButton {
        appearance: none;
        min-height: 40px;
        padding: 0 14px;
        border-radius: 10px;
        border: 1px solid var(--danger-line);
        background: var(--danger-soft);
        color: #fecaca;
        font-weight: 700;
        cursor: pointer;
        transition:
            transform 140ms ease,
            border-color 140ms ease,
            background 140ms ease;
    }

    .clearButton:hover:not(:disabled) {
        transform: translateY(-1px);
        border-color: rgba(248, 113, 113, 0.46);
        background: rgba(248, 113, 113, 0.18);
    }

    .clearButton:disabled {
        cursor: not-allowed;
        opacity: 0.55;
    }

    .listCard {
        padding: 16px;
    }

    .emptyState {
        border-radius: 12px;
        border: 1px dashed var(--line-strong);
        background: var(--surface-muted);
        padding: 24px 20px;
    }

    .emptyTitle {
        color: var(--text);
        font-size: 16px;
        font-weight: 700;
    }

    .emptySub {
        margin-top: 8px;
        color: var(--muted);
        line-height: 1.5;
    }

    .errorList {
        display: grid;
        gap: 12px;
    }

    .errorItem {
        display: grid;
        gap: 10px;
        padding: 14px;
        border-radius: 12px;
        border: 1px solid var(--danger-line);
        background: rgba(248, 113, 113, 0.06);
        box-shadow: inset 3px 0 0 var(--danger);
    }

    .errorMeta {
        display: flex;
        justify-content: flex-start;
        gap: 10px;
        align-items: center;
    }

    .errorTime {
        color: var(--muted-strong);
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .errorMessage {
        margin: 0;
        white-space: pre-wrap;
        word-break: break-word;
        color: var(--text);
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 12px;
        line-height: 1.55;
    }

    @media (max-width: 760px) {
        .heroCard {
            flex-direction: column;
        }

        .heroActions,
        .errorMeta {
            justify-content: flex-start;
        }
    }
</style>
