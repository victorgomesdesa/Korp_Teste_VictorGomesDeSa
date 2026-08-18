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

  it('closes the invoice, prints once and drops the idempotency key', async () => {
    const print = vi.spyOn(window, 'print').mockImplementation(() => undefined);
    const fixture = render('15');
    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    printButton(fixture).click();
    await settle(fixture);
    expect(printButton(fixture).disabled).toBe(true);
    expect(printButton(fixture).textContent).toContain('Processando...');

    const close = httpTesting.expectOne(`${invoicesUrl}/15/close`);
    close.flush(invoiceFixture({ status: 'CLOSED', closedAt: '2026-08-17T15:30:00Z' }));
    await settle(fixture);

    expect(fixture.nativeElement.textContent).toContain('Fechada');
    expect(fixture.nativeElement.querySelector('button')).toBeNull();
    expect(document.body.textContent).toContain('Nota fiscal fechada com sucesso.');
    expect(print).toHaveBeenCalledTimes(1);
    print.mockRestore();
  });

  it('ignores a second click while the close request is pending', async () => {
    const fixture = render('15');
    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    printButton(fixture).click();
    printButton(fixture).click();
    fixture.componentInstance.closeInvoice();
    await settle(fixture);

    httpTesting.expectOne(`${invoicesUrl}/15/close`).flush(invoiceFixture({ status: 'CLOSED' }));
    await settle(fixture);
  });

  it('keeps the same idempotency key when the Inventory is unavailable', async () => {
    const print = vi.spyOn(window, 'print').mockImplementation(() => undefined);
    const fixture = render('15');
    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    printButton(fixture).click();
    await settle(fixture);
    const first = httpTesting.expectOne(`${invoicesUrl}/15/close`);
    const firstKey = first.request.headers.get('Idempotency-Key');
    first.flush(
      { code: 'INVENTORY_SERVICE_UNAVAILABLE', message: 'Serviço de estoque indisponível.' },
      { status: 503, statusText: 'Service Unavailable' }
    );
    await settle(fixture);

    expect(document.body.textContent).toContain('Não foi possível concluir a operação. Tente novamente.');
    expect(fixture.componentInstance.invoice()?.status).toBe('OPEN');
    expect(print).not.toHaveBeenCalled();
    expect(printButton(fixture).disabled).toBe(false);

    printButton(fixture).click();
    await settle(fixture);
    const retry = httpTesting.expectOne(`${invoicesUrl}/15/close`);
    expect(retry.request.headers.get('Idempotency-Key')).toBe(firstKey);

    retry.flush(invoiceFixture({ status: 'CLOSED' }));
    await settle(fixture);
    print.mockRestore();
  });

  it('starts a new logical operation after insufficient stock', async () => {
    const print = vi.spyOn(window, 'print').mockImplementation(() => undefined);
    const fixture = render('15');
    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    printButton(fixture).click();
    await settle(fixture);
    const first = httpTesting.expectOne(`${invoicesUrl}/15/close`);
    const firstKey = first.request.headers.get('Idempotency-Key');
    first.flush(
      { code: 'INSUFFICIENT_STOCK', message: 'Estoque insuficiente para fechar a nota fiscal.' },
      { status: 409, statusText: 'Conflict' }
    );
    await settle(fixture);

    // A mensagem segura do backend é reaproveitada em vez de um texto genérico.
    expect(document.body.textContent).toContain('Estoque insuficiente para fechar a nota fiscal.');
    expect(fixture.componentInstance.invoice()?.status).toBe('OPEN');
    expect(print).not.toHaveBeenCalled();

    printButton(fixture).click();
    await settle(fixture);
    const retry = httpTesting.expectOne(`${invoicesUrl}/15/close`);
    expect(retry.request.headers.get('Idempotency-Key')).not.toBe(firstKey);

    retry.flush(invoiceFixture({ status: 'CLOSED' }));
    await settle(fixture);
    print.mockRestore();
  });

  it('reloads the invoice when it was already closed by another operation', async () => {
    const print = vi.spyOn(window, 'print').mockImplementation(() => undefined);
    const fixture = render('15');
    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    printButton(fixture).click();
    await settle(fixture);
    httpTesting.expectOne(`${invoicesUrl}/15/close`).flush(
      { code: 'INVOICE_ALREADY_CLOSED', message: 'Nota fiscal já fechada.' },
      { status: 409, statusText: 'Conflict' }
    );
    await settle(fixture);

    httpTesting
      .expectOne(`${invoicesUrl}/15`)
      .flush(invoiceFixture({ status: 'CLOSED', closedAt: '2026-08-17T15:30:00Z' }));
    await settle(fixture);

    expect(fixture.nativeElement.textContent).toContain('Fechada');
    expect(document.body.textContent).toContain('Esta nota já foi fechada.');
    expect(print).not.toHaveBeenCalled();
    print.mockRestore();
  });

  it('reports a close already in progress and reloads without printing', async () => {
    const print = vi.spyOn(window, 'print').mockImplementation(() => undefined);
    const fixture = render('15');
    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    printButton(fixture).click();
    await settle(fixture);
    httpTesting.expectOne(`${invoicesUrl}/15/close`).flush(
      { code: 'INVOICE_CLOSE_ALREADY_IN_PROGRESS', message: 'Fechamento em andamento.' },
      { status: 409, statusText: 'Conflict' }
    );
    await settle(fixture);

    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    expect(document.body.textContent).toContain('Esta nota já está sendo processada.');
    expect(fixture.componentInstance.invoice()?.status).toBe('OPEN');
    expect(print).not.toHaveBeenCalled();
    print.mockRestore();
  });

  it('does not offer the close action for a closed invoice', async () => {
    const fixture = render('15');
    httpTesting
      .expectOne(`${invoicesUrl}/15`)
      .flush(invoiceFixture({ status: 'CLOSED', closedAt: '2026-08-17T15:30:00Z' }));
    await settle(fixture);

    expect(fixture.nativeElement.textContent).not.toContain('Imprimir Nota');
  });

  it('reports a reused idempotency key without exposing internals', async () => {
    const print = vi.spyOn(window, 'print').mockImplementation(() => undefined);
    const fixture = render('15');
    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    printButton(fixture).click();
    await settle(fixture);
    httpTesting.expectOne(`${invoicesUrl}/15/close`).flush(
      { code: 'IDEMPOTENCY_KEY_REUSED', message: 'Idempotency-Key já utilizada em outra operação.' },
      { status: 409, statusText: 'Conflict' }
    );
    await settle(fixture);

    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    const feedback = document.body.textContent ?? '';
    expect(feedback).toContain('Não foi possível reutilizar esta operação.');
    expect(feedback).not.toContain('fingerprint');
    expect(feedback).not.toContain('Idempotency-Key');
    expect(print).not.toHaveBeenCalled();
    print.mockRestore();
  });

  it('shows a connection message when the Billing cannot be reached during the close', async () => {
    const print = vi.spyOn(window, 'print').mockImplementation(() => undefined);
    const fixture = render('15');
    httpTesting.expectOne(`${invoicesUrl}/15`).flush(invoiceFixture());
    await settle(fixture);

    printButton(fixture).click();
    await settle(fixture);
    httpTesting
      .expectOne(`${invoicesUrl}/15/close`)
      .error(new ProgressEvent('error'), { status: 0 });
    await settle(fixture);

    expect(document.body.textContent).toContain('Não foi possível conectar ao serviço. Tente novamente.');
    expect(fixture.componentInstance.invoice()?.status).toBe('OPEN');
    expect(print).not.toHaveBeenCalled();
    print.mockRestore();
  });

  function printButton(fixture: ComponentFixture<InvoiceDetailPageComponent>): HTMLButtonElement {
    return fixture.nativeElement.querySelector('button') as HTMLButtonElement;
  }

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
