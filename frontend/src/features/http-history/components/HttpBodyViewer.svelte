<script lang="ts">
    import HtmlRenderDialog from "./HtmlRenderDialog.svelte";
    import HtmlPrettyViewer from "./HtmlPrettyViewer.svelte";
    import JsonTreeNode from "./JsonTreeNode.svelte";
    import MultipartPrettyViewer from "./MultipartPrettyViewer.svelte";
    import SearchHighlight from "@/shared/components/SearchHighlight.svelte";
    import SearchablePre from "@/shared/components/SearchablePre.svelte";
    import {
        buildMultipartPrettySearchText,
        getHeaderValue,
        isHtmlContentType,
        isJsonContentType,
        normalizeContentType,
        parseFormUrlEncodedBody,
        parseMultipartBody,
        type PrettyFormEntry,
        type PrettyMultipartPart,
    } from "@/features/http-history/utils/httpPretty";
    import { countMatches } from "@/shared/utils/textSearch";

    type PrettyBody =
        | { kind: "json"; value: unknown; contentType: string }
        | {
              kind: "form";
              entries: PrettyFormEntry[];
              contentType: string;
          }
        | {
              kind: "multipart";
              parts: PrettyMultipartPart[];
              contentType: string;
          }
        | {
              kind: "html";
              html: string;
              contentType: string;
          }
        | { kind: "jsonError"; message: string; contentType: string };

    type ViewMode = "pretty" | "raw";

    export let headBlockStr: string = "";
    export let bodyStr: string = "";
    export let emptyLabel: string = "";
    export let searchQuery: string = "";
    export let matchCount: number = 0;
    export let allowHtmlRender: boolean = false;
    export let bodyIsEncoded: boolean = false;

    let activeView: ViewMode = "raw";
    let htmlPreviewOpen = false;
    let lastSignature = "";

    function resolvePrettyBody(
        rawHeaders: string,
        rawBody: string,
    ): PrettyBody | null {
        if (bodyIsEncoded) return null;
        if (rawBody.trim().length === 0) return null;

        const contentType = normalizeContentType(
            getHeaderValue(rawHeaders, "content-type"),
        );

        if (!contentType) return null;

        if (isJsonContentType(contentType)) {
            try {
                return {
                    kind: "json",
                    value: JSON.parse(rawBody),
                    contentType,
                };
            } catch (error: unknown) {
                return {
                    kind: "jsonError",
                    message:
                        error instanceof Error
                            ? error.message
                            : "JSON invalido",
                    contentType,
                };
            }
        }

        if (contentType === "application/x-www-form-urlencoded") {
            return {
                kind: "form",
                entries: parseFormUrlEncodedBody(rawBody),
                contentType,
            };
        }

        if (contentType.startsWith("multipart/")) {
            const parts = parseMultipartBody(
                getHeaderValue(rawHeaders, "content-type"),
                rawBody,
            );

            if (parts && parts.length > 0) {
                return {
                    kind: "multipart",
                    parts,
                    contentType,
                };
            }
        }

        if (isHtmlContentType(contentType)) {
            return {
                kind: "html",
                html: rawBody,
                contentType,
            };
        }

        return null;
    }

    function buildPrettySearchText(prettyBody: PrettyBody | null): string {
        if (!prettyBody) return "";

        if (prettyBody.kind === "json") {
            return JSON.stringify(prettyBody.value, null, 2) ?? "";
        }

        if (prettyBody.kind === "form") {
            return prettyBody.entries
                .map((entry) => `${entry.key}: ${entry.value}`)
                .join("\n");
        }

        if (prettyBody.kind === "multipart") {
            return buildMultipartPrettySearchText(prettyBody.parts);
        }

        if (prettyBody.kind === "html") {
            return prettyBody.html;
        }

        return prettyBody.message;
    }

    $: normalizedBody = bodyStr ?? "";
    $: prettyBody = resolvePrettyBody(headBlockStr, normalizedBody);
    $: hasPretty = prettyBody !== null;
    $: prettySearchText = buildPrettySearchText(prettyBody);
    $: displayedSearchText =
        activeView === "pretty" && prettyBody
            ? prettySearchText
            : normalizedBody;
    $: matchCount = countMatches(displayedSearchText, searchQuery);
    $: signature = `${headBlockStr}__${normalizedBody}__${bodyIsEncoded}__${allowHtmlRender}`;
    $: if (signature !== lastSignature) {
        lastSignature = signature;
        htmlPreviewOpen = false;
        activeView = hasPretty ? "pretty" : "raw";
    }
</script>

<div class="bodyViewer">
    {#if hasPretty}
        <div class="viewerTabs">
            <button
                type="button"
                class="viewerTab"
                class:active={activeView === "pretty"}
                on:click={() => (activeView = "pretty")}
            >
                Pretty
            </button>
            <button
                type="button"
                class="viewerTab"
                class:active={activeView === "raw"}
                on:click={() => (activeView = "raw")}
            >
                Raw
            </button>
            {#if allowHtmlRender && prettyBody?.kind === "html"}
                <button
                    type="button"
                    class="viewerTab renderTab"
                    aria-haspopup="dialog"
                    on:click={() => (htmlPreviewOpen = true)}
                >
                    Render
                </button>
            {/if}
        </div>
    {/if}

    {#if activeView === "pretty" && prettyBody}
        <div class="prettyPanel">
            <div class="prettyMeta">Content-Type: {prettyBody.contentType}</div>

            {#if prettyBody.kind === "json"}
                <div class="jsonTreeRoot">
                    <JsonTreeNode
                        value={prettyBody.value}
                        isRoot={true}
                        {searchQuery}
                        startLine={1}
                    />
                </div>
            {:else if prettyBody.kind === "form"}
                <div class="formList">
                    {#if prettyBody.entries.length === 0}
                        <div class="formRow empty">
                            <span>Empty body</span>
                        </div>
                    {:else}
                        {#each prettyBody.entries as entry (entry.id)}
                            <div class="formRow">
                                <SearchHighlight
                                    className="formKey"
                                    text={entry.key}
                                    query={searchQuery}
                                />
                                <SearchHighlight
                                    className="formValue"
                                    text={entry.value || '""'}
                                    query={searchQuery}
                                />
                            </div>
                        {/each}
                    {/if}
                </div>
            {:else if prettyBody.kind === "multipart"}
                <MultipartPrettyViewer parts={prettyBody.parts} {searchQuery} />
            {:else if prettyBody.kind === "html"}
                <HtmlPrettyViewer htmlText={prettyBody.html} {searchQuery} />
            {:else}
                <div class="prettyError">
                    <strong>Could not parse the JSON</strong>
                    <span>{prettyBody.message}</span>
                </div>
            {/if}
        </div>
    {:else}
        <SearchablePre
            text={normalizedBody}
            query={searchQuery}
            {emptyLabel}
            interactive={true}
            ariaLabel="Body raw"
        />
    {/if}
</div>

{#if htmlPreviewOpen && allowHtmlRender && prettyBody?.kind === "html"}
    <HtmlRenderDialog
        htmlText={prettyBody.html}
        on:close={() => (htmlPreviewOpen = false)}
    />
{/if}

<style>
    .bodyViewer {
        display: grid;
        gap: 10px;
    }

    .viewerTabs {
        display: inline-flex;
        gap: 8px;
    }

    .viewerTab {
        appearance: none;
        min-height: 32px;
        padding: 0 12px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--muted);
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        cursor: pointer;
        transition:
            border-color 140ms ease,
            background 140ms ease,
            color 140ms ease;
    }

    .viewerTab.active {
        color: var(--text);
        background: var(--accent-soft);
        border-color: var(--accent);
    }

    .viewerTab.renderTab {
        color: var(--text);
        border-color: rgba(var(--accent-rgb), 0.42);
        background: rgba(var(--accent-rgb), 0.1);
    }

    .viewerTab.renderTab:hover {
        border-color: var(--accent);
        background: var(--accent-soft);
    }

    .prettyPanel {
        display: grid;
        gap: 10px;
        padding: 12px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--field-bg);
    }

    .prettyMeta {
        color: var(--muted);
        font-size: 11px;
        font-weight: 800;
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .formList {
        counter-reset: pretty-form-line;
        display: grid;
        gap: 8px;
    }

    .formRow {
        position: relative;
        display: grid;
        grid-template-columns: minmax(0, 180px) minmax(0, 1fr);
        gap: 10px;
        align-items: start;
        padding: 10px 12px 10px 48px;
        border-radius: 10px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
    }

    .formRow::before {
        content: counter(pretty-form-line);
        counter-increment: pretty-form-line;
        position: absolute;
        top: 10px;
        left: 0;
        width: 36px;
        color: var(--muted);
        text-align: right;
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 12px;
        line-height: 1.5;
        user-select: none;
    }

    .formRow.empty {
        grid-template-columns: 1fr;
    }

    :global(.formKey) {
        color: var(--text);
        font-size: 12px;
        font-weight: 800;
        word-break: break-word;
    }

    :global(.formValue) {
        color: var(--info);
        font-size: 12px;
        word-break: break-word;
    }

    .prettyError {
        display: grid;
        gap: 6px;
        padding: 12px;
        border-radius: 10px;
        border: 1px solid var(--danger-line);
        background: var(--danger-soft);
        color: #fecaca;
    }

    @media (max-width: 760px) {
        .formRow {
            grid-template-columns: 1fr;
        }
    }
</style>
