import { Component, inject, signal } from '@angular/core';
import {
  AbstractControl,
  FormArray,
  FormBuilder,
  FormGroup,
  ReactiveFormsModule,
  ValidationErrors,
  Validators
} from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Router, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';

import { getApiError } from '../../../../core/http/error.interceptor';
import { ErrorMessageComponent } from '../../../../shared/components/error-message/error-message.component';
import { LoadingComponent } from '../../../../shared/components/loading/loading.component';
import { Product } from '../../../products/models/product';
import { ProductService } from '../../../products/services/product.service';
import { InvoiceService } from '../../services/invoice.service';

@Component({
  selector: 'app-invoice-create-page',
  imports: [
    ErrorMessageComponent,
    LoadingComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    ReactiveFormsModule,
    RouterLink
  ],
  template: `
    <section class="page" aria-labelledby="invoice-create-title">
      <p class="page-eyebrow">Faturamento</p>
      <h1 id="invoice-create-title">Nova nota fiscal</h1>

      @if (loadingProducts()) {
        <app-loading label="Carregando produtos..." />
      } @else if (productsErrorMessage()) {
        <div class="page-error">
          <app-error-message [message]="productsErrorMessage()!" />
          <button mat-stroked-button type="button" (click)="loadProducts()">Tentar novamente</button>
        </div>
      } @else {
        <form class="invoice-form" [formGroup]="form" (ngSubmit)="submit()">
          <div formArrayName="items">
            @for (item of items.controls; track $index; let itemIndex = $index) {
              <div class="invoice-item" [formGroupName]="itemIndex">
                <mat-form-field>
                  <mat-label>Produto</mat-label>
                  <mat-select formControlName="productId">
                    @for (product of products(); track product.id) {
                      <mat-option
                        [value]="product.id"
                        [disabled]="isProductSelectedElsewhere(product.id, itemIndex)"
                      >
                        {{ product.code }} — {{ product.description }} (Saldo: {{ product.balance }})
                      </mat-option>
                    }
                  </mat-select>
                  @if (item.controls.productId.invalid) {
                    <mat-error>Selecione um produto.</mat-error>
                  }
                </mat-form-field>

                <mat-form-field class="quantity-field">
                  <mat-label>Quantidade</mat-label>
                  <input matInput type="number" formControlName="quantity" min="1" step="1" />
                  @if (item.controls.quantity.invalid) {
                    <mat-error>Quantidade deve ser maior que zero.</mat-error>
                  }
                </mat-form-field>

                <button
                  mat-stroked-button
                  type="button"
                  [attr.aria-label]="'Remover item ' + (itemIndex + 1)"
                  (click)="removeItem(itemIndex)"
                >
                  Remover
                </button>
              </div>
            }
          </div>

          @if (items.hasError('duplicateProduct')) {
            <app-error-message message="O mesmo produto não pode ser informado duas vezes." />
          }
          @if (items.length === 0) {
            <app-error-message message="A nota fiscal deve possuir ao menos um item." />
          }

          <div class="form-actions">
            <button mat-stroked-button type="button" (click)="addItem()">Adicionar produto</button>
            <span class="form-actions-spacer"></span>
            <a mat-stroked-button routerLink="/invoices">Cancelar</a>
            <button mat-flat-button type="submit" [disabled]="submitting()">
              {{ submitting() ? 'Criando...' : 'Criar nota' }}
            </button>
          </div>
        </form>
      }
    </section>
  `,
  styles: `
    .page-error {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 1rem;
    }

    .invoice-form {
      display: flex;
      flex-direction: column;
      gap: 1rem;
      margin-top: 2rem;
    }

    .invoice-item {
      display: flex;
      align-items: center;
      gap: 1rem;
      flex-wrap: wrap;
    }

    .invoice-item mat-form-field {
      flex: 1 1 16rem;
    }

    .quantity-field {
      flex: 0 1 9rem;
    }

    .form-actions {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      flex-wrap: wrap;
    }

    .form-actions-spacer {
      flex: 1 1 auto;
    }
  `
})
export class InvoiceCreatePageComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly invoiceService = inject(InvoiceService);
  private readonly productService = inject(ProductService);
  private readonly router = inject(Router);
  private readonly snackBar = inject(MatSnackBar);

  readonly products = signal<Product[]>([]);
  readonly loadingProducts = signal(false);
  readonly productsErrorMessage = signal<string | null>(null);
  readonly submitting = signal(false);

  readonly form = this.formBuilder.nonNullable.group({
    items: this.formBuilder.nonNullable.array([this.createItem()], duplicateProductValidator)
  });

  constructor() {
    this.loadProducts();
  }

  get items(): FormArray<ReturnType<InvoiceCreatePageComponent['createItem']>> {
    return this.form.controls.items;
  }

  loadProducts(): void {
    this.loadingProducts.set(true);
    this.productsErrorMessage.set(null);

    this.productService
      .getProducts()
      .pipe(finalize(() => this.loadingProducts.set(false)))
      .subscribe({
        next: (products) => this.products.set(products),
        error: () =>
          this.productsErrorMessage.set(
            'Não foi possível carregar os produtos. Tente novamente.'
          )
      });
  }

  addItem(): void {
    this.items.push(this.createItem());
  }

  removeItem(index: number): void {
    this.items.removeAt(index);
  }

  isProductSelectedElsewhere(productId: number, index: number): boolean {
    return this.items.controls.some(
      (item, itemIndex) => itemIndex !== index && item.controls.productId.value === productId
    );
  }

  submit(): void {
    if (this.form.invalid || this.items.length === 0) {
      this.form.markAllAsTouched();
      return;
    }

    const items = this.items.controls.map((item) => item.getRawValue());
    this.submitting.set(true);

    this.invoiceService
      .createInvoice({ items })
      .pipe(finalize(() => this.submitting.set(false)))
      .subscribe({
        next: (invoice) => {
          this.snackBar.open('Nota fiscal criada com sucesso.', 'Fechar', { duration: 5000 });
          void this.router.navigate(['/invoices', invoice.id]);
        },
        error: (error: unknown) => this.snackBar.open(createErrorMessage(error), 'Fechar', {
          duration: 8000
        })
      });
  }

  private createItem() {
    return this.formBuilder.nonNullable.group({
      productId: [0, [Validators.required, Validators.min(1)]],
      quantity: [1, [Validators.required, Validators.min(1), integerValidator]]
    });
  }
}

function duplicateProductValidator(control: AbstractControl): ValidationErrors | null {
  const items = control as FormArray<FormGroup<{ productId: AbstractControl<number> }>>;
  const productIds = items.controls.map((item) => item.controls.productId.value);

  return new Set(productIds).size === productIds.length ? null : { duplicateProduct: true };
}

function integerValidator(control: AbstractControl<number>): ValidationErrors | null {
  return Number.isInteger(control.value) ? null : { integer: true };
}

function createErrorMessage(error: unknown): string {
  const apiError = getApiError(error);

  switch (apiError?.code) {
    case 'PRODUCT_NOT_FOUND':
      return 'Um dos produtos selecionados não está mais disponível.';
    case 'INVENTORY_SERVICE_UNAVAILABLE':
      return 'Não foi possível validar os produtos porque o serviço de estoque está indisponível. Tente novamente.';
    case 'INVALID_QUANTITY':
    case 'INVALID_PRODUCT_ID':
    case 'INVALID_INVOICE_ITEMS':
    case 'DUPLICATE_PRODUCT_ID':
      return 'Revise os itens da nota fiscal antes de continuar.';
    default:
      return 'Não foi possível criar a nota fiscal. Tente novamente.';
  }
}
