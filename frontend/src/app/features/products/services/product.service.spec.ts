import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { apiConfig } from '../../../core/config/api.config';
import { ProductService } from './product.service';

describe('ProductService', () => {
  const productsUrl = `${apiConfig.inventoryApiUrl}/api/products`;
  let httpTesting: HttpTestingController;
  let productService: ProductService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()]
    });
    httpTesting = TestBed.inject(HttpTestingController);
    productService = TestBed.inject(ProductService);
  });

  afterEach(() => httpTesting.verify());

  it('requests the Inventory product list', () => {
    const products = [
      {
        id: 1,
        code: 'PROD-001',
        description: 'Teclado Mecânico',
        balance: 10,
        createdAt: '2026-08-17T12:00:00Z',
        updatedAt: '2026-08-17T12:00:00Z'
      }
    ];

    let received: unknown;
    productService.getProducts().subscribe((result) => (received = result));

    const request = httpTesting.expectOne(productsUrl);
    expect(request.request.method).toBe('GET');
    request.flush(products);

    expect(received).toEqual(products);
  });

  it('creates a product without client-generated fields', () => {
    productService
      .createProduct({ code: 'PROD-005', description: 'Webcam', balance: 3, priceInCents: 14990 })
      .subscribe();

    const request = httpTesting.expectOne(productsUrl);
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual({
      code: 'PROD-005',
      description: 'Webcam',
      balance: 3,
      priceInCents: 14990
    });
    request.flush({});
  });
});
