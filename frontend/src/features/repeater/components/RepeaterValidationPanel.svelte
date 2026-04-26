<script lang="ts">
    import type { RepeaterValidationIssue } from "../utils/repeaterRequest";

    export let errors: RepeaterValidationIssue[] = [];
    export let warnings: RepeaterValidationIssue[] = [];
</script>

<div class="validationPanel">
    <div class="validationHeader">
        <span>Validation</span>
        <span class="validationMeta">
            {errors.length} errors | {warnings.length} warnings
        </span>
    </div>

    {#if errors.length === 0 && warnings.length === 0}
        <div class="validationEmpty">
            The request has no issues detected by the frontend validator.
        </div>
    {:else}
        <div class="issueList">
            {#each errors as issue (issue.id)}
                <div class="issueRow error">
                    <span class="issueBadge">Error</span>
                    <span class="issueText">
                        {#if issue.line !== null}
                            Line {issue.line}: 
                        {/if}
                        {issue.message}
                    </span>
                </div>
            {/each}

            {#each warnings as issue (issue.id)}
                <div class="issueRow warning">
                    <span class="issueBadge">Warning</span>
                    <span class="issueText">
                        {#if issue.line !== null}
                            Line {issue.line}: 
                        {/if}
                        {issue.message}
                    </span>
                </div>
            {/each}
        </div>
    {/if}
</div>

<style>
    .validationPanel {
        display: grid;
        gap: 10px;
        padding: 14px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
    }

    .validationHeader {
        display: flex;
        justify-content: space-between;
        gap: 10px;
        align-items: center;
        color: var(--muted);
        font-size: 11px;
        font-weight: 800;
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .validationMeta {
        color: var(--muted-strong);
    }

    .validationEmpty {
        color: var(--muted);
        font-size: 12px;
        line-height: 1.55;
    }

    .issueList {
        display: grid;
        gap: 8px;
    }

    .issueRow {
        display: flex;
        gap: 10px;
        align-items: flex-start;
        padding: 10px 12px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--field-bg);
    }

    .issueRow.error {
        border-color: var(--danger-line);
        background: var(--danger-soft);
    }

    .issueRow.warning {
        border-color: var(--warning-line);
        background: var(--warning-soft);
    }

    .issueBadge {
        flex: 0 0 auto;
        min-width: 54px;
        color: var(--text);
        font-size: 10px;
        font-weight: 900;
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .issueText {
        color: var(--text);
        font-size: 12px;
        line-height: 1.5;
    }
</style>
