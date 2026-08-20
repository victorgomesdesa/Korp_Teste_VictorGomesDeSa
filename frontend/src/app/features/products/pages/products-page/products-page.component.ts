import { CurrencyPipe } from '@angular/common';
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
import { Product } from '../../models/product';
import { ProductService } from '../../services/product.service';

@Component({
  selector: 'app-products-page',
  imports: [
    CurrencyPipe,
    EmptyStateComponent,
    ErrorMessageComponent,
    LoadingComponent,
    MatButtonModule,
    MatIconModule,
    MatTableModule,
    RouterLink
  ],
  template: `
    <section class="page" aria-labelledby="products-title">
      <div class="page-header">
        <div>
          <p class="page-eyebrow">Estoque</p>
          <h1 id="products-title">Produtos</h1>
        </div>

        <a mat-flat-button routerLink="/products/new"><mat-icon aria-hidden="true">add</mat-icon>Novo produto</a>
      </div>

      @if (loading()) {
        <app-loading label="Carregando produtos..." />
      } @else if (errorMessage()) {
        <div class="page-error">
          <app-error-message [message]="errorMessage()!" />
          <button mat-stroked-button type="button" (click)="loadProducts()">Tentar novamente</button>
        </div>
      } @else if (products().length === 0) {
        <div class="page-empty">
          <app-empty-state
            title="Nenhum produto cadastrado."
            message="Cadastre um produto para começar a controlar o estoque."
          />
          <a mat-flat-button routerLink="/products/new"><mat-icon aria-hidden="true">add</mat-icon>Cadastrar primeiro produto</a>
        </div>
      } @else {
        <div class="table-scroll">
          <table mat-table [dataSource]="products()">
            <ng-container matColumnDef="code">
              <th mat-header-cell *matHeaderCellDef scope="col">Código</th>
              <td mat-cell *matCellDef="let product">{{ product.code }}</td>
            </ng-container>

            <ng-container matColumnDef="description">
              <th mat-header-cell *matHeaderCellDef scope="col">Nome</th>
              <td mat-cell *matCellDef="let product">{{ product.description }}</td>
            </ng-container>

            <ng-container matColumnDef="balance">
              <th mat-header-cell *matHeaderCellDef scope="col">Estoque</th>
              <td mat-cell *matCellDef="let product">{{ product.balance }}</td>
            </ng-container>

            <ng-container matColumnDef="price">
              <th mat-header-cell *matHeaderCellDef scope="col">Valor/unidade</th>
              <td mat-cell *matCellDef="let product">
                {{ product.priceInCents / 100 | currency: 'BRL' : 'symbol' : '1.2-2' }}
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
  `
})
export class ProductsPageComponent {
  private readonly productService = inject(ProductService);

  readonly displayedColumns = ['code', 'description', 'balance', 'price'];
  readonly products = signal<Product[]>([]);
  readonly loading = signal(false);
  readonly errorMessage = signal<string | null>(null);

  constructor() {
    this.loadProducts();
  }

  loadProducts(): void {
    this.loading.set(true);
    this.errorMessage.set(null);

    this.productService
      .getProducts()
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (products) => this.products.set(products),
        error: (error: unknown) => this.errorMessage.set(loadErrorMessage(error, 'os produtos'))
      });
  }
}
