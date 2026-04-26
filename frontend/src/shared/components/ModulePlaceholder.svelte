<script lang="ts">
  export let title: string;
  export let description: string;
  export let emptyTitle: string;
  export let emptySub: string;
</script>

<div class="placeholderView">
  <div class="heroCard">
    <span class="eyebrow">Workspace</span>
    <h2>{title}</h2>
    <p>{description}</p>
  </div>

  <div class="emptyCard">
    <div class="signalRow">
      <span class="signalDot"></span>
      <span class="signalText">Not available yet</span>
    </div>

    <div class="emptyTitle">{emptyTitle}</div>
    <div class="emptySub">{emptySub}</div>
  </div>
</div>

<style>
  h2,
  p {
    margin: 0;
  }

  h2 {
    font-size: clamp(26px, 3vw, 32px);
    line-height: 1.05;
    letter-spacing: -0.04em;
    color: var(--text);
  }

  p {
    color: var(--muted);
    line-height: 1.6;
  }

  .placeholderView {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .heroCard,
  .emptyCard {
    border-radius: 16px;
    border: 1px solid var(--line);
    background: var(--surface-strong);
    box-shadow: var(--shadow-card);
  }

  .heroCard {
    padding: 18px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .eyebrow {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .emptyCard {
    padding: 18px;
    background: var(--surface-strong);
  }

  .signalRow {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 14px;
    color: var(--muted-strong);
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .signalDot {
    width: 10px;
    height: 10px;
    border-radius: 999px;
    background: var(--accent);
    box-shadow: 0 0 0 6px rgba(var(--accent-rgb), 0.14);
  }

  .emptyTitle {
    color: var(--text);
    font-size: 18px;
    font-weight: 700;
    letter-spacing: -0.03em;
  }

  .emptySub {
    margin-top: 8px;
    color: var(--muted);
    line-height: 1.55;
    max-width: 680px;
  }

  @media (max-width: 760px) {
    .heroCard,
    .emptyCard {
      border-radius: 14px;
    }
  }
</style>
