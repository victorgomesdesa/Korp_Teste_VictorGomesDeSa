import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router, withComponentInputBinding } from '@angular/router';

import { AppComponent } from './app.component';
import { routes } from './app.routes';

describe('AppComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AppComponent],
      providers: [
        provideRouter(routes, withComponentInputBinding()),
        provideHttpClient(),
        provideHttpClientTesting()
      ]
    }).compileComponents();
  });

  it('creates the application', () => {
    const fixture = TestBed.createComponent(AppComponent);
    expect(fixture.componentInstance).toBeTruthy();
  });

  it('redirects the root route to products', async () => {
    const fixture = TestBed.createComponent(AppComponent);
    const router = TestBed.inject(Router);

    await router.navigateByUrl('/');
    fixture.detectChanges();
    await fixture.whenStable();

    expect(router.url).toBe('/products');
    expect(fixture.nativeElement.textContent).toContain('Produtos');
  });

  it('resolves every application route to its page', async () => {
    const routes = [
      { path: '/products', heading: 'Produtos' },
      { path: '/products/new', heading: 'Novo produto' },
      { path: '/invoices', heading: 'Notas fiscais' },
      { path: '/invoices/new', heading: 'Nova nota fiscal' }
    ];

    const fixture = TestBed.createComponent(AppComponent);
    const router = TestBed.inject(Router);

    for (const route of routes) {
      await router.navigateByUrl(route.path);
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();

      expect(router.url).toBe(route.path);
      expect(fixture.nativeElement.textContent).toContain(route.heading);
    }
  });

  it('loads the invoice detail page', async () => {
    const fixture = TestBed.createComponent(AppComponent);
    const router = TestBed.inject(Router);

    await router.navigateByUrl('/invoices/42');
    fixture.detectChanges();
    await fixture.whenStable();

    expect(fixture.nativeElement.textContent).toContain('Detalhes da nota fiscal');
  });
});
