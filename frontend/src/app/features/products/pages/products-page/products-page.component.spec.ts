import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { apiConfig } from '../../../../core/config/api.config';
import { Product } from '../../models/product';
import { ProductsPageComponent } from './products-page.component';

describe('ProductsPageComponent', () => {
  const productsUrl = `${apiConfig.inventoryApiUrl}/api/products`;
  let httpTesting: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ProductsPageComponent],
      providers: [provideHttpClient(), provideHttpClientTesting(), provideRouter([])]
    }).compileComponents();
    httpTesting = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpTesting.verify());

  it('shows the loading state while the products are pending', () => {
    const fixture = TestBed.createComponent(ProductsPageComponent);
    fixture.detectChanges();

    expect(fixture.nativeElement.textContent).toContain('Carregando produtos...');

    httpTesting.expectOne(productsUrl).flush([]);
  });

  it('lists the products returned by the Inventory, including zero balance', async () => {
    const fixture = TestBed.createComponent(ProductsPageComponent);
    fixture.detectChanges();

    httpTesting.expectOne(productsUrl).flush([
      productFixture({ id: 1, code: 'PROD-001', description: 'Teclado Mecânico', balance: 10 }),
      productFixture({ id: 2, code: 'PROD-004', description: 'Cabo HDMI', balance: 0 })
    ]);
    await settle(fixture);

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('PROD-001');
    expect(text).toContain('Teclado Mecânico');
    expect(text).toContain('PROD-004');
    expect(text).not.toContain('Nenhum produto cadastrado.');

    const balances = Array.from(
      fixture.nativeElement.querySelectorAll('td.mat-column-balance') as NodeListOf<HTMLElement>
    ).map((cell) => cell.textContent?.trim());
    expect(balances).toEqual(['10', '0']);
  });

  it('keeps the order returned by the backend', async () => {
    const fixture = TestBed.createComponent(ProductsPageComponent);
    fixture.detectChanges();

    httpTesting.expectOne(productsUrl).flush([
      productFixture({ id: 3, code: 'PROD-003', description: 'Monitor', balance: 1 }),
      productFixture({ id: 1, code: 'PROD-001', description: 'Teclado Mecânico', balance: 10 })
    ]);
    await settle(fixture);

    const codes = Array.from(
      fixture.nativeElement.querySelectorAll('td.mat-column-code') as NodeListOf<HTMLElement>
    ).map((cell) => cell.textContent?.trim());
    expect(codes).toEqual(['PROD-003', 'PROD-001']);
  });

  it('shows the empty state when there is no product', async () => {
    const fixture = TestBed.createComponent(ProductsPageComponent);
    fixture.detectChanges();

    httpTesting.expectOne(productsUrl).flush([]);
    await settle(fixture);

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('Nenhum produto cadastrado.');
    expect(text).toContain('Cadastrar primeiro produto');
  });

  it('shows a controlled message when loading fails and retries on demand', async () => {
    const fixture = TestBed.createComponent(ProductsPageComponent);
    fixture.detectChanges();

    httpTesting
      .expectOne(productsUrl)
      .flush({ code: 'INTERNAL_ERROR', message: 'Erro interno do servidor.' }, {
        status: 500,
        statusText: 'Internal Server Error'
      });
    await settle(fixture);

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('Não foi possível carregar os produtos. Tente novamente.');
    expect(text).not.toContain('Http failure');
    expect(text).not.toContain(apiConfig.inventoryApiUrl);

    const retry = fixture.nativeElement.querySelector('button') as HTMLButtonElement;
    expect(retry.textContent).toContain('Tentar novamente');
    retry.click();
    await settle(fixture);

    httpTesting.expectOne(productsUrl).flush([productFixture({ id: 1 })]);
    await settle(fixture);
    expect(fixture.nativeElement.textContent).toContain('PROD-001');
  });

  it('links to the product registration page', () => {
    const fixture = TestBed.createComponent(ProductsPageComponent);
    fixture.detectChanges();
    httpTesting.expectOne(productsUrl).flush([]);

    const link = fixture.nativeElement.querySelector('a[href="/products/new"]') as HTMLElement;
    expect(link.textContent).toContain('Novo produto');
  });
});

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

async function settle(fixture: ComponentFixture<ProductsPageComponent>): Promise<void> {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}
