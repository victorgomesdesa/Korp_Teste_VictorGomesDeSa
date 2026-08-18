import { Component, input } from '@angular/core';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';

@Component({
  selector: 'app-loading',
  imports: [MatProgressSpinnerModule],
  template: `
    <div class="loading" role="status" [attr.aria-label]="label()">
      <mat-spinner diameter="36" />
      <span>{{ label() }}</span>
    </div>
  `,
  styles: `
    .loading {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 1rem;
      min-height: 8rem;
      color: var(--mat-sys-on-surface-variant);
    }
  `
})
export class LoadingComponent {
  readonly label = input('Carregando...');
}
