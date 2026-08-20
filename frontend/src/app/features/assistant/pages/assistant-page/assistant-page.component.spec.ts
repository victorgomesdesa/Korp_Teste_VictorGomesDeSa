import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';

import { apiConfig } from '../../../../core/config/api.config';
import { VoiceIntent } from '../../models/voice-intent';
import { AssistantPageComponent } from './assistant-page.component';

describe('AssistantPageComponent', () => {
  const intentUrl = `${apiConfig.voiceApiUrl}/api/voice/intent`;
  const productsUrl = `${apiConfig.inventoryApiUrl}/api/products`;
  const invoicesUrl = `${apiConfig.billingApiUrl}/api/invoices`;
  let httpTesting: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AssistantPageComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([
          { path: 'products', component: StubComponent },
          { path: 'invoices/:id', component: StubComponent }
        ])
      ]
    }).compileComponents();
    httpTesting = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpTesting.verify());

  it('shows the interpreted intent and writes nothing before the confirmation', async () => {
    const fixture = await interpret({
      acao: 'criar_produto',
      code: 'PROD-020',
      description: 'Monitor',
      balance: 5,
      price: 899
    });

    expect(fixture.nativeElement.textContent).toContain('Cadastrar produto');
    expect(fixture.nativeElement.textContent).toContain('PROD-020');
    expect(fixture.nativeElement.textContent).toContain('Monitor');
    httpTesting.expectNone(productsUrl);
  });

  it('creates the product only after the confirmation', async () => {
    const fixture = await interpret({
      acao: 'criar_produto',
      code: 'PROD-020',
      description: 'Monitor',
      balance: 5,
      price: 899
    });
    const router = TestBed.inject(Router);

    confirm(fixture);
    await settle(fixture);

    const request = httpTesting.expectOne(productsUrl);
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual({
      code: 'PROD-020',
      description: 'Monitor',
      balance: 5,
      priceInCents: 89900
    });

    request.flush({ id: 9, code: 'PROD-020', description: 'Monitor', balance: 5, priceInCents: 89900 });
    await settle(fixture);

    expect(router.url).toBe('/products');
  });

  it('blocks the confirmation while a required field is missing', async () => {
    const fixture = await interpret({ acao: 'criar_produto', code: 'PROD-020' });

    expect(confirmButton(fixture).disabled).toBe(true);
    expect(fixture.nativeElement.textContent).toContain('Faltou código, nome, estoque ou valor');
  });

  it('blocks product creation with zero balance or zero price', async () => {
    const zeroBalance = await interpret({
      acao: 'criar_produto', code: 'PROD-020', description: 'Monitor', balance: 0, price: 899
    });
    expect(confirmButton(zeroBalance).disabled).toBe(true);
    expect(zeroBalance.nativeElement.textContent).toContain('O estoque deve ser maior que zero.');

    zeroBalance.destroy();
    const zeroPrice = await interpret({
      acao: 'criar_produto', code: 'PROD-020', description: 'Monitor', balance: 5, price: 0
    });
    expect(confirmButton(zeroPrice).disabled).toBe(true);
    expect(zeroPrice.nativeElement.textContent).toContain('O valor deve ser maior que zero.');
  });

  // O comando fala códigos; a nota precisa dos ids que só a listagem do Inventory conhece.
  it('resolves the spoken product codes into ids before creating the invoice', async () => {
    const fixture = await interpret({
      acao: 'criar_nota',
      itens: [{ code: 'PROD-001', quantity: 2 }]
    });

    confirm(fixture);
    await settle(fixture);

    httpTesting.expectOne(productsUrl).flush([
      { id: 4, code: 'PROD-001', description: 'Teclado', balance: 10 },
      { id: 5, code: 'PROD-002', description: 'Mouse', balance: 8 }
    ]);
    await settle(fixture);

    const request = httpTesting.expectOne(invoicesUrl);
    expect(request.request.body).toEqual({ items: [{ productId: 4, quantity: 2 }] });

    request.flush({ id: 12, number: 12, status: 'OPEN', items: [] });
    await settle(fixture);

    expect(TestBed.inject(Router).url).toBe('/invoices/12');
  });

  it('reports a spoken product that is not registered', async () => {
    const fixture = await interpret({
      acao: 'criar_nota',
      itens: [{ code: 'PROD-999', quantity: 1 }]
    });

    confirm(fixture);
    await settle(fixture);

    httpTesting
      .expectOne(productsUrl)
      .flush([{ id: 4, code: 'PROD-001', description: 'Teclado', balance: 10 }]);
    await settle(fixture);

    httpTesting.expectNone(invoicesUrl);
    expect(fixture.nativeElement.textContent).toContain('Não encontrei um produto parecido com “PROD-999”');
  });

  it('resolves a product name with a small transcription error', async () => {
    const fixture = await interpret({
      acao: 'criar_nota',
      itens: [{ code: 'tecldo mecanico', quantity: 1 }]
    });

    confirm(fixture);
    await settle(fixture);
    httpTesting.expectOne(productsUrl).flush([
      { id: 4, code: 'PROD-001', description: 'Teclado Mecânico', balance: 10, priceInCents: 19990 },
      { id: 5, code: 'PROD-002', description: 'Mouse', balance: 8, priceInCents: 5990 }
    ]);
    await settle(fixture);

    const request = httpTesting.expectOne(invoicesUrl);
    expect(request.request.body).toEqual({ items: [{ productId: 4, quantity: 1 }] });
    request.flush({ id: 12, number: 12, status: 'OPEN', items: [] });
    await settle(fixture);
  });

  it('rejects an ambiguous product name instead of guessing', async () => {
    const fixture = await interpret({
      acao: 'criar_nota',
      itens: [{ code: 'teclado', quantity: 1 }]
    });

    confirm(fixture);
    await settle(fixture);
    httpTesting.expectOne(productsUrl).flush([
      { id: 4, code: 'PROD-001', description: 'Teclado Mecânico', balance: 10, priceInCents: 19990 },
      { id: 5, code: 'PROD-002', description: 'Teclado Gamer', balance: 8, priceInCents: 29990 }
    ]);
    await settle(fixture);

    httpTesting.expectNone(invoicesUrl);
    expect(fixture.nativeElement.textContent).toContain(
      'Não encontrei um produto parecido com “teclado”'
    );
  });

  // O usuário fala o número da nota; o fechamento é feito pelo id, com Idempotency-Key própria.
  it('closes the invoice found by its number with an idempotency key', async () => {
    const fixture = await interpret({ acao: 'fechar_nota', numeroNota: 7 });

    confirm(fixture);
    await settle(fixture);

    httpTesting.expectOne(invoicesUrl).flush([
      { id: 3, number: 6, status: 'CLOSED', createdAt: '', closedAt: '' },
      { id: 4, number: 7, status: 'OPEN', createdAt: '', closedAt: null }
    ]);
    await settle(fixture);

    const request = httpTesting.expectOne(`${invoicesUrl}/4/close`);
    expect(request.request.method).toBe('POST');
    expect(request.request.headers.get('Idempotency-Key')).toBeTruthy();

    request.flush({ id: 4, number: 7, status: 'CLOSED', items: [] });
    await settle(fixture);

    expect(TestBed.inject(Router).url).toBe('/invoices/4');
  });

  it('reports an invoice number that does not exist', async () => {
    const fixture = await interpret({ acao: 'fechar_nota', numeroNota: 99 });

    confirm(fixture);
    await settle(fixture);

    httpTesting
      .expectOne(invoicesUrl)
      .flush([{ id: 4, number: 7, status: 'OPEN', createdAt: '', closedAt: null }]);
    await settle(fixture);

    expect(fixture.nativeElement.textContent).toContain('Não encontrei uma nota com esse número.');
  });

  it('translates the stock failure of the closing into a controlled message', async () => {
    const fixture = await interpret({ acao: 'fechar_nota', numeroNota: 7 });

    confirm(fixture);
    await settle(fixture);

    httpTesting
      .expectOne(invoicesUrl)
      .flush([{ id: 4, number: 7, status: 'OPEN', createdAt: '', closedAt: null }]);
    await settle(fixture);

    httpTesting.expectOne(`${invoicesUrl}/4/close`).flush(
      { code: 'INSUFFICIENT_STOCK', message: 'Estoque insuficiente.' },
      { status: 409, statusText: 'Conflict' }
    );
    await settle(fixture);

    expect(fixture.nativeElement.textContent).toContain(
      'Estoque insuficiente para fechar a nota fiscal.'
    );
  });

  it('does not offer a confirmation for an unrecognized command', async () => {
    const fixture = await interpret({ acao: 'desconhecido' });

    expect(fixture.nativeElement.textContent).toContain('Não reconheci uma ação válida');
    expect(fixture.nativeElement.querySelectorAll('button').length).toBe(2);
  });

  it('shows a connection message when the assistant is unreachable', async () => {
    const fixture = TestBed.createComponent(AssistantPageComponent);
    fixture.detectChanges();

    fixture.componentInstance.form.controls.text.setValue('cadastrar produto');
    fixture.componentInstance.sendText();
    await settle(fixture);

    httpTesting.expectOne(intentUrl).error(new ProgressEvent('error'), { status: 0 });
    await settle(fixture);

    expect(fixture.nativeElement.textContent).toContain(
      'Não foi possível conectar ao assistente.'
    );
  });

  async function interpret(intent: VoiceIntent): Promise<ComponentFixture<AssistantPageComponent>> {
    const fixture = TestBed.createComponent(AssistantPageComponent);
    fixture.detectChanges();

    fixture.componentInstance.form.controls.text.setValue('comando falado');
    fixture.componentInstance.sendText();
    await settle(fixture);

    httpTesting.expectOne(intentUrl).flush({ transcript: 'comando falado', intent });
    await settle(fixture);

    return fixture;
  }
});

@Component({ template: '' })
class StubComponent {}

function confirmButton(fixture: ComponentFixture<AssistantPageComponent>): HTMLButtonElement {
  const buttons = Array.from(
    fixture.nativeElement.querySelectorAll('button')
  ) as HTMLButtonElement[];
  return buttons.find((button) => button.textContent?.includes('Confirmar'))!;
}

function confirm(fixture: ComponentFixture<AssistantPageComponent>): void {
  confirmButton(fixture).click();
}

async function settle(fixture: ComponentFixture<AssistantPageComponent>): Promise<void> {
  await fixture.whenStable();
  fixture.detectChanges();
}
