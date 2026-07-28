<script lang="ts">
    import HeadBlockViewer from "@/features/http-history/components/HeadBlockViewer.svelte";
    import HttpBodyViewer from "@/features/http-history/components/HttpBodyViewer.svelte";
    import type { RepeaterTabResponse } from "../state/repeaterStore";

    export let response: RepeaterTabResponse | null = null;
    export let requestState: "idle" | "sending" | "success" | "error" = "idle";
    export let requestError: string | null = null;

    function buildResponseLine(value: RepeaterTabResponse): string {
        const parts = [
            value.version.trim(),
            value.statusCode === null ? "" : String(value.statusCode),
        ].filter(Boolean);

        return parts.length > 0 ? parts.join(" ") : "Response received";
    }
</script>

<section class="responsePanel">
    <div class="responseHeader">
        <div>
            <span class="eyebrow">Response</span>
            <h3>Result</h3>
        </div>

        {#if response}
            <div class="responseMeta">
                <span class="metaPill">{buildResponseLine(response)}</span>
                {#if response.durationMs !== null}
                    <span class="metaPill">{response.durationMs} ms</span>
                {/if}
            </div>
        {/if}
    </div>

    {#if requestState === "sending"}
        <div class="emptyState">
            <div class="emptyTitle">Sending request...</div>
            <div class="emptySub">Waiting for the backend response.</div>
        </div>
    {:else if requestError}
        <div class="emptyState error">
            <div class="emptyTitle">Could not send the request</div>
            <div class="emptySub">{requestError}</div>
        </div>
    {:else if response}
        <div class="responseContent">
            {#if response.unsupportedContentEncodings.length > 0 || response.contentDecodingFailed}
                <div class="encodingWarningNotice" role="status">
                    <span class="encodingWarningIcon" aria-hidden="true">!</span>
                    <span>
                        {#if response.unsupportedContentEncodings.length > 0}
                            Marmota cannot decode the response Content-Encoding
                            <strong>
                                {response.unsupportedContentEncodings.join(", ")}
                            </strong>
                            .
                        {:else}
                            Marmota could not decode the response body.
                        {/if}
                        The captured body is shown as raw encoded data.
                    </span>
                </div>
            {/if}

            <div class="responseBlock">
                <div class="blockLabel">Head Block</div>
                <HeadBlockViewer text={response.headBlockStr} />
            </div>

            <div class="responseBlock bodyBlock">
                <div class="blockLabel">Body</div>
                <HttpBodyViewer
                    headBlockStr={response.headBlockStr}
                    bodyStr={response.bodyStr}
                    allowHtmlRender={true}
                    bodyIsEncoded={response.contentDecodingFailed ||
                        response.unsupportedContentEncodings.length > 0}
                />
            </div>
        </div>
    {:else}
        <div class="emptyState">
            <div class="emptyTitle">No response yet</div>
            <div class="emptySub">
                Send the request from this tab to see the response here.
            </div>
        </div>
    {/if}
</section>

<style>
    h3 {
        margin: 0;
        font-size: 18px;
        line-height: 1.15;
        letter-spacing: -0.03em;
        color: var(--text);
    }

    .responsePanel {
        display: grid;
        gap: 14px;
        min-width: 0;
    }

    .responseHeader {
        display: flex;
        justify-content: space-between;
        gap: 14px;
        align-items: flex-start;
    }

    .eyebrow,
    .blockLabel {
        color: var(--muted);
        font-size: 11px;
        font-weight: 800;
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .responseMeta {
        display: flex;
        flex-wrap: wrap;
        justify-content: flex-end;
        gap: 8px;
    }

    .metaPill {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-height: 32px;
        padding: 0 10px;
        border-radius: 999px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--text);
        font-size: 11px;
        font-weight: 700;
    }

    .responseContent {
        display: grid;
        gap: 14px;
    }

    .encodingWarningNotice {
        display: flex;
        align-items: flex-start;
        gap: 10px;
        padding: 11px 12px;
        border-radius: 10px;
        border: 1px solid var(--warning-line);
        background: var(--warning-soft);
        color: var(--warning);
        font-size: 12px;
        line-height: 1.5;
    }

    .encodingWarningNotice strong {
        color: var(--text);
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
    }

    .encodingWarningIcon {
        flex: 0 0 auto;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 24px;
        height: 24px;
        border-radius: 8px;
        border: 1px solid var(--warning-line);
        background: var(--warning-soft);
        color: var(--warning);
        font-size: 13px;
        font-weight: 900;
        line-height: 1;
    }

    .responseBlock {
        display: grid;
        gap: 10px;
        min-width: 0;
        min-height: 0;
    }

    .bodyBlock :global(.prettyPanel) {
        max-height: min(56vh, 620px);
        overflow: auto;
        scrollbar-gutter: stable;
    }

    .emptyState {
        border-radius: 12px;
        border: 1px dashed var(--line-strong);
        background: var(--surface-muted);
        padding: 20px;
    }

    .emptyState.error {
        border-style: solid;
        border-color: var(--danger-line);
        background: var(--danger-soft);
    }

    .emptyTitle {
        color: var(--text);
        font-size: 15px;
        font-weight: 800;
    }

    .emptySub {
        margin-top: 8px;
        color: var(--muted);
        line-height: 1.5;
    }
</style>
