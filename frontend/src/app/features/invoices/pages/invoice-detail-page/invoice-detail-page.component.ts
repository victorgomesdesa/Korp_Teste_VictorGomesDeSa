import { DatePipe } from '@angular/common';
import { Component, Injector, OnInit, afterNextRender, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { RouterLink } from '@angular/router';
import { finalize } from 'rxjs';

import { getApiError } from '../../../../core/http/error.interceptor';
import { EmptyStateComponent } from '../../../../shared/components/empty-state/empty-state.component';
import { ErrorMessageComponent } from '../../../../shared/components/error-message/error-message.component';
import { LoadingComponent } from '../../../../shared/components/loading/loading.component';
import { Invoice, invoiceStatusLabel } from '../../models/invoice';
import { InvoiceService } from '../../services/invoice.service';

@Component({
  selector: 'app-invoice-detail-page',
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
    <section class="page" aria-labelledby="invoice-detail-title">
      <p class="page-eyebrow">Faturamento</p>
      <h1 id="invoice-detail-title">Detalhes da nota fiscal</h1>

      @if (loading()) {
        <app-loading label="Carregando nota fiscal..." />
      } @else if (notFound()) {
        <div class="page-empty">
          <app-empty-state
            title="Nota fiscal não encontrada."
            message="A nota informada não existe ou não está mais disponível."
          />
          <a mat-flat-button routerLink="/invoices">Voltar para notas</a>
        </div>
      } @else if (errorMessage()) {
        <div class="page-error">
          <app-error-message [message]="errorMessage()!" />
          <button mat-stroked-button type="button" (click)="loadInvoice()">Tentar novamente</button>
        </div>
      } @else if (invoice(); as invoice) {
        <dl class="invoice-summary">
          <div>
            <dt>Número</dt>
            <dd>{{ invoice.number }}</dd>
          </div>
          <div>
            <dt>Status</dt>
            <dd>{{ statusLabel(invoice.status) }}</dd>
          </div>
          <div>
            <dt>Criada em</dt>
            <dd>{{ invoice.createdAt | date: 'dd/MM/yyyy HH:mm' }}</dd>
          </div>
          @if (invoice.closedAt) {
            <div>
              <dt>Fechada em</dt>
              <dd>{{ invoice.closedAt | date: 'dd/MM/yyyy HH:mm' }}</dd>
            </div>
          }
        </dl>

        <h2>Itens</h2>
        <div class="table-scroll">
          <table mat-table [dataSource]="invoice.items">
            <ng-container matColumnDef="productCode">
              <th mat-header-cell *matHeaderCellDef scope="col">Código</th>
              <td mat-cell *matCellDef="let item">{{ item.productCode }}</td>
            </ng-container>

            <ng-container matColumnDef="productDescription">
              <th mat-header-cell *matHeaderCellDef scope="col">Descrição</th>
              <td mat-cell *matCellDef="let item">{{ item.productDescription }}</td>
            </ng-container>

            <ng-container matColumnDef="quantity">
              <th mat-header-cell *matHeaderCellDef scope="col">Quantidade</th>
              <td mat-cell *matCellDef="let item">{{ item.quantity }}</td>
            </ng-container>

            <tr mat-header-row *matHeaderRowDef="displayedColumns"></tr>
            <tr mat-row *matRowDef="let row; columns: displayedColumns"></tr>
          </table>
        </div>

        <div class="detail-actions">
          <a mat-stroked-button routerLink="/invoices">Voltar para notas</a>
          @if (invoice.status === 'OPEN') {
            <button
              mat-flat-button
              type="button"
              [disabled]="closing()"
              [attr.aria-busy]="closing()"
              (click)="closeInvoice()"
            >
              {{ closing() ? 'Processando...' : 'Imprimir Nota' }}
            </button>
          }
        </div>
      }
    </section>
  `,
  styles: `
    .page-error,
    .page-empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 1rem;
    }

    .invoice-summary {
      display: flex;
      gap: 2rem;
      flex-wrap: wrap;
      margin: 2rem 0;
    }

    dt {
      color: var(--mat-sys-on-surface-variant);
      font: var(--mat-sys-label-large);
    }

    dd {
      margin: 0.25rem 0 0;
      font: var(--mat-sys-title-medium);
    }

    h2 {
      font: var(--mat-sys-title-medium);
    }

    .table-scroll {
      overflow-x: auto;
    }

    table {
      width: 100%;
      background: transparent;
    }

    .detail-actions {
      display: flex;
      justify-content: flex-end;
      margin-top: 2rem;
    }
  `
})
export class InvoiceDetailPageComponent implements OnInit {
  private readonly injector = inject(Injector);
  private readonly invoiceService = inject(InvoiceService);
  private readonly snackBar = inject(MatSnackBar);

  // Identidade lógica do fechamento em andamento: preservada entre tentativas ambíguas e
  // descartada quando o backend confirma que nenhuma operação ficou pendente.
  private idempotencyKey: string | null = null;

  readonly id = input.required<string>();

  readonly displayedColumns = ['productCode', 'productDescription', 'quantity'];
  readonly statusLabel = invoiceStatusLabel;
  readonly invoice = signal<Invoice | null>(null);
  readonly loading = signal(false);
  readonly closing = signal(false);
  readonly notFound = signal(false);
  readonly errorMessage = signal<string | null>(null);

  ngOnInit(): void {
    this.loadInvoice();
  }

  loadInvoice(): void {
    this.loading.set(true);
    this.notFound.set(false);
    this.errorMessage.set(null);

    this.invoiceService
      .getInvoice(Number(this.id()))
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (invoice) => this.invoice.set(invoice),
        error: (error: unknown) => {
          if (getApiError(error)?.code === 'INVOICE_NOT_FOUND') {
            this.notFound.set(true);
            return;
          }
          this.errorMessage.set('Não foi possível carregar a nota fiscal. Tente novamente.');
        }
      });
  }

  closeInvoice(): void {
    if (this.closing()) {
      return;
    }

    // Retry de uma tentativa ambígua reaproveita a chave; uma nova operação lógica gera outra.
    this.idempotencyKey ??= crypto.randomUUID();
    this.closing.set(true);

    this.invoiceService
      .closeInvoice(Number(this.id()), this.idempotencyKey)
      .pipe(finalize(() => this.closing.set(false)))
      .subscribe({
        next: (invoice) => {
          this.idempotencyKey = null;
          this.invoice.set(invoice);
          this.snackBar.open('Nota fiscal fechada com sucesso.', 'Fechar', { duration: 5000 });
          this.printWhenRendered();
        },
        error: (error: unknown) => this.handleCloseError(error)
      });
  }

  private handleCloseError(error: unknown): void {
    const code = getApiError(error)?.code;

    switch (code) {
      case 'INSUFFICIENT_STOCK':
      case 'PRODUCT_NOT_FOUND':
        // Falha definitiva: o Billing liberou a operação e uma nova tentativa é outra operação lógica.
        this.idempotencyKey = null;
        break;
      case 'INVOICE_ALREADY_CLOSED':
      case 'INVOICE_CLOSE_ALREADY_IN_PROGRESS':
      case 'IDEMPOTENCY_KEY_REUSED':
        // O fechamento pertence a outra operação; a chave local nunca assumiu a nota.
        this.idempotencyKey = null;
        this.loadInvoice();
        break;
      default:
        // Resultado ambíguo (503, timeout, 500): manter a chave permite recuperar a mesma operação.
        break;
    }

    this.snackBar.open(closeErrorMessage(code), 'Fechar', { duration: 8000 });
  }

  // A impressão só começa depois que o DOM reflete a nota fechada.
  private printWhenRendered(): void {
    afterNextRender(() => window.print(), { injector: this.injector });
  }
}

function closeErrorMessage(code: string | undefined): string {
  switch (code) {
    case 'INSUFFICIENT_STOCK':
      return 'Estoque insuficiente para fechar a nota fiscal.';
    case 'PRODUCT_NOT_FOUND':
      return 'Um dos produtos da nota não está mais disponível.';
    case 'INVOICE_ALREADY_CLOSED':
      return 'Esta nota já foi fechada por outra operação.';
    case 'INVOICE_CLOSE_ALREADY_IN_PROGRESS':
      return 'Esta nota já está sendo processada.';
    case 'IDEMPOTENCY_KEY_REUSED':
      return 'Não foi possível concluir a operação por inconsistência da tentativa anterior.';
    default:
      return 'Não foi possível concluir a operação. Tente novamente.';
  }
}
