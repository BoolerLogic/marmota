<script lang="ts">
    import SearchHighlight from "@/shared/components/SearchHighlight.svelte";
    import SearchablePre from "@/shared/components/SearchablePre.svelte";
    import type { PrettyMultipartPart } from "@/features/http-history/utils/httpPretty";

    export let parts: PrettyMultipartPart[] = [];
    export let searchQuery = "";
</script>

<div class="multipartViewer">
    {#if parts.length === 0}
        <div class="multipartEmpty">No multipart parts</div>
    {:else}
        {#each parts as part, index (part.id)}
            <section class="partCard">
                <div class="partHeader">
                    <span class="partTitle">Part {index + 1}</span>
                    <span class={`partKind ${part.bodyKind}`}>
                        {part.bodyKind === "binary"
                            ? "Binary"
                            : part.bodyKind === "empty"
                              ? "Empty"
                              : "Text"}
                    </span>
                </div>

                <div class="partMeta">
                    {#if part.name}
                        <span class="partMetaItem">
                            <span class="partMetaLabel">Name</span>
                            <SearchHighlight
                                className="partMetaValue"
                                text={part.name}
                                query={searchQuery}
                            />
                        </span>
                    {/if}

                    {#if part.filename}
                        <span class="partMetaItem">
                            <span class="partMetaLabel">Filename</span>
                            <SearchHighlight
                                className="partMetaValue"
                                text={part.filename}
                                query={searchQuery}
                            />
                        </span>
                    {/if}

                    {#if part.contentType}
                        <span class="partMetaItem">
                            <span class="partMetaLabel">Content-Type</span>
                            <SearchHighlight
                                className="partMetaValue"
                                text={part.contentType}
                                query={searchQuery}
                            />
                        </span>
                    {/if}
                </div>

                {#if part.headers.length > 0}
                    <div class="partHeaders">
                        {#each part.headers as header (header.id)}
                            <div class="partHeaderLine">
                                <SearchHighlight
                                    className="partHeaderKey"
                                    text={header.key}
                                    query={searchQuery}
                                />
                                <span class="partHeaderColon">: </span>
                                <SearchHighlight
                                    className="partHeaderValue"
                                    text={header.value}
                                    query={searchQuery}
                                />
                            </div>
                        {/each}
                    </div>
                {/if}

                <SearchablePre
                    text={part.displayBody}
                    query={searchQuery}
                    emptyLabel="(no content)"
                />
            </section>
        {/each}
    {/if}
</div>

<style>
    .multipartViewer {
        display: grid;
        gap: 12px;
    }

    .multipartEmpty {
        color: var(--muted);
        font-size: 12px;
    }

    .partCard {
        display: grid;
        gap: 10px;
        padding: 12px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
    }

    .partHeader {
        display: flex;
        justify-content: space-between;
        gap: 10px;
        align-items: center;
    }

    .partTitle {
        color: var(--text);
        font-size: 12px;
        font-weight: 700;
        letter-spacing: 0.04em;
        text-transform: uppercase;
    }

    .partKind {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-height: 24px;
        padding: 0 8px;
        border-radius: 999px;
        border: 1px solid var(--line);
        background: var(--surface-soft);
        color: var(--muted-strong);
        font-size: 10px;
        font-weight: 700;
        letter-spacing: 0.06em;
        text-transform: uppercase;
    }

    .partKind.text {
        color: var(--success);
        border-color: var(--success-line);
        background: var(--success-soft);
    }

    .partKind.binary {
        color: var(--warning);
        border-color: var(--warning-line);
        background: var(--warning-soft);
    }

    .partMeta {
        display: grid;
        gap: 8px;
    }

    .partMetaItem {
        display: inline-flex;
        gap: 6px;
        align-items: center;
        padding: 6px 8px;
        border-radius: 999px;
        border: 1px solid var(--line);
        background: var(--field-bg);
        font-size: 11px;
    }

    .partMetaLabel {
        color: var(--muted);
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }

    :global(.partMetaValue) {
        color: var(--text);
    }

    .partHeaders {
        display: grid;
        gap: 4px;
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 12px;
        line-height: 1.5;
    }

    .partHeaderLine {
        padding-left: 0;
        white-space: pre-wrap;
        word-break: break-word;
    }

    :global(.partHeaderKey) {
        color: #93c5fd;
    }

    .partHeaderColon {
        color: var(--muted-strong);
    }

    :global(.partHeaderValue) {
        color: #cbd5e1;
    }
</style>
