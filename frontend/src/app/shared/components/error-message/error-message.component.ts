import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

@Component({
  selector: 'app-error-message',
  imports: [MatIconModule],
  template: `
    <div class="error-message" role="alert">
      <mat-icon aria-hidden="true">error</mat-icon>
      <span>{{ message() }}</span>
    </div>
  `,
  styles: `
    .error-message {
      display: flex;
      align-items: flex-start;
      gap: 0.75rem;
      padding: 1rem 1.125rem;
      color: var(--mat-sys-on-error-container);
      background: var(--mat-sys-error-container);
      border: 2px solid var(--mat-sys-error);
      border-radius: 0.75rem;
      box-shadow: var(--mat-sys-level2);
      font-weight: 600;
    }

    mat-icon {
      flex: 0 0 auto;
      color: var(--mat-sys-error);
    }
  `
})
export class ErrorMessageComponent {
  readonly message = input.required<string>();
}
