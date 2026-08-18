export type InvoiceStatus = 'OPEN' | 'CLOSED';

const invoiceStatusLabels: Record<InvoiceStatus, string> = {
  OPEN: 'Aberta',
  CLOSED: 'Fechada'
};

export function invoiceStatusLabel(status: InvoiceStatus): string {
  return invoiceStatusLabels[status];
}

export interface InvoiceSummary {
  id: number;
  number: number;
  status: InvoiceStatus;
  createdAt: string;
  closedAt: string | null;
}

export interface Invoice extends InvoiceSummary {
  items: InvoiceItem[];
}

export interface InvoiceItem {
  id: number;
  productId: number;
  productCode: string;
  productDescription: string;
  quantity: number;
}

export interface CreateInvoiceRequest {
  items: CreateInvoiceItemRequest[];
}

export interface CreateInvoiceItemRequest {
  productId: number;
  quantity: number;
}
