<script lang="ts">
    import { createEventDispatcher } from "svelte";

    export let value = "";
    export let label = "";
    export let placeholder = "";
    export let minLines = 8;
    export let issueLines: number[] = [];
    export let ariaLabel = "Editor raw";

    const dispatch = createEventDispatcher<{
        change: { value: string };
    }>();

    let textareaElement: HTMLTextAreaElement | null = null;
    let gutterElement: HTMLDivElement | null = null;
    let issueLineSet = new Set<number>();

    function syncGutter() {
        if (!textareaElement || !gutterElement) return;
        gutterElement.style.transform = `translateY(-${textareaElement.scrollTop}px)`;
    }

    function emitChange(event: Event) {
        dispatch("change", {
            value: (event.currentTarget as HTMLTextAreaElement).value,
        });
    }

    $: issueLineSet = new Set(issueLines);
    $: actualLineCount = Math.max(1, value.split(/\r?\n/).length || 1);
    $: visibleRows = Math.max(minLines, Math.min(actualLineCount, 16));
</script>

<div class="editorRoot">
    <div class="editorHeader">
        <span>{label}</span>
        <span class="lineCount">
            {actualLineCount} {actualLineCount === 1 ? "line" : "lines"}
        </span>
    </div>

    <div class="editorShell">
        <div class="editorGutter" aria-hidden="true">
            <div class="editorGutterInner" bind:this={gutterElement}>
                {#each Array.from({ length: actualLineCount }) as _, index (`editor-line-${index}`)}
                    <div
                        class="editorGutterLine"
                        class:issue={issueLineSet.has(index + 1)}
                    >
                        {index + 1}
                    </div>
                {/each}
            </div>
        </div>

        <textarea
            bind:this={textareaElement}
            bind:value
            class="editorInput"
            spellcheck="false"
            rows={visibleRows}
            {placeholder}
            aria-label={ariaLabel}
            on:input={emitChange}
            on:scroll={syncGutter}
        ></textarea>
    </div>
</div>

<style>
    .editorRoot {
        display: grid;
        gap: 10px;
    }

    .editorHeader {
        display: flex;
        justify-content: space-between;
        gap: 10px;
        align-items: center;
        color: var(--muted);
        font-size: 11px;
        font-weight: 800;
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .lineCount {
        color: var(--muted-strong);
    }

    .editorShell {
        display: grid;
        grid-template-columns: 48px minmax(0, 1fr);
        border-radius: 12px;
        border: 1px solid var(--line);
        background: var(--field-bg);
        overflow: hidden;
        min-height: 200px;
        max-height: 340px;
    }

    .editorGutter {
        overflow: hidden;
        border-right: 1px solid var(--line);
        background: rgba(255, 255, 255, 0.02);
        user-select: none;
    }

    .editorGutterInner {
        padding: 14px 8px 14px 0;
        will-change: transform;
    }

    .editorGutterLine {
        min-height: 1.5em;
        color: var(--muted);
        text-align: right;
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 12px;
        line-height: 1.5;
        white-space: pre;
    }

    .editorGutterLine.issue {
        color: var(--danger);
        font-weight: 700;
    }

    .editorInput {
        width: 100%;
        height: 100%;
        margin: 0;
        padding: 14px;
        border: none;
        background: transparent;
        color: var(--text);
        resize: none;
        overflow: auto;
        outline: none;
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 12px;
        line-height: 1.5;
        white-space: pre;
        tab-size: 4;
    }

    .editorInput::selection {
        background: rgba(var(--accent-rgb), 0.18);
    }

    .editorInput::placeholder {
        color: var(--muted);
    }
</style>
