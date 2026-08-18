import { DatePipe } from '@angular/common';
import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatTableModule } from '@angular/material/table';
import { RouterLink } from '@angular/router';
import { finalize } from 'rxjs';

import { loadErrorMessage } from '../../../../core/http/load-error-message';
import { EmptyStateComponent } from '../../../../shared/components/empty-state/empty-state.component';
import { ErrorMessageComponent } from '../../../../shared/components/error-message/error-message.component';
import { LoadingComponent } from '../../../../shared/components/loading/loading.component';
import { InvoiceSummary, invoiceStatusLabel } from '../../models/invoice';
import { InvoiceService } from '../../services/invoice.service';

@Component({
  selector: 'app-invoices-page',
  imports: [
    DatePipe,
    EmptyStateComponent,
    ErrorMessageComponent,
    LoadingComponent,
    MatButtonModule,
    MatTableModule,
    RouterLink
  ],
  template: `
    <section class="page" aria-labelledby="invoices-title">
      <div class="page-header">
        <div>
          <p class="page-eyebrow">Faturamento</p>
          <h1 id="invoices-title">Notas fiscais</h1>
        </div>

        <a mat-flat-button routerLink="/invoices/new">Nova nota</a>
      </div>

      @if (loading()) {
        <app-loading label="Carregando notas fiscais..." />
      } @else if (errorMessage()) {
        <div class="page-error">
          <app-error-message [message]="errorMessage()!" />
          <button mat-stroked-button type="button" (click)="loadInvoices()">Tentar novamente</button>
        </div>
      } @else if (invoices().length === 0) {
        <div class="page-empty">
          <app-empty-state
            title="Nenhuma nota fiscal cadastrada."
            message="Crie uma nota para registrar os produtos a serem processados."
          />
          <a mat-flat-button routerLink="/invoices/new">Criar primeira nota</a>
        </div>
      } @else {
        <div class="table-scroll">
          <table mat-table [dataSource]="invoices()">
            <ng-container matColumnDef="number">
              <th mat-header-cell *matHeaderCellDef scope="col">Número</th>
              <td mat-cell *matCellDef="let invoice">{{ invoice.number }}</td>
            </ng-container>

            <ng-container matColumnDef="status">
              <th mat-header-cell *matHeaderCellDef scope="col">Status</th>
              <td mat-cell *matCellDef="let invoice">{{ statusLabel(invoice.status) }}</td>
            </ng-container>

            <ng-container matColumnDef="createdAt">
              <th mat-header-cell *matHeaderCellDef scope="col">Criada em</th>
              <td mat-cell *matCellDef="let invoice">
                {{ invoice.createdAt | date: 'dd/MM/yyyy HH:mm' }}
              </td>
            </ng-container>

            <ng-container matColumnDef="closedAt">
              <th mat-header-cell *matHeaderCellDef scope="col">Fechada em</th>
              <td mat-cell *matCellDef="let invoice">
                {{ invoice.closedAt ? (invoice.closedAt | date: 'dd/MM/yyyy HH:mm') : '—' }}
              </td>
            </ng-container>

            <ng-container matColumnDef="actions">
              <th mat-header-cell *matHeaderCellDef scope="col">Ações</th>
              <td mat-cell *matCellDef="let invoice">
                <a
                  mat-stroked-button
                  [routerLink]="['/invoices', invoice.id]"
                  [attr.aria-label]="'Visualizar nota ' + invoice.number"
                >
                  Visualizar
                </a>
              </td>
            </ng-container>

            <tr mat-header-row *matHeaderRowDef="displayedColumns"></tr>
            <tr mat-row *matRowDef="let row; columns: displayedColumns"></tr>
          </table>
        </div>
      }
    </section>
  `,
  styles: `
    .page-header {
      display: flex;
      align-items: flex-end;
      justify-content: space-between;
      gap: 1rem;
      flex-wrap: wrap;
      margin-bottom: 2rem;
    }

    .page-error,
    .page-empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 1rem;
    }

    .table-scroll {
      overflow-x: auto;
    }

    table {
      width: 100%;
      background: transparent;
    }
  `
})
export class InvoicesPageComponent {
  private readonly invoiceService = inject(InvoiceService);

  readonly displayedColumns = ['number', 'status', 'createdAt', 'closedAt', 'actions'];
  readonly statusLabel = invoiceStatusLabel;
  readonly invoices = signal<InvoiceSummary[]>([]);
  readonly loading = signal(false);
  readonly errorMessage = signal<string | null>(null);

  constructor() {
    this.loadInvoices();
  }

  loadInvoices(): void {
    this.loading.set(true);
    this.errorMessage.set(null);

    this.invoiceService
      .getInvoices()
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (invoices) => this.invoices.set(invoices),
        error: (error: unknown) =>
          this.errorMessage.set(loadErrorMessage(error, 'as notas fiscais'))
      });
  }
}
