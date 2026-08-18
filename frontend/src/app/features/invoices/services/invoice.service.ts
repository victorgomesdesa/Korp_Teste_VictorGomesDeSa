import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { apiConfig } from '../../../core/config/api.config';
import { CreateInvoiceRequest, Invoice, InvoiceSummary } from '../models/invoice';

@Injectable({ providedIn: 'root' })
export class InvoiceService {
  private readonly httpClient = inject(HttpClient);
  private readonly invoicesUrl = `${apiConfig.billingApiUrl}/api/invoices`;

  getInvoices(): Observable<InvoiceSummary[]> {
    return this.httpClient.get<InvoiceSummary[]>(this.invoicesUrl);
  }

  getInvoice(id: number): Observable<Invoice> {
    return this.httpClient.get<Invoice>(`${this.invoicesUrl}/${id}`);
  }

  createInvoice(request: CreateInvoiceRequest): Observable<Invoice> {
    return this.httpClient.post<Invoice>(this.invoicesUrl, request);
  }

  // A Idempotency-Key identifica a operação lógica de fechamento e é controlada pela página, que
  // precisa reenviar a mesma chave quando o resultado da tentativa anterior ficou ambíguo.
  closeInvoice(id: number, idempotencyKey: string): Observable<Invoice> {
    return this.httpClient.post<Invoice>(`${this.invoicesUrl}/${id}/close`, null, {
      headers: { 'Idempotency-Key': idempotencyKey }
    });
  }
}
