export interface Product {
  id: number;
  code: string;
  description: string;
  balance: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateProductRequest {
  code: string;
  description: string;
  balance: number;
}
