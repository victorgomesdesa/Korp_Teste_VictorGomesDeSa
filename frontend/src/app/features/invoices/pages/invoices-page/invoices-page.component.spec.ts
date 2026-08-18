import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { apiConfig } from '../../../../core/config/api.config';
import { InvoiceSummary } from '../../models/invoice';
import { InvoicesPageComponent } from './invoices-page.component';

describe('InvoicesPageComponent', () => {
  const invoicesUrl = `${apiConfig.billingApiUrl}/api/invoices`;
  let httpTesting: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [InvoicesPageComponent],
      providers: [provideHttpClient(), provideHttpClientTesting(), provideRouter([])]
    }).compileComponents();
    httpTesting = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpTesting.verify());

  it('shows the loading state while the invoices are pending', () => {
    const fixture = TestBed.createComponent(InvoicesPageComponent);
    fixture.detectChanges();

    expect(fixture.nativeElement.textContent).toContain('Carregando notas fiscais...');

    httpTesting.expectOne(invoicesUrl).flush([]);
  });

  it('lists invoices translating the status and formatting the dates', async () => {
    const fixture = TestBed.createComponent(InvoicesPageComponent);
    fixture.detectChanges();

    httpTesting.expectOne(invoicesUrl).flush([
      invoiceFixture({ id: 1, number: 1001, status: 'OPEN' }),
      invoiceFixture({
        id: 2,
        number: 1002,
        status: 'CLOSED',
        closedAt: '2026-08-17T15:30:00Z'
      })
    ]);
    await settle(fixture);

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('1001');
    expect(text).toContain('Aberta');
    expect(text).toContain('1002');
    expect(text).toContain('Fechada');
    expect(text).toContain('17/08/2026');
    expect(text).not.toContain('OPEN');
    expect(text).not.toContain('CLOSED');
  });

  it('shows the empty state when there is no invoice', async () => {
    const fixture = TestBed.createComponent(InvoicesPageComponent);
    fixture.detectChanges();

    httpTesting.expectOne(invoicesUrl).flush([]);
    await settle(fixture);

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('Nenhuma nota fiscal cadastrada.');
    expect(text).toContain('Criar primeira nota');
  });

  it('shows a controlled message when loading fails and retries on demand', async () => {
    const fixture = TestBed.createComponent(InvoicesPageComponent);
    fixture.detectChanges();

    httpTesting.expectOne(invoicesUrl).flush(
      { code: 'INTERNAL_ERROR', message: 'Erro interno do servidor.' },
      { status: 500, statusText: 'Internal Server Error' }
    );
    await settle(fixture);

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('Não foi possível carregar as notas fiscais. Tente novamente.');
    expect(text).not.toContain('Http failure');
    expect(text).not.toContain(apiConfig.billingApiUrl);

    (fixture.nativeElement.querySelector('button') as HTMLButtonElement).click();
    await settle(fixture);

    httpTesting.expectOne(invoicesUrl).flush([invoiceFixture()]);
    await settle(fixture);
    expect(fixture.nativeElement.textContent).toContain('1001');
  });

  it('links to the creation page and to each invoice detail', async () => {
    const fixture = TestBed.createComponent(InvoicesPageComponent);
    fixture.detectChanges();

    httpTesting.expectOne(invoicesUrl).flush([invoiceFixture({ id: 7 })]);
    await settle(fixture);

    const newInvoice = fixture.nativeElement.querySelector('a[href="/invoices/new"]') as HTMLElement;
    expect(newInvoice.textContent).toContain('Nova nota');

    const detail = fixture.nativeElement.querySelector('a[href="/invoices/7"]') as HTMLElement;
    expect(detail.textContent).toContain('Visualizar');
  });
});

function invoiceFixture(invoice: Partial<InvoiceSummary> = {}): InvoiceSummary {
  return {
    id: 1,
    number: 1001,
    status: 'OPEN',
    createdAt: '2026-08-17T12:00:00Z',
    closedAt: null,
    ...invoice
  };
}

async function settle(fixture: ComponentFixture<InvoicesPageComponent>): Promise<void> {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}
