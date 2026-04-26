<script lang="ts">
    import CodeBlockShell from "@/shared/components/CodeBlockShell.svelte";
    import SearchHighlight from "@/shared/components/SearchHighlight.svelte";

    type HtmlSegment = {
        id: string;
        text: string;
        className: string;
    };

    type HtmlLine = {
        id: string;
        segments: HtmlSegment[];
    };

    export let htmlText = "";
    export let searchQuery = "";

    function tokenizeTag(tagText: string): HtmlSegment[] {
        const segments: HtmlSegment[] = [];
        let index = 0;
        let segmentIndex = 0;

        function pushSegment(text: string, className: string) {
            if (text.length === 0) return;

            segments.push({
                id: `segment-${segmentIndex}`,
                text,
                className,
            });
            segmentIndex += 1;
        }

        if (tagText[index] === "<") {
            pushSegment("<", "htmlPunctuation");
            index += 1;
        }

        while (index < tagText.length && /\s/.test(tagText[index])) {
            const start = index;
            while (index < tagText.length && /\s/.test(tagText[index])) {
                index += 1;
            }
            pushSegment(tagText.slice(start, index), "htmlWhitespace");
        }

        if (tagText[index] === "/") {
            pushSegment("/", "htmlPunctuation");
            index += 1;
        }

        const tagNameStart = index;
        while (index < tagText.length && !/[\s/>]/.test(tagText[index])) {
            index += 1;
        }
        pushSegment(tagText.slice(tagNameStart, index), "htmlTagName");

        while (index < tagText.length) {
            const currentChar = tagText[index];

            if (/\s/.test(currentChar)) {
                const start = index;
                while (index < tagText.length && /\s/.test(tagText[index])) {
                    index += 1;
                }
                pushSegment(tagText.slice(start, index), "htmlWhitespace");
                continue;
            }

            if (currentChar === "/" || currentChar === ">" || currentChar === "=") {
                pushSegment(currentChar, "htmlPunctuation");
                index += 1;
                continue;
            }

            if (currentChar === '"' || currentChar === "'") {
                const quote = currentChar;
                pushSegment(quote, "htmlQuote");
                index += 1;

                const valueStart = index;
                while (index < tagText.length && tagText[index] !== quote) {
                    index += 1;
                }

                pushSegment(tagText.slice(valueStart, index), "htmlAttrValue");

                if (index < tagText.length) {
                    pushSegment(tagText[index], "htmlQuote");
                    index += 1;
                }
                continue;
            }

            const attributeStart = index;
            while (index < tagText.length && !/[\s=/>]/.test(tagText[index])) {
                index += 1;
            }
            pushSegment(tagText.slice(attributeStart, index), "htmlAttrName");
        }

        return segments;
    }

    function tokenizeLine(lineText: string): HtmlSegment[] {
        if (lineText.length === 0) {
            return [
                {
                    id: "segment-empty",
                    text: " ",
                    className: "htmlWhitespace",
                },
            ];
        }

        const tokenPattern = new RegExp("<!--.*?-->|<\\/?[^>]+>|[^<]+", "g");
        const segments: HtmlSegment[] = [];
        let segmentIndex = 0;
        let match = tokenPattern.exec(lineText);

        while (match) {
            const token = match[0];

            if (token.startsWith("<!--")) {
                segments.push({
                    id: `segment-${segmentIndex}`,
                    text: token,
                    className: "htmlComment",
                });
                segmentIndex += 1;
            } else if (token.startsWith("<")) {
                for (const segment of tokenizeTag(token)) {
                    segments.push({
                        ...segment,
                        id: `segment-${segmentIndex}-${segment.id}`,
                    });
                    segmentIndex += 1;
                }
            } else {
                segments.push({
                    id: `segment-${segmentIndex}`,
                    text: token,
                    className: "htmlText",
                });
                segmentIndex += 1;
            }

            match = tokenPattern.exec(lineText);
        }

        return segments;
    }

    function buildLines(rawHtml: string): HtmlLine[] {
        return rawHtml.split(/\r?\n/).map((lineText, index) => ({
            id: `line-${index}`,
            segments: tokenizeLine(lineText),
        }));
    }

    $: lines = buildLines(htmlText ?? "");
</script>

<CodeBlockShell text={htmlText} emptyLabel="(no HTML content)">
    {#if lines.length === 0}
        <div class="htmlLine">
            <span class="htmlEmpty">(no HTML content)</span>
        </div>
    {:else}
        {#each lines as line (line.id)}
            <div class="htmlLine">
                {#each line.segments as segment (segment.id)}
                    <SearchHighlight
                        className={segment.className}
                        text={segment.text}
                        query={searchQuery}
                    />
                {/each}
            </div>
        {/each}
    {/if}
</CodeBlockShell>

<style>
    .htmlLine {
        white-space: pre-wrap;
        word-break: break-word;
        color: var(--text);
    }

    .htmlEmpty {
        color: var(--muted);
    }

    :global(.htmlPunctuation),
    :global(.htmlQuote) {
        color: var(--muted-strong);
    }

    :global(.htmlTagName) {
        color: #93c5fd;
    }

    :global(.htmlAttrName) {
        color: #fbbf24;
    }

    :global(.htmlAttrValue) {
        color: #86efac;
    }

    :global(.htmlText) {
        color: #e5e7eb;
    }

    :global(.htmlComment) {
        color: var(--muted);
    }

    :global(.htmlWhitespace) {
        color: inherit;
    }
</style>
