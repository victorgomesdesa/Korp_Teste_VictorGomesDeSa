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
import { MatSnackBar } from '@angular/material/snack-bar';
import { Router, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';

import { getApiError } from '../../../../core/http/error.interceptor';
import { ProductService } from '../../services/product.service';

@Component({
  selector: 'app-product-create-page',
  imports: [MatButtonModule, MatFormFieldModule, MatInputModule, ReactiveFormsModule, RouterLink],
  template: `
    <section class="page" aria-labelledby="product-create-title">
      <p class="page-eyebrow">Estoque</p>
      <h1 id="product-create-title">Novo produto</h1>

      <form class="product-form" [formGroup]="form" (ngSubmit)="submit()">
        <mat-form-field>
          <mat-label>Código</mat-label>
          <input matInput formControlName="code" autocomplete="off" />
          @if (form.controls.code.hasError('codeAlreadyExists')) {
            <mat-error>Já existe um produto com esse código.</mat-error>
          } @else if (form.controls.code.invalid) {
            <mat-error>Código é obrigatório.</mat-error>
          }
        </mat-form-field>

        <mat-form-field>
          <mat-label>Descrição</mat-label>
          <input matInput formControlName="description" autocomplete="off" />
          @if (form.controls.description.invalid) {
            <mat-error>Descrição é obrigatória.</mat-error>
          }
        </mat-form-field>

        <mat-form-field>
          <mat-label>Saldo</mat-label>
          <input matInput type="number" formControlName="balance" min="0" step="1" />
          @if (form.controls.balance.hasError('required')) {
            <mat-error>Saldo é obrigatório.</mat-error>
          } @else if (form.controls.balance.hasError('min')) {
            <mat-error>Saldo não pode ser negativo.</mat-error>
          } @else if (form.controls.balance.invalid) {
            <mat-error>Saldo deve ser um número inteiro.</mat-error>
          }
        </mat-form-field>

        <div class="form-actions">
          <a mat-stroked-button routerLink="/products">Cancelar</a>
          <button mat-flat-button type="submit" [disabled]="submitting()">
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
      gap: 0.75rem;
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

  readonly form = this.formBuilder.nonNullable.group({
    code: ['', [Validators.required, notBlankValidator]],
    description: ['', [Validators.required, notBlankValidator]],
    balance: [0, [Validators.required, Validators.min(0), integerValidator]]
  });

  submit(): void {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }

    const { code, description, balance } = this.form.getRawValue();
    this.submitting.set(true);

    this.productService
      .createProduct({ code: code.trim(), description: description.trim(), balance })
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
    const apiError = getApiError(error);

    if (apiError?.code === 'PRODUCT_CODE_ALREADY_EXISTS') {
      this.form.controls.code.setErrors({ codeAlreadyExists: true });
      this.snackBar.open('Já existe um produto com esse código.', 'Fechar', { duration: 5000 });
      return;
    }

    if (apiError?.code === 'VALIDATION_ERROR') {
      this.snackBar.open('Dados do produto inválidos. Revise os campos.', 'Fechar', {
        duration: 5000
      });
      return;
    }

    this.snackBar.open('Não foi possível cadastrar o produto. Tente novamente.', 'Fechar', {
      duration: 5000
    });
  }
}

function notBlankValidator(control: AbstractControl<string>): ValidationErrors | null {
  return control.value.trim() === '' ? { required: true } : null;
}

function integerValidator(control: AbstractControl<number>): ValidationErrors | null {
  return Number.isInteger(control.value) ? null : { integer: true };
}
