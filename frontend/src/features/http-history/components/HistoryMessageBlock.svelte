<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import HeadBlockViewer from "./HeadBlockViewer.svelte";
    import HttpBodyViewer from "./HttpBodyViewer.svelte";
    import HistorySearchForm from "./HistorySearchForm.svelte";

    export let label = "";
    export let kind: "head" | "body" = "head";
    export let text = "";
    export let headBlockStr = "";
    export let bodyStr = "";
    export let emptyLabel = "(empty)";
    export let searchQuery = "";
    export let searchInput = "";
    export let searchProgress = "0/0";
    export let hasContent = false;
    export let navigateDisabled = false;
    export let previousLabel = "Go to previous match";
    export let nextLabel = "Go to next match";
    export let matchCount = 0;
    export let containerElement: HTMLDivElement | null = null;

    const dispatch = createEventDispatcher<{
        submit: void;
        previous: void;
        next: void;
    }>();
</script>

<div class="messageBlock">
    <div class="messageBlockHeader">
        <div class="messageLabel">{label}</div>
    </div>

    <div bind:this={containerElement}>
        {#if kind === "body"}
            <HttpBodyViewer
                {headBlockStr}
                {bodyStr}
                {emptyLabel}
                searchQuery={searchQuery}
                bind:matchCount
            />
        {:else}
            <HeadBlockViewer text={text} query={searchQuery} {emptyLabel} />
        {/if}
    </div>

    {#if hasContent}
        <HistorySearchForm
            bind:value={searchInput}
            compact={true}
            placeholder="Search only in this block"
            progress={searchProgress}
            submitDisabled={false}
            {navigateDisabled}
            {previousLabel}
            {nextLabel}
            on:submit={() => dispatch("submit")}
            on:previous={() => dispatch("previous")}
            on:next={() => dispatch("next")}
        />
    {/if}
</div>

<style>
    .messageBlock {
        display: grid;
        gap: 10px;
    }

    .messageBlockHeader {
        display: flex;
        justify-content: space-between;
        gap: 10px;
        align-items: center;
    }

    .messageLabel {
        color: var(--muted);
        font-size: 11px;
        font-weight: 900;
        letter-spacing: 0.12em;
        text-transform: uppercase;
    }
</style>
