<script lang="ts">
    export let tabId: string;
</script>

<span class="iconShell" aria-hidden="true">
    <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor">
        {#if tabId === "configureProxy"}
            <path d="M4 7h10" stroke-linecap="round" stroke-width="1.8" />
            <path d="M18 7h2" stroke-linecap="round" stroke-width="1.8" />
            <circle cx="16" cy="7" r="2.3" stroke-width="1.8" />
            <path d="M4 17h2" stroke-linecap="round" stroke-width="1.8" />
            <path d="M10 17h10" stroke-linecap="round" stroke-width="1.8" />
            <circle cx="8" cy="17" r="2.3" stroke-width="1.8" />
        {:else if tabId === "httpHistory"}
            <circle cx="12" cy="12" r="7.5" stroke-width="1.8" />
            <path d="M12 8.5v4.2l2.8 1.8" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" />
        {:else if tabId === "savedRequests"}
            <path d="M7 5.5h10A1.5 1.5 0 0 1 18.5 7v11.5l-6.5-3-6.5 3V7A1.5 1.5 0 0 1 7 5.5Z" stroke-linejoin="round" stroke-width="1.8" />
        {:else if tabId === "interception"}
            <path d="M6 5h12l-4.6 5.2v5.1l-2.8 1.7v-6.8z" stroke-linejoin="round" stroke-width="1.8" />
        {:else if tabId === "errorLog"}
            <path d="M12 4.5 20 18.5H4z" stroke-linejoin="round" stroke-width="1.8" />
            <path d="M12 9v4.2" stroke-linecap="round" stroke-width="1.8" />
            <circle cx="12" cy="15.8" r="0.9" fill="currentColor" stroke="none" />
        {:else}
            <path d="M7 8H18" stroke-linecap="round" stroke-width="1.8" />
            <path d="M15.5 5.5 18 8l-2.5 2.5" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" />
            <path d="M17 16H6" stroke-linecap="round" stroke-width="1.8" />
            <path d="M8.5 13.5 6 16l2.5 2.5" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" />
        {/if}
    </svg>
</span>

<style>
    .iconShell {
        width: 36px;
        height: 36px;
        flex: 0 0 auto;
        border-radius: 10px;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        border: 1px solid var(--line);
        background: var(--surface-muted);
    }

    .icon {
        width: 18px;
        height: 18px;
    }
</style>
