<script lang="ts">
    import { createEventDispatcher, onMount, tick } from "svelte";
    import { buildSafeHtmlPreviewDocument } from "./htmlPreviewDocument";

    export let htmlText = "";

    const dispatch = createEventDispatcher<{
        close: void;
    }>();

    let closeButton: HTMLButtonElement | null = null;
    let previewFrame: HTMLIFrameElement | null = null;
    let modalOverlay: HTMLDivElement | null = null;
    let previouslyFocusedElement: HTMLElement | null = null;
    let inertSnapshots: Array<{
        element: HTMLElement;
        wasInert: boolean;
    }> = [];
    let previousBodyOverflow = "";
    let previousBodyPaddingRight = "";
    let mounted = false;

    $: previewDocument = buildSafeHtmlPreviewDocument(htmlText);

    onMount(() => {
        mounted = true;
        previouslyFocusedElement =
            document.activeElement instanceof HTMLElement
                ? document.activeElement
                : null;

        previousBodyOverflow = document.body.style.overflow;
        previousBodyPaddingRight = document.body.style.paddingRight;
        inertSnapshots = isolateModalBranch(modalOverlay);

        const scrollbarWidth =
            window.innerWidth - document.documentElement.clientWidth;

        document.body.style.overflow = "hidden";
        if (scrollbarWidth > 0) {
            document.body.style.paddingRight = `${scrollbarWidth}px`;
        }

        void tick().then(() => {
            if (mounted) {
                closeButton?.focus({ preventScroll: true });
            }
        });

        return () => {
            mounted = false;
            document.body.style.overflow = previousBodyOverflow;
            document.body.style.paddingRight = previousBodyPaddingRight;
            restoreInertSnapshots(inertSnapshots);
            inertSnapshots = [];

            if (previouslyFocusedElement?.isConnected) {
                previouslyFocusedElement.focus({ preventScroll: true });
            }
        };
    });

    function closeDialog() {
        dispatch("close");
    }

    function handleBackdropClick(event: MouseEvent) {
        if (event.target === event.currentTarget) {
            closeDialog();
        }
    }

    function handleBackdropKeydown(event: KeyboardEvent) {
        if (
            event.target === event.currentTarget &&
            (event.key === "Enter" || event.key === " ")
        ) {
            event.preventDefault();
            closeDialog();
        }
    }

    function handleModalWheel(event: WheelEvent) {
        if (event.ctrlKey || event.metaKey) return;
        event.preventDefault();
    }

    function handleWindowKeydown(event: KeyboardEvent) {
        if (event.key === "Escape") {
            event.preventDefault();
            event.stopImmediatePropagation();
            closeDialog();
        }
    }

    function handleCloseButtonKeydown(event: KeyboardEvent) {
        if (event.key !== "Tab" || !event.shiftKey) return;

        event.preventDefault();
        previewFrame?.focus({ preventScroll: true });
    }

    function cycleFocusToCloseButton() {
        closeButton?.focus({ preventScroll: true });
    }

    function isolateModalBranch(
        modalElement: HTMLElement | null,
    ): Array<{ element: HTMLElement; wasInert: boolean }> {
        if (!modalElement) return [];

        const snapshots: Array<{
            element: HTMLElement;
            wasInert: boolean;
        }> = [];
        let branch: HTMLElement = modalElement;
        let parent = branch.parentElement;

        while (parent) {
            for (const sibling of Array.from(parent.children)) {
                if (
                    sibling === branch ||
                    !(sibling instanceof HTMLElement)
                ) {
                    continue;
                }

                snapshots.push({
                    element: sibling,
                    wasInert: sibling.inert,
                });
                sibling.inert = true;
            }

            if (parent === document.body) break;
            branch = parent;
            parent = parent.parentElement;
        }

        return snapshots;
    }

    function restoreInertSnapshots(
        snapshots: Array<{ element: HTMLElement; wasInert: boolean }>,
    ) {
        for (const snapshot of snapshots) {
            snapshot.element.inert = snapshot.wasInert;
        }
    }
</script>

<svelte:window on:keydown|capture={handleWindowKeydown} />

<div
    bind:this={modalOverlay}
    class="modalOverlay"
    role="presentation"
    tabindex="-1"
    on:click={handleBackdropClick}
    on:keydown={handleBackdropKeydown}
    on:wheel={handleModalWheel}
    on:touchmove|preventDefault={() => undefined}
>
    <section
        class="modalCard"
        role="dialog"
        aria-modal="true"
        aria-labelledby="html-render-title"
        aria-describedby="html-render-description"
    >
        <header class="modalHeader">
            <div class="headerCopy">
                <span class="eyebrow">Isolated preview</span>
                <h2 id="html-render-title">Rendered HTML</h2>
                <p id="html-render-description">
                    Scripts, forms, navigation, popups, media and network
                    access are blocked.
                </p>
            </div>

            <button
                bind:this={closeButton}
                type="button"
                class="closeButton"
                aria-label="Close rendered HTML preview"
                title="Close preview"
                on:click={closeDialog}
                on:keydown={handleCloseButtonKeydown}
            >
                <svg
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                    focusable="false"
                >
                    <path d="M6 6l12 12M18 6L6 18" />
                </svg>
            </button>
        </header>

        <div class="previewViewport">
            <!-- svelte-ignore a11y-no-noninteractive-tabindex -->
            <iframe
                bind:this={previewFrame}
                title="Isolated rendered HTML response"
                srcdoc={previewDocument}
                sandbox=""
                allow=""
                referrerpolicy="no-referrer"
                tabindex="0"
            />
        </div>

        <button
            type="button"
            class="focusSentinel"
            aria-label="Return focus to preview controls"
            on:focus={cycleFocusToCloseButton}
        />
    </section>
</div>

<style>
    .modalOverlay {
        position: fixed;
        inset: 0;
        z-index: 120;
        display: grid;
        place-items: center;
        padding: 3vh 3vw;
        background: rgba(2, 6, 23, 0.76);
        backdrop-filter: blur(10px);
    }

    .modalCard {
        width: 94vw;
        height: 92vh;
        min-width: 0;
        min-height: 0;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        border-radius: 18px;
        border: 1px solid var(--line-strong);
        background: var(--surface-strong);
        box-shadow: var(--shadow-card);
    }

    .modalHeader {
        flex: 0 0 auto;
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        gap: 18px;
        padding: 18px 20px;
        border-bottom: 1px solid var(--line);
    }

    .headerCopy {
        min-width: 0;
        display: grid;
        gap: 5px;
    }

    .eyebrow {
        color: var(--muted);
        font-size: 10px;
        font-weight: 800;
        letter-spacing: 0.12em;
        text-transform: uppercase;
    }

    h2 {
        margin: 0;
        color: var(--text);
        font-size: 20px;
        line-height: 1.15;
        letter-spacing: -0.025em;
    }

    p {
        margin: 0;
        color: var(--muted);
        font-size: 12px;
        line-height: 1.5;
    }

    .closeButton {
        appearance: none;
        flex: 0 0 auto;
        width: 40px;
        height: 40px;
        display: inline-grid;
        place-items: center;
        padding: 0;
        border-radius: 11px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        color: var(--muted);
        cursor: pointer;
        outline: none;
        transition:
            color 140ms ease,
            border-color 140ms ease,
            background 140ms ease,
            transform 140ms ease,
            box-shadow 140ms ease;
    }

    .closeButton:hover {
        color: var(--text);
        border-color: var(--line-strong);
        background: rgba(148, 163, 184, 0.1);
        transform: translateY(-1px);
    }

    .closeButton:focus-visible {
        border-color: rgba(var(--accent-rgb), 0.72);
        box-shadow: 0 0 0 4px rgba(var(--accent-rgb), 0.16);
    }

    .closeButton svg {
        width: 19px;
        height: 19px;
        fill: none;
        stroke: currentColor;
        stroke-width: 2;
        stroke-linecap: round;
    }

    .previewViewport {
        flex: 1 1 auto;
        min-width: 0;
        min-height: 0;
        margin: 12px;
        overflow: hidden;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: #ffffff;
    }

    iframe {
        display: block;
        width: 100%;
        height: 100%;
        border: 0;
        background: #ffffff;
    }

    .focusSentinel {
        position: absolute;
        width: 1px;
        height: 1px;
        padding: 0;
        margin: -1px;
        overflow: hidden;
        clip: rect(0 0 0 0);
        white-space: nowrap;
        border: 0;
    }

    @media (max-width: 760px) {
        .modalOverlay {
            padding: 10px;
        }

        .modalCard {
            width: calc(100vw - 20px);
            height: calc(100vh - 20px);
            border-radius: 14px;
        }

        .modalHeader {
            padding: 14px;
        }

        .previewViewport {
            margin: 8px;
        }
    }
</style>
