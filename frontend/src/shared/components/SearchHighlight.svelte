<script lang="ts">
    import { splitHighlightedText } from "@/shared/utils/textSearch";

    export let text: string = "";
    export let query: string = "";
    export let className: string = "";
    export let wrap = true;

    $: segments = splitHighlightedText(text ?? "", query);
</script>

<span class={`highlightText ${wrap ? "" : "noWrap"} ${className}`}>{#each segments as segment, index (`${index}-${segment.text}`)}{#if segment.match}<mark class="searchHit" tabindex="-1">{segment.text}</mark>{:else}<span>{segment.text}</span>{/if}{/each}</span>

<style>
    .highlightText {
        white-space: pre-wrap;
        word-break: break-word;
    }

    .highlightText.noWrap {
        white-space: pre;
        word-break: normal;
    }

    .searchHit {
        padding: 0;
        border-radius: 4px;
        background: rgba(250, 204, 21, 0.42);
        color: #111827;
        scroll-margin-block: 32px;
        scroll-margin-inline: 24px;
        transition:
            background 140ms ease,
            color 140ms ease,
            outline-color 140ms ease,
            box-shadow 140ms ease;
    }

    .searchHit.activeSearchHit,
    .searchHit:focus {
        background: rgba(250, 204, 21, 0.82);
        color: #020617;
        box-shadow: 0 0 0 2px rgba(250, 204, 21, 0.22);
        outline: 2px solid rgba(254, 240, 138, 0.95);
        outline-offset: 1px;
    }
</style>
