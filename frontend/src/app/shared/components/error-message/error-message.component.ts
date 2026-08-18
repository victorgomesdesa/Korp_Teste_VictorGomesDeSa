import { Component, input } from '@angular/core';

@Component({
  selector: 'app-error-message',
  template: `<p class="error-message" role="alert">{{ message() }}</p>`,
  styles: `
    .error-message {
      padding: 0.875rem 1rem;
      color: var(--mat-sys-on-error-container);
      background: var(--mat-sys-error-container);
      border-radius: 0.5rem;
    }
  `
})
export class ErrorMessageComponent {
  readonly message = input.required<string>();
}
