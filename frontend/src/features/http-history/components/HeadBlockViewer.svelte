<script lang="ts">
    import CodeBlockShell from "@/shared/components/CodeBlockShell.svelte";
    import SearchHighlight from "@/shared/components/SearchHighlight.svelte";

    type HeaderLine = {
        id: string;
        key: string | null;
        value: string;
    };

    export let text = "";
    export let query = "";
    export let emptyLabel = "(empty)";

    function normalizeHeadBlock(rawText: string): string {
        if (!rawText) return rawText;

        return rawText.replace(/(\r?\n)(\r?\n)$/, "$1");
    }

    function parseHeadBlock(rawText: string): {
        firstLine: string;
        headers: HeaderLine[];
    } {
        const lines = rawText.split(/\r?\n/);
        const [firstLine = "", ...headerLines] = lines;

        return {
            firstLine,
            headers: headerLines.map((line, index) => {
                const separatorIndex = line.indexOf(":");
                if (separatorIndex <= 0) {
                    return {
                        id: `header-${index}`,
                        key: null,
                        value: line,
                    };
                }

                return {
                    id: `header-${index}-${line.slice(0, separatorIndex).trim()}`,
                    key: line.slice(0, separatorIndex).trim(),
                    value: line.slice(separatorIndex + 1),
                };
            }),
        };
    }

    $: normalizedText = normalizeHeadBlock(text);
    $: displayText = normalizedText.trim().length > 0 ? normalizedText : emptyLabel;
    $: hasContent = normalizedText.trim().length > 0;
    $: parsed = parseHeadBlock(displayText);
</script>

<CodeBlockShell
    text={normalizedText}
    {emptyLabel}
    interactive={true}
    ariaLabel="Head block"
>
    {#if !hasContent}
        <div class="headLine">
            <SearchHighlight text={displayText} {query} wrap={false} />
        </div>
    {:else}
        <div class="headLine headStartLine">
            <SearchHighlight
                className="headStartText"
                text={parsed.firstLine}
                query={query}
                wrap={false}
            />
        </div>

        {#each parsed.headers as header (header.id)}
            <div class="headLine">
                {#if header.key !== null}
                    <SearchHighlight
                        className="headHeaderKey"
                        text={header.key}
                        query={query}
                        wrap={false}
                    />
                    <span class="headHeaderColon">:</span>
                    <SearchHighlight
                        className="headHeaderValue"
                        text={header.value}
                        query={query}
                        wrap={false}
                    />
                {:else}
                    <SearchHighlight
                        className="headHeaderValue"
                        text={header.value}
                        query={query}
                        wrap={false}
                    />
                {/if}
            </div>
        {/each}
    {/if}
</CodeBlockShell>

<style>
    .headLine {
        display: flex;
        align-items: flex-start;
        min-height: 1.5em;
        min-width: max-content;
    }

    :global(.headStartText) {
        color: #dbeafe;
        font-weight: 700;
    }

    :global(.headHeaderKey) {
        color: #93c5fd;
    }

    .headHeaderColon {
        color: var(--muted-strong);
        white-space: pre;
    }

    :global(.headHeaderValue) {
        color: #cbd5e1;
    }
</style>
