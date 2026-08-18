import { Component, input } from '@angular/core';

@Component({
  selector: 'app-empty-state',
  template: `
    <section class="empty-state" aria-live="polite">
      <h2>{{ title() }}</h2>
      <p>{{ message() }}</p>
    </section>
  `,
  styles: `
    .empty-state {
      padding: 2rem;
      text-align: center;
      color: var(--mat-sys-on-surface-variant);
      border: 1px dashed var(--mat-sys-outline-variant);
      border-radius: 0.75rem;
    }

    h2 {
      margin-top: 0;
      color: var(--mat-sys-on-surface);
    }

    p {
      margin-bottom: 0;
    }
  `
})
export class EmptyStateComponent {
  readonly title = input.required<string>();
  readonly message = input.required<string>();
}
