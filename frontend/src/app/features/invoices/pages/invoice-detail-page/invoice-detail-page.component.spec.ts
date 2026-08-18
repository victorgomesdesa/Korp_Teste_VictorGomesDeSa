import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { apiConfig } from '../../../../core/config/api.config';
import { Invoice } from '../../models/invoice';
import { InvoiceDetailPageComponent } from './invoice-detail-page.component';

describe('InvoiceDetailPageComponent', () => {
  const invoicesUrl = `${apiConfig.billingApiUrl}/api/invoices`;
  let httpTesting: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [InvoiceDetailPageComponent],
      providers: [provideHttpClient(), provideHttpClientTesting(), provideRouter([])]
    }).compileComponents();
    httpTesting = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpTesting.verify());

  it('shows the loading state while the invoice is pending', () => {
    const fixture = render('15');

    expect(fixture.nativeElement.textContent).toContain('Carregando nota fiscal...');

    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
  });

  it('shows an open invoice with the persisted item snapshots', async () => {
    const fixture = render('15');

    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('1001');
    expect(text).toContain('Aberta');
    expect(text).toContain('17/08/2026');
    expect(text).toContain('PROD-001');
    expect(text).toContain('Teclado Mecânico');
    expect(text).not.toContain('Fechada em');
  });

  it('shows a closed invoice with the closing date', async () => {
    const fixture = render('15');

    httpTesting.expectOne(`${invoicesUrl}/15`).flush(
      invoiceFixture({ status: 'CLOSED', closedAt: '2026-08-17T15:30:00Z' })
    );
    await settle(fixture);

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('Fechada');
    expect(text).toContain('Fechada em');
    expect(text).toContain('17/08/2026');
  });

  it('does not ask the Inventory to rebuild the item snapshots', async () => {
    const fixture = render('15');

    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    httpTesting.expectNone(`${apiConfig.inventoryApiUrl}/api/products`);
    httpTesting.expectNone(`${apiConfig.inventoryApiUrl}/api/products/1`);
  });

  it('shows a friendly state when the invoice does not exist', async () => {
    const fixture = render('999');

    httpTesting
      .expectOne(`${invoicesUrl}/999`)
      .flush({ code: 'INVOICE_NOT_FOUND', message: 'Nota fiscal não encontrada.' }, {
        status: 404,
        statusText: 'Not Found'
      });
    await settle(fixture);

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('Nota fiscal não encontrada.');
    expect(text).toContain('Voltar para notas');
    expect(text).not.toContain('Carregando nota fiscal...');
  });

  it('shows a controlled message on unexpected failures', async () => {
    const fixture = render('15');

    httpTesting.expectOne(`${invoicesUrl}/15`).flush(
      { code: 'INTERNAL_ERROR', message: 'Erro interno do servidor.' },
      { status: 500, statusText: 'Internal Server Error' }
    );
    await settle(fixture);

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('Não foi possível carregar a nota fiscal. Tente novamente.');
    expect(text).not.toContain('Http failure');
    expect(text).not.toContain(apiConfig.billingApiUrl);
  });

  function render(id: string): ComponentFixture<InvoiceDetailPageComponent> {
    const fixture = TestBed.createComponent(InvoiceDetailPageComponent);
    fixture.componentRef.setInput('id', id);
    fixture.detectChanges();
    return fixture;
  }
});

function invoiceFixture(invoice: Partial<Invoice> = {}): Invoice {
  return {
    id: 15,
    number: 1001,
    status: 'OPEN',
    createdAt: '2026-08-17T12:00:00Z',
    closedAt: null,
    items: [
      {
        id: 1,
        productId: 1,
        productCode: 'PROD-001',
        productDescription: 'Teclado Mecânico',
        quantity: 2
      }
    ],
    ...invoice
  };
}

async function settle(fixture: ComponentFixture<InvoiceDetailPageComponent>): Promise<void> {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}
