import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { apiConfig } from '../../../core/config/api.config';
import { InvoiceService } from './invoice.service';

describe('InvoiceService', () => {
  const invoicesUrl = `${apiConfig.billingApiUrl}/api/invoices`;
  let httpTesting: HttpTestingController;
  let invoiceService: InvoiceService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()]
    });
    httpTesting = TestBed.inject(HttpTestingController);
    invoiceService = TestBed.inject(InvoiceService);
  });

  afterEach(() => httpTesting.verify());

  it('requests the Billing invoice list', () => {
    const invoices = [
      { id: 1, number: 1001, status: 'OPEN', createdAt: '2026-08-17T12:00:00Z', closedAt: null }
    ];

    let received: unknown;
    invoiceService.getInvoices().subscribe((result) => (received = result));

    const request = httpTesting.expectOne(invoicesUrl);
    expect(request.request.method).toBe('GET');
    request.flush(invoices);

    expect(received).toEqual(invoices);
  });

  it('requests a single invoice by id', () => {
    invoiceService.getInvoice(15).subscribe();

    const request = httpTesting.expectOne(`${invoicesUrl}/15`);
    expect(request.request.method).toBe('GET');
    request.flush({});
  });

  it('creates an invoice sending only product id and quantity', () => {
    invoiceService
      .createInvoice({
        items: [
          { productId: 1, quantity: 2 },
          { productId: 2, quantity: 1 }
        ]
      })
      .subscribe();

    const request = httpTesting.expectOne(invoicesUrl);
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual({
      items: [
        { productId: 1, quantity: 2 },
        { productId: 2, quantity: 1 }
      ]
    });

    const sentItemKeys = Object.keys((request.request.body as { items: object[] }).items[0]);
    expect(sentItemKeys).toEqual(['productId', 'quantity']);

    request.flush({});
  });

  it('closes an invoice sending the idempotency key and no body', () => {
    invoiceService.closeInvoice(15, 'key-1').subscribe();

    const request = httpTesting.expectOne(`${invoicesUrl}/15/close`);
    expect(request.request.method).toBe('POST');
    expect(request.request.headers.get('Idempotency-Key')).toBe('key-1');
    expect(request.request.body).toBeNull();
    request.flush({});
  });
});
