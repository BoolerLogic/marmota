<script lang="ts">
    import SearchHighlight from "@/shared/components/SearchHighlight.svelte";

    export let value: unknown;
    export let propertyName: string | null = null;
    export let level: number = 0;
    export let isRoot: boolean = false;
    export let trailingComma: boolean = false;
    export let searchQuery: string = "";
    export let startLine: number = 1;

    type ChildEntry = {
        id: string;
        propertyName: string | null;
        value: unknown;
        trailingComma: boolean;
        startLine: number;
        lineSpan: number;
    };

    let isOpen = false;
    let openStateKey = "";

    function resolveNodeLineSpan(currentValue: unknown): number {
        if (Array.isArray(currentValue)) {
            if (currentValue.length === 0) return 1;

            return (
                2 +
                currentValue.reduce(
                    (total, entry) => total + resolveNodeLineSpan(entry),
                    0,
                )
            );
        }

        if (currentValue !== null && typeof currentValue === "object") {
            const entries = Object.values(
                currentValue as Record<string, unknown>,
            ) as unknown[];

            if (entries.length === 0) return 1;

            return (
                2 +
                entries.reduce<number>(
                    (total, entry) => total + resolveNodeLineSpan(entry),
                    0,
                )
            );
        }

        return 1;
    }

    function resolveChildEntries(
        currentValue: unknown,
        parentStartLine: number,
    ): ChildEntry[] {
        let nextStartLine = parentStartLine + 1;

        if (Array.isArray(currentValue)) {
            return currentValue.map((entry, index, source) => {
                const lineSpan = resolveNodeLineSpan(entry);
                const childEntry = {
                    id: `idx-${index}`,
                    propertyName: null,
                    value: entry,
                    trailingComma: index < source.length - 1,
                    startLine: nextStartLine,
                    lineSpan,
                };

                nextStartLine += lineSpan;
                return childEntry;
            });
        }

        if (currentValue !== null && typeof currentValue === "object") {
            return Object.entries(currentValue as Record<string, unknown>).map(
                ([entryKey, entryValue], index, source) => {
                    const lineSpan = resolveNodeLineSpan(entryValue);
                    const childEntry = {
                        id: entryKey,
                        propertyName: entryKey,
                        value: entryValue,
                        trailingComma: index < source.length - 1,
                        startLine: nextStartLine,
                        lineSpan,
                    };

                    nextStartLine += lineSpan;
                    return childEntry;
                },
            );
        }

        return [];
    }

    function formatPropertyName(name: string): string {
        return JSON.stringify(name);
    }

    function formatPrimitive(currentValue: unknown): string {
        const formatted = JSON.stringify(currentValue);
        return formatted ?? String(currentValue);
    }

    function getValueClass(currentValue: unknown): string {
        if (currentValue === null) return "null";
        return typeof currentValue;
    }

    function toggleNode() {
        isOpen = !isOpen;
    }

    function handleToggleKeydown(event: KeyboardEvent) {
        if (event.key !== "Enter" && event.key !== " ") return;

        event.preventDefault();
        toggleNode();
    }

    $: isArrayValue = Array.isArray(value);
    $: isObjectValue =
        value !== null && typeof value === "object" && !isArrayValue;
    $: openToken = isArrayValue ? "[" : "{";
    $: closeToken = isArrayValue ? "]" : "}";
    $: lineSpan = resolveNodeLineSpan(value);
    $: childEntries = resolveChildEntries(value, startLine);
    $: hasChildren = childEntries.length > 0;
    $: initialOpen = isRoot || level <= 1;
    $: collapsedLabel = isArrayValue
        ? `${childEntries.length} item${childEntries.length === 1 ? "" : "s"}`
        : `${childEntries.length} key${childEntries.length === 1 ? "" : "s"}`;
    $: closingLine = startLine + lineSpan - 1;
    $: nextOpenStateKey = [
        propertyName ?? "__root__",
        level,
        startLine,
        isArrayValue ? "array" : isObjectValue ? "object" : typeof value,
        childEntries.length,
    ].join("|");
    $: if (nextOpenStateKey !== openStateKey) {
        openStateKey = nextOpenStateKey;
        isOpen = initialOpen;
    }
</script>

{#if (isArrayValue || isObjectValue) && hasChildren}
    <div class="jsonNode" class:isOpen={isOpen}>
        <div class="jsonRow">
            <div class="jsonLineNumber" aria-hidden="true">{startLine}</div>

            <div class="jsonRowContent" style={`--json-level: ${level};`}>
                <span class="jsonIndent" aria-hidden="true"></span>

                <button
                    type="button"
                    class="jsonToggleButton"
                    class:isOpen={isOpen}
                    aria-expanded={isOpen}
                    aria-label={isOpen ? "Collapse JSON node" : "Expand JSON node"}
                    on:click={toggleNode}
                    on:keydown={handleToggleKeydown}
                >
                    <span class="jsonToggle" aria-hidden="true"></span>
                </button>

                <div class="jsonCode">
                    {#if propertyName !== null}
                        <span class="jsonKey">
                            <SearchHighlight
                                text={formatPropertyName(propertyName)}
                                query={searchQuery}
                            />
                        </span>
                        <span class="jsonColon">: </span>
                    {/if}

                    <span class="jsonBrace">{openToken}</span>

                    {#if !isOpen}
                        <span class="jsonCollapsed">
                            <span class="jsonEllipsis">...</span>
                            <span class="jsonMeta">{collapsedLabel}</span>
                            <span class="jsonBrace">{closeToken}</span>
                            {#if trailingComma}
                                <span class="jsonComma">,</span>
                            {/if}
                        </span>
                    {/if}
                </div>
            </div>
        </div>

        <div class="jsonChildren" aria-hidden={!isOpen}>
            {#each childEntries as entry (entry.id)}
                <svelte:self
                    value={entry.value}
                    propertyName={entry.propertyName}
                    level={level + 1}
                    trailingComma={entry.trailingComma}
                    searchQuery={searchQuery}
                    startLine={entry.startLine}
                />
            {/each}

            <div class="jsonRow">
                <div class="jsonLineNumber" aria-hidden="true">
                    {closingLine}
                </div>

                <div class="jsonRowContent" style={`--json-level: ${level};`}>
                    <span class="jsonIndent" aria-hidden="true"></span>
                    <span class="jsonToggleSlot" aria-hidden="true"></span>

                    <div class="jsonCode">
                        <span class="jsonBrace">{closeToken}</span>
                        {#if trailingComma}
                            <span class="jsonComma">,</span>
                        {/if}
                    </div>
                </div>
            </div>
        </div>
    </div>
{:else}
    <div class="jsonRow">
        <div class="jsonLineNumber" aria-hidden="true">{startLine}</div>

        <div class="jsonRowContent" style={`--json-level: ${level};`}>
            <span class="jsonIndent" aria-hidden="true"></span>
            <span class="jsonToggleSlot" aria-hidden="true"></span>

            <div class="jsonCode">
                {#if propertyName !== null}
                    <span class="jsonKey">
                        <SearchHighlight
                            text={formatPropertyName(propertyName)}
                            query={searchQuery}
                        />
                    </span>
                    <span class="jsonColon">: </span>
                {/if}

                {#if isArrayValue || isObjectValue}
                    <span class="jsonBrace">{openToken}{closeToken}</span>
                {:else}
                    <span class={`jsonValue ${getValueClass(value)}`}>
                        <SearchHighlight
                            text={formatPrimitive(value)}
                            query={searchQuery}
                        />
                    </span>
                {/if}

                {#if trailingComma}
                    <span class="jsonComma">,</span>
                {/if}
            </div>
        </div>
    </div>
{/if}

<style>
    .jsonNode,
    .jsonChildren,
    .jsonRow {
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 12px;
        line-height: 1.6;
    }

    .jsonNode {
        display: block;
    }

    .jsonChildren {
        display: grid;
        gap: 2px;
    }

    .jsonRow {
        display: grid;
        grid-template-columns: 36px minmax(0, 1fr);
        column-gap: 8px;
        align-items: start;
    }

    .jsonLineNumber {
        color: var(--muted);
        text-align: right;
        user-select: none;
    }

    .jsonRowContent {
        display: grid;
        grid-template-columns: calc(var(--json-level, 0) * 18px) 18px minmax(0, 1fr);
        align-items: start;
        min-width: 0;
        color: var(--text);
    }

    .jsonIndent,
    .jsonToggleSlot {
        min-width: 0;
        min-height: 1.6em;
    }

    .jsonToggleButton {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 18px;
        min-height: 1.6em;
        padding: 0;
        border: none;
        background: transparent;
        color: inherit;
        cursor: pointer;
    }

    .jsonToggleButton:focus-visible {
        outline: 1px solid rgba(var(--accent-rgb), 0.55);
        outline-offset: 1px;
        border-radius: 4px;
    }

    .jsonToggle {
        display: inline-block;
        width: 0;
        height: 0;
        border-left: 5px solid rgba(148, 163, 184, 0.9);
        border-top: 4px solid transparent;
        border-bottom: 4px solid transparent;
        transform-origin: center;
        transition: transform 140ms ease;
    }

    .jsonToggleButton.isOpen .jsonToggle {
        transform: rotate(90deg);
    }

    .jsonCode {
        min-width: 0;
        color: var(--text);
    }

    .jsonCollapsed {
        display: inline;
    }

    .jsonNode:not(.isOpen) > .jsonChildren {
        display: none;
    }

    .jsonKey {
        color: #7dd3fc;
    }

    .jsonColon,
    .jsonComma,
    .jsonBrace {
        color: #cbd5e1;
    }

    .jsonValue.string {
        color: #86efac;
    }

    .jsonValue.number {
        color: #fbbf24;
    }

    .jsonValue.boolean {
        color: #c084fc;
    }

    .jsonValue.null {
        color: #94a3b8;
    }

    .jsonMeta,
    .jsonEllipsis {
        color: var(--muted);
    }
</style>
