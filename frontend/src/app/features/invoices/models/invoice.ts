export type InvoiceStatus = 'OPEN' | 'CLOSED';

const invoiceStatusLabels: Record<InvoiceStatus, string> = {
  OPEN: 'Aberta',
  CLOSED: 'Fechada'
};

export function invoiceStatusLabel(status: InvoiceStatus): string {
  return invoiceStatusLabels[status];
}

interface InvoiceBase {
  id: number;
  number: number;
  status: InvoiceStatus;
  createdAt: string;
  closedAt: string | null;
}

export interface InvoiceSummary extends InvoiceBase {
  productCodes: string[];
  totalInCents: number;
}

export interface Invoice extends InvoiceBase {
  items: InvoiceItem[];
}

export interface InvoiceItem {
  id: number;
  productId: number;
  productCode: string;
  productDescription: string;
  unitPriceInCents: number;
  quantity: number;
}

export interface CreateInvoiceRequest {
  items: CreateInvoiceItemRequest[];
}

export interface CreateInvoiceItemRequest {
  productId: number;
  quantity: number;
}
