import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { apiConfig } from '../../../core/config/api.config';
import { CreateProductRequest, Product } from '../models/product';

@Injectable({ providedIn: 'root' })
export class ProductService {
  private readonly httpClient = inject(HttpClient);
  private readonly productsUrl = `${apiConfig.inventoryApiUrl}/api/products`;

  getProducts(): Observable<Product[]> {
    return this.httpClient.get<Product[]>(this.productsUrl);
  }

  createProduct(request: CreateProductRequest): Observable<Product> {
    return this.httpClient.post<Product>(this.productsUrl, request);
  }
}
