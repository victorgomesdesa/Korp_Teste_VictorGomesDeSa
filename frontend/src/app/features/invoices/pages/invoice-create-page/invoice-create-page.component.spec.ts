import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';

import { apiConfig } from '../../../../core/config/api.config';
import { Product } from '../../../products/models/product';
import { InvoiceCreatePageComponent } from './invoice-create-page.component';

describe('InvoiceCreatePageComponent', () => {
  const invoicesUrl = `${apiConfig.billingApiUrl}/api/invoices`;
  const productsUrl = `${apiConfig.inventoryApiUrl}/api/products`;
  let httpTesting: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [InvoiceCreatePageComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([{ path: 'invoices/:id', component: InvoiceDetailStubComponent }])
      ]
    }).compileComponents();
    httpTesting = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpTesting.verify());

  it('starts with a single empty item and loads the available products', async () => {
    const fixture = await renderWithProducts();

    expect(fixture.componentInstance.items.length).toBe(1);
    expect(fixture.componentInstance.form.invalid).toBe(true);
  });

  it('keeps products without balance available for selection', async () => {
    const fixture = await renderWithProducts();

    const options = fixture.componentInstance.products();
    expect(options.map((product) => product.code)).toContain('PROD-003');

    const zeroBalance = options.find((product) => product.code === 'PROD-003');
    expect(zeroBalance?.balance).toBe(0);
    expect(fixture.componentInstance.isProductSelectedElsewhere(3, 0)).toBe(false);
  });

  it('adds and removes items from the FormArray', async () => {
    const fixture = await renderWithProducts();
    const component = fixture.componentInstance;

    component.addItem();
    expect(component.items.length).toBe(2);

    component.removeItem(1);
    expect(component.items.length).toBe(1);

    component.removeItem(0);
    expect(component.items.length).toBe(0);
    component.submit();
    httpTesting.expectNone(invoicesUrl);
  });

  it('requires a product and a positive integer quantity', async () => {
    const fixture = await renderWithProducts();
    const item = fixture.componentInstance.items.controls[0];

    expect(item.controls.productId.invalid).toBe(true);

    item.controls.productId.setValue(1);
    item.controls.quantity.setValue(0);
    expect(item.controls.quantity.hasError('min')).toBe(true);

    item.controls.quantity.setValue(1.5);
    expect(item.controls.quantity.hasError('integer')).toBe(true);

    item.controls.quantity.setValue(2);
    expect(fixture.componentInstance.form.valid).toBe(true);
  });

  it('rejects the same product twice and disables it in the other items', async () => {
    const fixture = await renderWithProducts();
    const component = fixture.componentInstance;

    component.items.controls[0].setValue({ productId: 1, quantity: 2 });
    component.addItem();
    component.items.controls[1].setValue({ productId: 1, quantity: 3 });

    expect(component.items.hasError('duplicateProduct')).toBe(true);
    expect(component.form.invalid).toBe(true);
    expect(component.isProductSelectedElsewhere(1, 1)).toBe(true);
    expect(component.isProductSelectedElsewhere(2, 1)).toBe(false);

    component.submit();
    httpTesting.expectNone(invoicesUrl);
  });

  it('disables an already selected product in the other item selects', async () => {
    const fixture = await renderWithProducts();
    const component = fixture.componentInstance;

    component.items.controls[0].setValue({ productId: 1, quantity: 2 });
    component.addItem();
    await settle(fixture);

    // Abre o select do segundo item para inspecionar as opções renderizadas.
    const selects = fixture.nativeElement.querySelectorAll('mat-select');
    (selects[1] as HTMLElement).click();
    await settle(fixture);

    const options = Array.from(document.querySelectorAll('mat-option')).map((option) => ({
      text: option.textContent?.trim() ?? '',
      disabled: option.getAttribute('aria-disabled') === 'true'
    }));

    expect(options.find((option) => option.text.startsWith('PROD-001'))?.disabled).toBe(true);
    expect(options.find((option) => option.text.startsWith('PROD-002'))?.disabled).toBe(false);
    expect(options.find((option) => option.text.startsWith('PROD-003'))?.disabled).toBe(false);
  });

  it('does not call the API when the form is invalid', async () => {
    const fixture = await renderWithProducts();

    fixture.componentInstance.submit();

    httpTesting.expectNone(invoicesUrl);
    expect(fixture.componentInstance.items.controls[0].controls.productId.touched).toBe(true);
  });

  it('sends the canonical payload, blocks a double submit and navigates to the detail', async () => {
    const fixture = await renderWithProducts();
    const router = TestBed.inject(Router);
    fillValidInvoice(fixture);

    submitForm(fixture);
    await settle(fixture);
    expect(submitButton(fixture).disabled).toBe(true);

    const request = httpTesting.expectOne(invoicesUrl);
    expect(request.request.body).toEqual({
      items: [
        { productId: 1, quantity: 2 },
        { productId: 2, quantity: 1 }
      ]
    });

    request.flush({ id: 15, number: 1001, status: 'OPEN', createdAt: '2026-08-17T12:00:00Z', closedAt: null, items: [] });
    await settle(fixture);

    expect(fixture.componentInstance.submitting()).toBe(false);
    expect(router.url).toBe('/invoices/15');
  });

  it('reports backend failures without clearing the form', async () => {
    const tests = [
      { code: 'PRODUCT_NOT_FOUND', status: 404, message: 'Um dos produtos selecionados não está mais disponível.' },
      {
        code: 'INVENTORY_SERVICE_UNAVAILABLE',
        status: 503,
        message: 'Não foi possível validar os produtos porque o serviço de estoque está indisponível. Tente novamente.'
      },
      { code: 'INVALID_QUANTITY', status: 422, message: 'Revise os itens da nota fiscal antes de continuar.' }
    ];

    for (const test of tests) {
      const fixture = await renderWithProducts();
      fillValidInvoice(fixture);

      submitForm(fixture);
      await settle(fixture);

      httpTesting
        .expectOne(invoicesUrl)
        .flush({ code: test.code, message: 'mensagem do backend' }, { status: test.status, statusText: 'error' });
      await settle(fixture);

      expect(document.body.textContent).toContain(test.message);
      expect(fixture.componentInstance.items.length).toBe(2);
      expect(fixture.componentInstance.submitting()).toBe(false);
    }
  });

  async function renderWithProducts(): Promise<ComponentFixture<InvoiceCreatePageComponent>> {
    const fixture = TestBed.createComponent(InvoiceCreatePageComponent);
    fixture.detectChanges();

    httpTesting.expectOne(productsUrl).flush([
      productFixture({ id: 1, code: 'PROD-001', description: 'Teclado Mecânico', balance: 10 }),
      productFixture({ id: 2, code: 'PROD-002', description: 'Mouse', balance: 5 }),
      productFixture({ id: 3, code: 'PROD-003', description: 'Monitor', balance: 0 })
    ]);
    await settle(fixture);
    return fixture;
  }
});

@Component({ template: '' })
class InvoiceDetailStubComponent {}

function fillValidInvoice(fixture: ComponentFixture<InvoiceCreatePageComponent>): void {
  const component = fixture.componentInstance;
  component.items.controls[0].setValue({ productId: 1, quantity: 2 });
  component.addItem();
  component.items.controls[1].setValue({ productId: 2, quantity: 1 });
}

function productFixture(product: Partial<Product> = {}): Product {
  return {
    id: 1,
    code: 'PROD-001',
    description: 'Teclado Mecânico',
    balance: 10,
    createdAt: '2026-08-17T12:00:00Z',
    updatedAt: '2026-08-17T12:00:00Z',
    ...product
  };
}

function submitForm(fixture: ComponentFixture<InvoiceCreatePageComponent>): void {
  const form = fixture.nativeElement.querySelector('form') as HTMLFormElement;
  form.dispatchEvent(new Event('submit'));
}

function submitButton(fixture: ComponentFixture<InvoiceCreatePageComponent>): HTMLButtonElement {
  return fixture.nativeElement.querySelector('button[type="submit"]') as HTMLButtonElement;
}

async function settle(fixture: ComponentFixture<InvoiceCreatePageComponent>): Promise<void> {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}
