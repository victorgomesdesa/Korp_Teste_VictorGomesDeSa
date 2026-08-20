import { Component, inject, signal } from '@angular/core';
import {
  AbstractControl,
  FormBuilder,
  ReactiveFormsModule,
  ValidationErrors,
  Validators
} from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Router, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';

import { getApiError, isConnectionError } from '../../../../core/http/error.interceptor';
import { ErrorMessageComponent } from '../../../../shared/components/error-message/error-message.component';
import { ProductService } from '../../services/product.service';

@Component({
  selector: 'app-product-create-page',
  imports: [ErrorMessageComponent, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, ReactiveFormsModule, RouterLink],
  template: `
    <section class="page" aria-labelledby="product-create-title">
      <p class="page-eyebrow">Estoque</p>
      <h1 id="product-create-title">Novo produto</h1>

      <form class="product-form" [formGroup]="form" (ngSubmit)="submit()">
        <mat-form-field>
          <mat-label>Número do código</mat-label>
          <span matTextPrefix>PROD-&nbsp;</span>
          <input matInput formControlName="code" inputmode="numeric" autocomplete="off" />
          @if (form.controls.code.hasError('codeAlreadyExists')) {
            <mat-error>Já existe um produto com esse código.</mat-error>
          } @else if (form.controls.code.invalid) {
            <mat-error>Informe somente o número do código.</mat-error>
          }
        </mat-form-field>

        <mat-form-field>
          <mat-label>Nome</mat-label>
          <input matInput formControlName="description" autocomplete="off" />
          @if (form.controls.description.invalid) {
            <mat-error>Nome é obrigatório.</mat-error>
          }
        </mat-form-field>

        <mat-form-field>
          <mat-label>Estoque</mat-label>
          <input matInput type="number" formControlName="balance" min="1" step="1" />
          @if (form.controls.balance.hasError('required')) {
            <mat-error>Estoque é obrigatório.</mat-error>
          } @else if (form.controls.balance.hasError('min')) {
            <mat-error>Estoque deve ser maior que zero.</mat-error>
          } @else if (form.controls.balance.invalid) {
            <mat-error>Estoque deve ser um número inteiro.</mat-error>
          }
        </mat-form-field>

        <mat-form-field>
          <mat-label>Valor/unidade</mat-label>
          <span matTextPrefix>R$&nbsp;</span>
          <input matInput type="number" formControlName="price" min="0.01" step="0.01" />
          @if (form.controls.price.hasError('required')) {
            <mat-error>Valor é obrigatório.</mat-error>
          } @else if (form.controls.price.hasError('min')) {
            <mat-error>Valor deve ser maior que zero.</mat-error>
          } @else if (form.controls.price.invalid) {
            <mat-error>Use no máximo duas casas decimais.</mat-error>
          }
        </mat-form-field>

        @if (errorMessage()) {
          <app-error-message [message]="errorMessage()!" />
        }

        <div class="form-actions">
          <a mat-stroked-button routerLink="/products">Cancelar</a>
          <button mat-flat-button type="submit" [disabled]="submitting()">
            <mat-icon aria-hidden="true">save</mat-icon>
            {{ submitting() ? 'Salvando...' : 'Salvar' }}
          </button>
        </div>
      </form>
    </section>
  `,
  styles: `
    .product-form {
      display: flex;
      flex-direction: column;
      gap: 0.5rem;
      max-width: 32rem;
      margin-top: 2rem;
    }

    .form-actions {
      display: flex;
      justify-content: flex-end;
      gap: 1rem;
      margin-top: 1rem;
    }
  `
})
export class ProductCreatePageComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly productService = inject(ProductService);
  private readonly router = inject(Router);
  private readonly snackBar = inject(MatSnackBar);

  readonly submitting = signal(false);
  readonly errorMessage = signal<string | null>(null);

  readonly form = this.formBuilder.nonNullable.group({
    code: ['', [Validators.required, productCodeNumberValidator]],
    description: ['', [Validators.required, notBlankValidator]],
    balance: [0, [Validators.required, Validators.min(1), integerValidator]],
    price: [0, [Validators.required, Validators.min(0.01), currencyValidator]]
  });

  submit(): void {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }

    const { code, description, balance, price } = this.form.getRawValue();
    this.submitting.set(true);
    this.errorMessage.set(null);

    this.productService
      .createProduct({
        code: `PROD-${code.trim()}`,
        description: description.trim(),
        balance,
        priceInCents: Math.round(price * 100)
      })
      .pipe(finalize(() => this.submitting.set(false)))
      .subscribe({
        next: () => {
          this.snackBar.open('Produto cadastrado com sucesso.', 'Fechar', { duration: 5000 });
          void this.router.navigateByUrl('/products');
        },
        error: (error: unknown) => this.handleCreateError(error)
      });
  }

  private handleCreateError(error: unknown): void {
    // Código duplicado é sinalizado no próprio campo; os demais erros vão para o snackbar.
    if (getApiError(error)?.code === 'PRODUCT_CODE_ALREADY_EXISTS') {
      this.form.controls.code.setErrors({ codeAlreadyExists: true });
      return;
    }

    this.errorMessage.set(createProductErrorMessage(error));
  }
}

function createProductErrorMessage(error: unknown): string {
  if (isConnectionError(error)) {
    return 'Não foi possível conectar ao serviço. Tente novamente.';
  }

  switch (getApiError(error)?.code) {
    case 'VALIDATION_ERROR':
      return 'Revise os dados informados.';
    default:
      return 'Não foi possível cadastrar o produto. Tente novamente.';
  }
}

function notBlankValidator(control: AbstractControl<string>): ValidationErrors | null {
  return control.value.trim() === '' ? { required: true } : null;
}

function integerValidator(control: AbstractControl<number>): ValidationErrors | null {
  return Number.isInteger(control.value) ? null : { integer: true };
}

function currencyValidator(control: AbstractControl<number>): ValidationErrors | null {
  return Number.isInteger(control.value * 100) ? null : { currency: true };
}

function productCodeNumberValidator(control: AbstractControl<string>): ValidationErrors | null {
  return /^\d+$/.test(control.value.trim()) ? null : { pattern: true };
}
