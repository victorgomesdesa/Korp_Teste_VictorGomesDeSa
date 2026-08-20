import { CurrencyPipe, DatePipe } from '@angular/common';
import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatTableModule } from '@angular/material/table';
import { MatIconModule } from '@angular/material/icon';
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
    CurrencyPipe,
    DatePipe,
    EmptyStateComponent,
    ErrorMessageComponent,
    LoadingComponent,
    MatButtonModule,
    MatIconModule,
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

        <a mat-flat-button routerLink="/invoices/new"><mat-icon aria-hidden="true">add</mat-icon>Nova nota</a>
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
          <a mat-flat-button routerLink="/invoices/new"><mat-icon aria-hidden="true">add</mat-icon>Criar primeira nota</a>
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

            <ng-container matColumnDef="total">
              <th mat-header-cell *matHeaderCellDef scope="col">Valor total</th>
              <td mat-cell *matCellDef="let invoice">
                {{ invoice.totalInCents / 100 | currency: 'BRL' : 'symbol' : '1.2-2' }}
              </td>
            </ng-container>

            <ng-container matColumnDef="products">
              <th mat-header-cell *matHeaderCellDef scope="col">Produtos</th>
              <td mat-cell *matCellDef="let invoice">
                <span class="product-codes">{{ visibleProductCodes(invoice).join(', ') }}</span>
                @if (hiddenProductCount(invoice) > 0) {
                  <span class="more-products">+{{ hiddenProductCount(invoice) }}</span>
                }
              </td>
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
                  <mat-icon aria-hidden="true">arrow_forward</mat-icon>
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

    th.mat-mdc-header-cell,
    td.mat-mdc-cell {
      text-align: center;
      vertical-align: middle;
    }

    .product-codes {
      white-space: nowrap;
    }

    .more-products {
      margin-left: 0.4rem;
      color: var(--mat-sys-primary);
      font-weight: 600;
      white-space: nowrap;
    }
  `
})
export class InvoicesPageComponent {
  private readonly invoiceService = inject(InvoiceService);

  readonly displayedColumns = ['number', 'products', 'total', 'status', 'createdAt', 'closedAt', 'actions'];
  readonly statusLabel = invoiceStatusLabel;
  readonly invoices = signal<InvoiceSummary[]>([]);
  readonly loading = signal(false);
  readonly errorMessage = signal<string | null>(null);

  constructor() {
    this.loadInvoices();
  }

  visibleProductCodes(invoice: InvoiceSummary): string[] {
    return [...new Set(invoice.productCodes ?? [])].slice(0, 5);
  }

  hiddenProductCount(invoice: InvoiceSummary): number {
    return Math.max(0, new Set(invoice.productCodes ?? []).size - 5);
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
