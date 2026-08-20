import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';

import { apiConfig } from '../../../../core/config/api.config';
import { ProductCreatePageComponent } from './product-create-page.component';

describe('ProductCreatePageComponent', () => {
  const productsUrl = `${apiConfig.inventoryApiUrl}/api/products`;
  let httpTesting: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ProductCreatePageComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([{ path: 'products', component: ProductsStubComponent }])
      ]
    }).compileComponents();
    httpTesting = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpTesting.verify());

  it('requires code and description that are not only whitespace', () => {
    const fixture = TestBed.createComponent(ProductCreatePageComponent);
    const form = fixture.componentInstance.form;

    expect(form.valid).toBe(false);

    form.setValue({ code: '   ', description: '   ', balance: 1, price: 10 });
    expect(form.controls.code.valid).toBe(false);
    expect(form.controls.description.valid).toBe(false);

    form.setValue({ code: '005', description: 'Webcam', balance: 1, price: 10 });
    expect(form.valid).toBe(true);
  });

  it('requires positive integer balance and positive price', () => {
    const fixture = TestBed.createComponent(ProductCreatePageComponent);
    const balance = fixture.componentInstance.form.controls.balance;
    const price = fixture.componentInstance.form.controls.price;

    balance.setValue(-1);
    expect(balance.hasError('min')).toBe(true);

    balance.setValue(1.5);
    expect(balance.hasError('integer')).toBe(true);

    balance.setValue(0);
    expect(balance.hasError('min')).toBe(true);

    balance.setValue(1);
    expect(balance.valid).toBe(true);

    price.setValue(0);
    expect(price.hasError('min')).toBe(true);

    price.setValue(0.01);
    expect(price.valid).toBe(true);
  });

  it('does not call the API when the form is invalid', () => {
    const fixture = TestBed.createComponent(ProductCreatePageComponent);
    fixture.detectChanges();

    fixture.componentInstance.submit();

    httpTesting.expectNone(productsUrl);
    expect(fixture.componentInstance.form.controls.code.touched).toBe(true);
  });

  it('trims the payload, disables the submit button and navigates on success', async () => {
    const fixture = TestBed.createComponent(ProductCreatePageComponent);
    const router = TestBed.inject(Router);
    fixture.detectChanges();

    fixture.componentInstance.form.setValue({
      code: '  005  ',
      description: '  Webcam  ',
      balance: 3,
      price: 149.9
    });
    submitForm(fixture);
    await settle(fixture);

    expect(submitButton(fixture).disabled).toBe(true);

    const request = httpTesting.expectOne(productsUrl);
    expect(request.request.body).toEqual({
      code: 'PROD-005',
      description: 'Webcam',
      balance: 3,
      priceInCents: 14990
    });

    request.flush({ id: 1, code: 'PROD-005', description: 'Webcam', balance: 3, priceInCents: 14990 });
    await settle(fixture);

    expect(fixture.componentInstance.submitting()).toBe(false);
    expect(router.url).toBe('/products');
  });

  it('reports a duplicated product code next to the field', async () => {
    const fixture = TestBed.createComponent(ProductCreatePageComponent);
    fixture.detectChanges();

    fixture.componentInstance.form.setValue({
      code: '001',
      description: 'Teclado Mecânico',
      balance: 10,
      price: 200
    });
    submitForm(fixture);
    await settle(fixture);

    httpTesting.expectOne(productsUrl).flush(
      { code: 'PRODUCT_CODE_ALREADY_EXISTS', message: 'Já existe um produto com esse código.' },
      { status: 409, statusText: 'Conflict' }
    );
    await settle(fixture);

    expect(fixture.componentInstance.form.controls.code.hasError('codeAlreadyExists')).toBe(true);
    expect(fixture.nativeElement.textContent).toContain('Já existe um produto com esse código.');
    expect(submitButton(fixture).disabled).toBe(false);
    // O feedback aparece apenas no campo, sem duplicar em snackbar.
    expect(document.querySelector('mat-snack-bar-container')).toBeNull();
  });

  it('clears the server-side code error when the user edits the field', async () => {
    const fixture = TestBed.createComponent(ProductCreatePageComponent);
    fixture.detectChanges();

    fixture.componentInstance.form.setValue({
      code: '001',
      description: 'Teclado Mecânico',
      balance: 10,
      price: 200
    });
    submitForm(fixture);
    await settle(fixture);

    httpTesting.expectOne(productsUrl).flush(
      { code: 'PRODUCT_CODE_ALREADY_EXISTS', message: 'Já existe um produto com esse código.' },
      { status: 409, statusText: 'Conflict' }
    );
    await settle(fixture);
    expect(fixture.componentInstance.form.controls.code.hasError('codeAlreadyExists')).toBe(true);

    fixture.componentInstance.form.controls.code.setValue('002');
    await settle(fixture);

    expect(fixture.componentInstance.form.controls.code.hasError('codeAlreadyExists')).toBe(false);
    expect(fixture.componentInstance.form.controls.code.valid).toBe(true);
  });

  it('shows a connection message when the Inventory cannot be reached', async () => {
    const fixture = TestBed.createComponent(ProductCreatePageComponent);
    fixture.detectChanges();

    fixture.componentInstance.form.setValue({
      code: '005',
      description: 'Webcam',
      balance: 3,
      price: 149.9
    });
    submitForm(fixture);
    await settle(fixture);

    httpTesting.expectOne(productsUrl).error(new ProgressEvent('error'), { status: 0 });
    await settle(fixture);

    expect(document.body.textContent).toContain('Não foi possível conectar ao serviço. Tente novamente.');
  });

  it('keeps the user on the page with a controlled message on unexpected failures', async () => {
    const fixture = TestBed.createComponent(ProductCreatePageComponent);
    const router = TestBed.inject(Router);
    fixture.detectChanges();

    fixture.componentInstance.form.setValue({
      code: '005',
      description: 'Webcam',
      balance: 3,
      price: 149.9
    });
    submitForm(fixture);
    await settle(fixture);

    httpTesting
      .expectOne(productsUrl)
      .flush({ code: 'INTERNAL_ERROR', message: 'Erro interno do servidor.' }, {
        status: 500,
        statusText: 'Internal Server Error'
      });
    await settle(fixture);

    expect(router.url).not.toBe('/products');
    expect(fixture.componentInstance.submitting()).toBe(false);
    expect(document.body.textContent).toContain(
      'Não foi possível cadastrar o produto. Tente novamente.'
    );
  });
});

@Component({ template: '' })
class ProductsStubComponent {}

function submitForm(fixture: ComponentFixture<ProductCreatePageComponent>): void {
  const form = fixture.nativeElement.querySelector('form') as HTMLFormElement;
  form.dispatchEvent(new Event('submit'));
}

function submitButton(fixture: ComponentFixture<ProductCreatePageComponent>): HTMLButtonElement {
  return fixture.nativeElement.querySelector('button[type="submit"]') as HTMLButtonElement;
}

async function settle(fixture: ComponentFixture<ProductCreatePageComponent>): Promise<void> {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}
