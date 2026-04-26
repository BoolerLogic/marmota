<script lang="ts">
    import { onDestroy, tick } from "svelte";

    export let text = "";
    export let emptyLabel = "(empty)";
    export let interactive = false;
    export let ariaLabel = "Code block";

    let shellElement: HTMLDivElement | null = null;
    let renderViewport: HTMLDivElement | null = null;
    let inputOverlay: HTMLDivElement | null = null;
    let gutterInner: HTMLDivElement | null = null;
    let renderInner: HTMLDivElement | null = null;
    let syncSource: "render" | "input" | null = null;
    let scrollbarWidth = 0;
    let resizeObserver: ResizeObserver | null = null;
    let measureFrame = 0;
    let observedElements: Element[] = [];

    function splitLines(value: string): string[] {
        return value.split("\n");
    }

    function normalizeDisplayText(value: string): string {
        return value.replace(/\r\n/g, "\n");
    }

    function syncGutter(scrollTop: number) {
        if (!gutterInner) return;
        gutterInner.style.transform = `translateY(-${scrollTop}px)`;
    }

    function updateShellHeight() {
        if (!shellElement || !renderInner) return;

        const shellStyles = getComputedStyle(shellElement);
        const maxHeight = parseFloat(shellStyles.maxHeight) || 300;
        const borderTop = parseFloat(shellStyles.borderTopWidth) || 0;
        const borderBottom = parseFloat(shellStyles.borderBottomWidth) || 0;
        const contentHeight = Math.ceil(
            Math.max(
                renderInner.scrollHeight,
                renderViewport?.scrollHeight ?? 0,
                inputOverlay?.scrollHeight ?? 0,
            ),
        );
        const nextHeight = Math.min(
            maxHeight,
            contentHeight + borderTop + borderBottom,
        );

        shellElement.style.setProperty("--code-shell-height", `${nextHeight}px`);
    }

    function measureNow() {
        updateShellHeight();
        updateScrollbarMetrics();
    }

    function scheduleMeasurements() {
        if (measureFrame) {
            cancelAnimationFrame(measureFrame);
        }

        measureFrame = requestAnimationFrame(() => {
            measureFrame = 0;
            measureNow();

            // A second pass catches late scrollbar/layout stabilization on first paint.
            requestAnimationFrame(() => {
                measureNow();
            });
        });
    }

    function reconnectResizeObserver() {
        const nextElements = [
            shellElement,
            renderViewport,
            inputOverlay,
            renderInner,
        ].filter(Boolean) as Element[];

        const isSameTargetSet =
            nextElements.length === observedElements.length &&
            nextElements.every((element, index) => element === observedElements[index]);

        if (isSameTargetSet) return;

        resizeObserver?.disconnect();
        resizeObserver = null;
        observedElements = nextElements;

        if (typeof ResizeObserver === "undefined" || nextElements.length === 0) {
            return;
        }

        resizeObserver = new ResizeObserver(() => {
            scheduleMeasurements();
        });

        nextElements.forEach((element) => {
            resizeObserver?.observe(element);
        });
    }

    function updateScrollbarMetrics() {
        const metricsSource = interactive ? inputOverlay : renderViewport;
        if (!shellElement || !metricsSource) return;

        const nextScrollbarWidth = Math.max(
            0,
            metricsSource.offsetWidth - metricsSource.clientWidth,
        );

        if (nextScrollbarWidth === scrollbarWidth) return;

        scrollbarWidth = nextScrollbarWidth;
        shellElement.style.setProperty(
            "--code-scrollbar-width",
            `${scrollbarWidth}px`,
        );
    }

    function syncInteractiveViewportFromOverlay() {
        if (!inputOverlay || !renderViewport) return;

        renderViewport.scrollTop = inputOverlay.scrollTop;
        renderViewport.scrollLeft = inputOverlay.scrollLeft;

        const effectiveScrollTop = renderViewport.scrollTop;
        const effectiveScrollLeft = renderViewport.scrollLeft;

        if (inputOverlay.scrollTop !== effectiveScrollTop) {
            inputOverlay.scrollTop = effectiveScrollTop;
        }

        if (inputOverlay.scrollLeft !== effectiveScrollLeft) {
            inputOverlay.scrollLeft = effectiveScrollLeft;
        }

        syncGutter(effectiveScrollTop);
    }

    function releaseSync() {
        requestAnimationFrame(() => {
            syncSource = null;
        });
    }

    function preventOverlayMutation(event: InputEvent) {
        event.preventDefault();
    }

    function handleOverlayKeydown(event: KeyboardEvent) {
        const isPlainCharacterInput =
            event.key.length === 1 &&
            !event.ctrlKey &&
            !event.metaKey &&
            !event.altKey;

        if (
            isPlainCharacterInput ||
            event.key === "Backspace" ||
            event.key === "Delete" ||
            event.key === "Enter" ||
            event.key === "Tab"
        ) {
            event.preventDefault();
        }
    }

    function syncFromRenderViewport() {
        if (!renderViewport) return;
        if (syncSource === "input") return;

        syncSource = "render";
        syncGutter(renderViewport.scrollTop);

        if (interactive && inputOverlay) {
            inputOverlay.scrollTop = renderViewport.scrollTop;
            inputOverlay.scrollLeft = renderViewport.scrollLeft;
        }

        releaseSync();
    }

    function syncFromInputOverlay() {
        if (!inputOverlay) return;
        if (syncSource === "render") return;

        syncSource = "input";
        syncInteractiveViewportFromOverlay();

        releaseSync();
    }

    $: displayText = normalizeDisplayText(
        text.trim().length > 0 ? text : emptyLabel,
    );
    $: lines = splitLines(displayText);
    $: if (renderViewport && !interactive) {
        syncGutter(renderViewport.scrollTop);
    }
    $: if (renderViewport && inputOverlay && interactive) {
        syncInteractiveViewportFromOverlay();
    }
    $: if (shellElement && renderInner && (renderViewport || inputOverlay)) {
        tick().then(scheduleMeasurements);
    }
    $: if (shellElement && renderInner) {
        reconnectResizeObserver();
    }

    onDestroy(() => {
        resizeObserver?.disconnect();
        observedElements = [];

        if (measureFrame) {
            cancelAnimationFrame(measureFrame);
        }
    });
</script>

<svelte:window on:resize={scheduleMeasurements} />

<div bind:this={shellElement} class="codeBlockShell" class:interactive>
    <div class="codeGutter" aria-hidden="true">
        <div class="codeGutterInner" bind:this={gutterInner}>
            {#each lines as _, index (`line-number-${index}`)}
                <div class="codeGutterLine">{index + 1}</div>
            {/each}
        </div>
    </div>

    <div class="codeViewportStack">
        <div
            class="codeRenderViewport"
            class:interactive
            bind:this={renderViewport}
            on:scroll={syncFromRenderViewport}
        >
            <div class="codeRenderInner" bind:this={renderInner}>
                <slot {displayText} {lines}></slot>
            </div>
        </div>

        {#if interactive}
            <div
                bind:this={inputOverlay}
                class="codeInputOverlay"
                contenteditable="true"
                spellcheck="false"
                role="textbox"
                aria-multiline="true"
                aria-label={ariaLabel}
                tabindex="0"
                on:beforeinput={preventOverlayMutation}
                on:keydown={handleOverlayKeydown}
                on:scroll={syncFromInputOverlay}
            >{displayText}</div>
        {/if}
    </div>
</div>

<style>
    .codeBlockShell {
        display: grid;
        grid-template-columns: 48px minmax(0, 1fr);
        height: var(--code-shell-height, auto);
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--field-bg);
        color: var(--text);
        overflow: hidden;
        max-height: 300px;
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 12px;
        line-height: 1.5;
    }

    .codeGutter {
        overflow: hidden;
        border-right: 1px solid var(--line);
        background: rgba(255, 255, 255, 0.02);
        user-select: none;
    }

    .codeGutterInner {
        padding: 14px 8px 14px 0;
        will-change: transform;
    }

    .codeGutterLine {
        min-height: 1.5em;
        line-height: 1.5em;
        color: var(--muted);
        text-align: right;
        white-space: pre;
    }

    .codeViewportStack {
        position: relative;
        min-width: 0;
        min-height: 0;
        height: 100%;
    }

    .codeRenderViewport,
    .codeInputOverlay {
        box-sizing: border-box;
        width: 100%;
        height: 100%;
        font: inherit;
        line-height: inherit;
        tab-size: 4;
        overscroll-behavior: contain;
    }

    .codeRenderViewport {
        overflow: auto;
    }

    .codeRenderViewport.interactive {
        pointer-events: none;
        overflow: hidden;
        scrollbar-width: none;
    }

    .codeRenderViewport.interactive::-webkit-scrollbar {
        width: 0;
        height: 0;
    }

    .codeRenderInner {
        box-sizing: border-box;
        min-height: 100%;
        min-width: max-content;
        padding: 14px calc(14px + var(--code-scrollbar-width, 0px)) 14px 14px;
    }

    .codeInputOverlay {
        position: absolute;
        inset: 0;
        margin: 0;
        padding: 14px;
        border: none;
        background: transparent;
        color: transparent;
        caret-color: var(--text);
        cursor: default;
        overflow: auto;
        outline: none;
        white-space: pre;
        min-width: 0;
        overflow-wrap: normal;
        word-break: normal;
        scrollbar-gutter: stable;
    }

    .codeInputOverlay::selection {
        background: rgba(var(--accent-rgb), 0.34);
        color: transparent;
    }
</style>
