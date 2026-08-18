import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    pathMatch: 'full',
    redirectTo: 'products'
  },
  {
    path: 'products/new',
    loadComponent: () =>
      import('./features/products/pages/product-create-page/product-create-page.component').then(
        (component) => component.ProductCreatePageComponent
      )
  },
  {
    path: 'products',
    loadComponent: () =>
      import('./features/products/pages/products-page/products-page.component').then(
        (component) => component.ProductsPageComponent
      )
  },
  {
    path: 'invoices/new',
    loadComponent: () =>
      import('./features/invoices/pages/invoice-create-page/invoice-create-page.component').then(
        (component) => component.InvoiceCreatePageComponent
      )
  },
  {
    path: 'invoices',
    loadComponent: () =>
      import('./features/invoices/pages/invoices-page/invoices-page.component').then(
        (component) => component.InvoicesPageComponent
      )
  },
  {
    path: 'invoices/:id',
    loadComponent: () =>
      import('./features/invoices/pages/invoice-detail-page/invoice-detail-page.component').then(
        (component) => component.InvoiceDetailPageComponent
      )
  }
];
