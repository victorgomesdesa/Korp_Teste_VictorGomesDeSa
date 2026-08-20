export interface Product {
  id: number;
  code: string;
  description: string;
  balance: number;
  priceInCents: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateProductRequest {
  code: string;
  description: string;
  balance: number;
  priceInCents: number;
}
