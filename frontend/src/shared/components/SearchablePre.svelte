<script lang="ts">
    import CodeBlockShell from "./CodeBlockShell.svelte";
    import SearchHighlight from "./SearchHighlight.svelte";

    export let text: string = "";
    export let query: string = "";
    export let emptyLabel: string = "(empty)";
    export let interactive = false;
    export let ariaLabel = "Text block";
</script>

<CodeBlockShell text={text} {emptyLabel} {interactive} {ariaLabel} let:lines>
    {#each lines as line, index (`code-line-${index}`)}
        <div class="searchablePreLine">
            <SearchHighlight text={line} {query} wrap={false} />
        </div>
    {/each}
</CodeBlockShell>

<style>
    .searchablePreLine {
        display: flex;
        align-items: flex-start;
        min-height: 1.5em;
        min-width: max-content;
    }
</style>
