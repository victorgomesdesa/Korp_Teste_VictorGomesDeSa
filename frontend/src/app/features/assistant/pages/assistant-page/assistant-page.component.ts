import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Router } from '@angular/router';
import { finalize, switchMap } from 'rxjs';

import { getApiError, isConnectionError } from '../../../../core/http/error.interceptor';
import { ErrorMessageComponent } from '../../../../shared/components/error-message/error-message.component';
import { LoadingComponent } from '../../../../shared/components/loading/loading.component';
import { Invoice } from '../../../invoices/models/invoice';
import { InvoiceService } from '../../../invoices/services/invoice.service';
import { Product } from '../../../products/models/product';
import { ProductService } from '../../../products/services/product.service';
import { VoiceIntent, VoiceInterpretation } from '../../models/voice-intent';
import { VoiceAssistantService } from '../../services/voice-assistant.service';

@Component({
  selector: 'app-assistant-page',
  imports: [
    ErrorMessageComponent,
    LoadingComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatIconModule,
    ReactiveFormsModule
  ],
  template: `
    <section class="page" aria-labelledby="assistant-title">
      <p class="page-eyebrow">Assistente</p>
      <h1 id="assistant-title">Converse com o sistema</h1>

      <div class="chat-shell">
        <header class="chat-header">
          <span class="assistant-avatar" aria-hidden="true">IA</span>
          <div>
            <strong>Assistente Korp</strong>
            <span>Pronto para ajudar</span>
          </div>
        </header>

        <div class="chat-messages" aria-live="polite">
          <div class="message assistant-message">
            Olá! Posso cadastrar produtos, criar notas e fechar notas fiscais. Para o código do
            produto, diga apenas o número. Exemplo: “Cadastrar produto 20, nome Monitor, estoque 5,
            valor 899 reais”. Só executo depois da sua confirmação.
          </div>

          @if (interpretation(); as result) {
            <div class="message user-message">{{ result.transcript }}</div>
            <article class="message assistant-message assistant-result">
              <strong>Entendi o seguinte:</strong>
              @if (summary(); as summaryLines) {
                <dl>
                  @for (line of summaryLines; track line.label) {
                    <div><dt>{{ line.label }}</dt><dd>{{ line.value }}</dd></div>
                  }
                </dl>
                @if (missing(); as missingMessage) {
                  <app-error-message [message]="missingMessage" />
                }
                <div class="assistant-confirm">
                  <button mat-stroked-button type="button" (click)="discard()">
                    <mat-icon aria-hidden="true">close</mat-icon>Descartar
                  </button>
                  <button mat-flat-button type="button" [disabled]="busy() || missing() !== null" (click)="confirm()">
                    <mat-icon aria-hidden="true">check</mat-icon>Confirmar
                  </button>
                </div>
              } @else {
                <app-error-message message="Não reconheci uma ação válida nesse comando. Tente novamente." />
              }
            </article>
          }

          @if (recording()) {
            <div class="message assistant-message assistant-status" role="status">Gravando... fale seu comando.</div>
          }
          @if (interpreting()) { <app-loading label="Interpretando o comando..." /> }
          @if (executing()) { <app-loading label="Executando a ação confirmada..." /> }
          @if (errorMessage()) { <app-error-message [message]="errorMessage()!" /> }
        </div>

        <form class="chat-composer" [formGroup]="form" (ngSubmit)="sendText()">
          <button
            mat-icon-button
            type="button"
            [disabled]="busy()"
            [attr.aria-label]="recording() ? 'Parar gravação' : 'Gravar comando por voz'"
            [class.recording-button]="recording()"
            (click)="toggleRecording()"
          >
            <mat-icon aria-hidden="true">{{ recording() ? 'stop_circle' : 'mic' }}</mat-icon>
          </button>
          <mat-form-field subscriptSizing="dynamic">
            <mat-label>Digite seu comando</mat-label>
            <input matInput formControlName="text" autocomplete="off" />
          </mat-form-field>
          <button mat-flat-button type="submit" [disabled]="busy()">
            <mat-icon aria-hidden="true">send</mat-icon>Enviar
          </button>
        </form>
      </div>
    </section>
  `,
  styles: `
    .chat-shell {
      max-width: 52rem;
      min-height: 34rem;
      margin-top: 1.5rem;
      border: 1px solid var(--mat-sys-outline-variant);
      border-radius: 1rem;
      overflow: hidden;
      background: var(--mat-sys-surface-container-low);
      display: flex;
      flex-direction: column;
    }
    .chat-header, .chat-composer { display: flex; align-items: center; gap: 1rem; padding: 1rem; background: var(--mat-sys-surface); }
    .chat-header { border-bottom: 1px solid var(--mat-sys-outline-variant); }
    .chat-header div { display: flex; flex-direction: column; }
    .chat-header span { color: var(--mat-sys-on-surface-variant); font-size: 0.8rem; }
    .assistant-avatar { display: grid; place-items: center; width: 2.5rem; height: 2.5rem; border-radius: 50%; background: var(--mat-sys-primary); color: var(--mat-sys-on-primary) !important; font-weight: 700; }
    .chat-messages { flex: 1; display: flex; flex-direction: column; gap: 1rem; padding: 1.25rem; }
    .message { max-width: min(80%, 38rem); padding: 0.85rem 1rem; border-radius: 1rem; line-height: 1.45; }
    .assistant-message { align-self: flex-start; background: var(--mat-sys-surface-container-high); border-bottom-left-radius: 0.25rem; }
    .user-message { align-self: flex-end; background: var(--mat-sys-primary); color: var(--mat-sys-on-primary); border-bottom-right-radius: 0.25rem; }
    .assistant-result { width: min(80%, 38rem); }
    .chat-composer { border-top: 1px solid var(--mat-sys-outline-variant); }
    .chat-composer mat-form-field { flex: 1; }
    .assistant-status { color: var(--mat-sys-primary); }
    dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr)); gap: 0.75rem; margin: 1rem 0; }
    dl div { min-width: 0; }
    dt { color: var(--mat-sys-on-surface-variant); font: var(--mat-sys-label-large); }
    dd { margin: 0.2rem 0 0; font: var(--mat-sys-title-medium); overflow-wrap: anywhere; }
    .assistant-confirm { display: flex; justify-content: flex-end; gap: 1rem; margin-top: 1.25rem; }
    .recording-button { color: var(--mat-sys-error); background: var(--mat-sys-error-container); }
    @media (max-width: 600px) {
      .chat-composer { flex-wrap: wrap; }
      .chat-composer mat-form-field { order: -1; flex-basis: 100%; }
      .message, .assistant-result { max-width: 92%; width: auto; }
    }
  `
})
export class AssistantPageComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly voiceAssistantService = inject(VoiceAssistantService);
  private readonly productService = inject(ProductService);
  private readonly invoiceService = inject(InvoiceService);
  private readonly router = inject(Router);
  private readonly snackBar = inject(MatSnackBar);

  private recorder: MediaRecorder | null = null;
  private chunks: Blob[] = [];

  readonly form = this.formBuilder.nonNullable.group({ text: [''] });
  readonly recording = signal(false);
  readonly interpreting = signal(false);
  readonly executing = signal(false);
  readonly errorMessage = signal<string | null>(null);
  readonly interpretation = signal<VoiceInterpretation | null>(null);

  busy(): boolean {
    return this.interpreting() || this.executing();
  }

  // Resumo legível da intenção, para o usuário conferir antes de confirmar.
  summary(): { label: string; value: string }[] | null {
    const intent = this.interpretation()?.intent;
    if (intent === undefined || intent.acao === 'desconhecido') {
      return null;
    }

    if (intent.acao === 'criar_produto') {
      return [
        { label: 'Ação', value: 'Cadastrar produto' },
        { label: 'Código', value: intent.code ?? '—' },
        { label: 'Nome', value: intent.description ?? '—' },
        { label: 'Estoque', value: intent.balance === undefined ? '—' : String(intent.balance) },
        { label: 'Valor', value: intent.price === undefined ? '—' : formatCurrency(intent.price) }
      ];
    }

    if (intent.acao === 'criar_nota') {
      const itens = (intent.itens ?? [])
        .map((item) => `${item.code} × ${item.quantity}`)
        .join(', ');
      return [
        { label: 'Ação', value: 'Criar nota fiscal' },
        { label: 'Itens', value: itens === '' ? '—' : itens }
      ];
    }

    return [
      { label: 'Ação', value: 'Fechar nota fiscal' },
      { label: 'Número', value: intent.numeroNota === undefined ? '—' : String(intent.numeroNota) }
    ];
  }

  // Bloqueia a confirmação enquanto faltar informação obrigatória para a API.
  missing(): string | null {
    const intent = this.interpretation()?.intent;
    if (intent === undefined) {
      return null;
    }

    if (intent.acao === 'criar_produto') {
      if (!intent.code || !intent.description || intent.balance === undefined || intent.price === undefined) {
        return 'Faltou código, nome, estoque ou valor. Repita o comando informando os quatro.';
      }
      if (intent.balance <= 0) return 'O estoque deve ser maior que zero.';
      return intent.price <= 0 ? 'O valor deve ser maior que zero.' : null;
    }

    if (intent.acao === 'criar_nota') {
      return (intent.itens ?? []).length === 0
        ? 'Não identifiquei os itens da nota. Repita informando produto e quantidade.'
        : null;
    }

    if (intent.acao === 'fechar_nota') {
      return intent.numeroNota === undefined
        ? 'Não identifiquei o número da nota. Repita informando o número.'
        : null;
    }

    return null;
  }

  async toggleRecording(): Promise<void> {
    if (this.recording()) {
      this.recorder?.stop();
      return;
    }

    this.reset();
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      this.chunks = [];
      this.recorder = new MediaRecorder(stream);
      this.recorder.ondataavailable = (event) => this.chunks.push(event.data);
      this.recorder.onstop = () => {
        stream.getTracks().forEach((track) => track.stop());
        this.recording.set(false);
        this.interpretAudio(new Blob(this.chunks, { type: 'audio/webm' }));
      };
      this.recorder.start();
      this.recording.set(true);
    } catch {
      this.errorMessage.set('Não foi possível acessar o microfone. Verifique a permissão do navegador.');
    }
  }

  sendText(): void {
    const text = this.form.controls.text.value.trim();
    if (text === '' || this.busy()) {
      return;
    }

    this.reset();
    this.interpreting.set(true);
    this.voiceAssistantService
      .interpretText(text)
      .pipe(finalize(() => this.interpreting.set(false)))
      .subscribe({
        next: (result) => this.interpretation.set(result),
        error: (error: unknown) => this.errorMessage.set(interpretErrorMessage(error))
      });
  }

  discard(): void {
    this.reset();
    this.form.reset();
  }

  confirm(): void {
    const intent = this.interpretation()?.intent;
    if (intent === undefined || this.busy() || this.missing() !== null) {
      return;
    }

    this.executing.set(true);
    this.errorMessage.set(null);

    if (intent.acao === 'criar_produto') {
      this.createProduct(intent);
      return;
    }
    if (intent.acao === 'criar_nota') {
      this.createInvoice(intent);
      return;
    }
    if (intent.acao === 'fechar_nota') {
      this.closeInvoice(intent);
      return;
    }

    this.executing.set(false);
  }

  private createProduct(intent: VoiceIntent): void {
    this.productService
      .createProduct({
        code: intent.code!.trim(),
        description: intent.description!.trim(),
        balance: intent.balance!,
        priceInCents: Math.round(intent.price! * 100)
      })
      .pipe(finalize(() => this.executing.set(false)))
      .subscribe({
        next: () => {
          this.snackBar.open('Produto cadastrado com sucesso.', 'Fechar', { duration: 5000 });
          this.discard();
          void this.router.navigateByUrl('/products');
        },
        error: (error: unknown) => this.errorMessage.set(executeErrorMessage(error))
      });
  }

  // O comando fala códigos de produto; a API espera ids, então resolvemos pela listagem atual.
  private createInvoice(intent: VoiceIntent): void {
    this.productService
      .getProducts()
      .pipe(
        switchMap((products) => {
          const items = resolveItems(intent, products);
          return this.invoiceService.createInvoice({ items });
        }),
        finalize(() => this.executing.set(false))
      )
      .subscribe({
        next: (invoice) => {
          this.snackBar.open('Nota fiscal criada com sucesso.', 'Fechar', { duration: 5000 });
          this.discard();
          void this.router.navigate(['/invoices', invoice.id]);
        },
        error: (error: unknown) => this.errorMessage.set(executeErrorMessage(error))
      });
  }

  private closeInvoice(intent: VoiceIntent): void {
    this.invoiceService
      .getInvoices()
      .pipe(
        switchMap((invoices) => {
          const invoice = invoices.find((candidate) => candidate.number === intent.numeroNota);
          if (invoice === undefined) {
            throw new UnknownInvoiceError();
          }
          return this.invoiceService.closeInvoice(invoice.id, crypto.randomUUID());
        }),
        finalize(() => this.executing.set(false))
      )
      .subscribe({
        next: (invoice: Invoice) => {
          this.snackBar.open('Nota fiscal fechada com sucesso.', 'Fechar', { duration: 5000 });
          this.discard();
          void this.router.navigate(['/invoices', invoice.id]);
        },
        error: (error: unknown) => this.errorMessage.set(executeErrorMessage(error))
      });
  }

  private interpretAudio(audio: Blob): void {
    this.interpreting.set(true);
    this.voiceAssistantService
      .interpretAudio(audio)
      .pipe(finalize(() => this.interpreting.set(false)))
      .subscribe({
        next: (result) => this.interpretation.set(result),
        error: (error: unknown) => this.errorMessage.set(interpretErrorMessage(error))
      });
  }

  private reset(): void {
    this.interpretation.set(null);
    this.errorMessage.set(null);
  }
}

class UnknownInvoiceError extends Error {}
class UnknownProductError extends Error {
  constructor(readonly code: string) {
    super(code);
  }
}
class InsufficientProductStockError extends Error {
  constructor(readonly code: string, readonly available: number) { super(code); }
}

function resolveItems(intent: VoiceIntent, products: Product[]): { productId: number; quantity: number }[] {
  return (intent.itens ?? []).map((item) => {
    const product = findSimilarProduct(item.code, products);
    if (product === undefined) {
      throw new UnknownProductError(item.code);
    }
    if (item.quantity > product.balance) {
      throw new InsufficientProductStockError(product.code, product.balance);
    }
    return { productId: product.id, quantity: item.quantity };
  });
}

function findSimilarProduct(reference: string, products: Product[]): Product | undefined {
  const normalizedReference = normalizeSearchText(reference);
  const referenceNumber = productNumber(reference);

  const exact = products.find(
    (product) =>
      normalizeSearchText(product.code) === normalizedReference ||
      (referenceNumber !== null && productNumber(product.code) === referenceNumber)
  );
  if (exact !== undefined) return exact;
  if (/^(?:PROD[-\s]?)?\d+$/i.test(reference.trim())) return undefined;

  const ranked = products
    .map((product) => ({
      product,
      score: Math.max(
        similarity(normalizedReference, normalizeSearchText(product.code)),
        similarity(normalizedReference, normalizeSearchText(product.description))
      )
    }))
    .sort((left, right) => right.score - left.score);

  if (ranked[0] === undefined || ranked[0].score < 0.55) return undefined;
  if (ranked[1] !== undefined && ranked[0].score - ranked[1].score < 0.08) return undefined;
  return ranked[0].product;
}

function productNumber(value: string): number | null {
  const match = value.toUpperCase().match(/(?:PROD[-\s]?)?0*(\d+)$/);
  return match === null ? null : Number(match[1]);
}

function normalizeSearchText(value: string): string {
  return value.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toUpperCase().replace(/[^A-Z0-9]/g, '');
}

function similarity(left: string, right: string): number {
  if (left === '' || right === '') return 0;
  if (left.includes(right) || right.includes(left)) return 0.9;

  const distances = Array.from({ length: right.length + 1 }, (_, index) => index);
  for (let leftIndex = 1; leftIndex <= left.length; leftIndex++) {
    let previous = distances[0];
    distances[0] = leftIndex;
    for (let rightIndex = 1; rightIndex <= right.length; rightIndex++) {
      const current = distances[rightIndex];
      distances[rightIndex] = Math.min(
        distances[rightIndex] + 1,
        distances[rightIndex - 1] + 1,
        previous + (left[leftIndex - 1] === right[rightIndex - 1] ? 0 : 1)
      );
      previous = current;
    }
  }
  return 1 - distances[right.length] / Math.max(left.length, right.length);
}

function formatCurrency(value: number): string {
  return new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' }).format(value);
}

function interpretErrorMessage(error: unknown): string {
  if (isConnectionError(error)) {
    return 'Não foi possível conectar ao assistente. Verifique se o Worker está no ar.';
  }

  switch (getApiError(error)?.code) {
    case 'EMPTY_TRANSCRIPT':
      return 'Não consegui entender o áudio. Grave novamente falando mais perto do microfone.';
    case 'AUDIO_TOO_LARGE':
      return 'O áudio ficou longo demais. Grave um comando curto.';
    default:
      return 'Não foi possível interpretar o comando. Tente novamente.';
  }
}

function executeErrorMessage(error: unknown): string {
  if (error instanceof UnknownProductError) {
    return `Não encontrei um produto parecido com “${error.code}”. Confira o número ou o nome.`;
  }
  if (error instanceof UnknownInvoiceError) {
    return 'Não encontrei uma nota com esse número.';
  }
  if (error instanceof InsufficientProductStockError) {
    return `${error.code} possui somente ${error.available} unidade${error.available === 1 ? '' : 's'} em estoque. A nota não foi criada.`;
  }
  if (isConnectionError(error)) {
    return 'Não foi possível conectar ao serviço. Tente novamente.';
  }

  switch (getApiError(error)?.code) {
    case 'PRODUCT_CODE_ALREADY_EXISTS':
      return 'Já existe um produto com esse código.';
    case 'PRODUCT_NOT_FOUND':
      return 'Um dos produtos informados não está mais disponível.';
    case 'INSUFFICIENT_STOCK':
      return 'Estoque insuficiente para fechar a nota fiscal.';
    case 'INVOICE_ALREADY_CLOSED':
      return 'Esta nota já foi fechada.';
    case 'INVOICE_CLOSE_ALREADY_IN_PROGRESS':
      return 'O fechamento desta nota já está em andamento. Aguarde e consulte a nota.';
    case 'INVENTORY_SERVICE_UNAVAILABLE':
      return 'Serviço de estoque indisponível. Tente novamente.';
    default:
      return 'Não foi possível concluir a ação. Tente novamente.';
  }
}
