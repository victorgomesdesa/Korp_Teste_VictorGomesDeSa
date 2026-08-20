import { describe, expect, it } from 'vitest';

import worker from './index';
import { normalizeIntent } from './intent';

const env = {
  AI: {} as Ai,
  ALLOWED_ORIGIN: 'http://localhost:4200'
} satisfies Env;

describe('voice worker', () => {
  it('rejeita JSON malformado como erro de cliente', async () => {
    const response = await worker.fetch(
      new Request('http://localhost/api/voice/intent', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{'
      }),
      env
    );

    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toEqual({ code: 'INVALID_REQUEST', message: 'JSON inválido.' });
  });

  it('limita comandos de texto antes de chamar a IA', async () => {
    const response = await worker.fetch(
      new Request('http://localhost/api/voice/intent', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: 'a'.repeat(2_001) })
      }),
      env
    );

    expect(response.status).toBe(413);
    await expect(response.json()).resolves.toEqual({
      code: 'TEXT_TOO_LARGE',
      message: 'O comando é muito longo.'
    });
  });
});

describe('normalizeIntent', () => {
  it('normaliza um cadastro de produto', () => {
    expect(
      normalizeIntent({
        acao: 'criar_produto',
        code: ' prod-020 ',
        description: '  Monitor  ',
        balance: 5
      })
    ).toEqual({
      acao: 'criar_produto',
      code: 'PROD-020',
      description: 'Monitor',
      balance: 5
    });
  });

  it('adiciona o prefixo PROD- quando o usuário informa somente o número', () => {
    expect(normalizeIntent({ acao: 'criar_nota', itens: [{ code: '02', quantity: 1 }] })).toEqual({
      acao: 'criar_nota',
      itens: [{ code: 'PROD-02', quantity: 1 }]
    });
  });

  it('recupera o saldo quando o modelo o coloca no campo do número da nota', () => {
    expect(normalizeIntent({ acao: 'criar_produto', code: 'PROD-020', numeroNota: 5 })).toEqual({
      acao: 'criar_produto',
      code: 'PROD-020',
      balance: 5
    });
  });

  it('recupera o número da nota quando o modelo o coloca no campo de saldo', () => {
    expect(normalizeIntent({ acao: 'fechar_nota', balance: 7 })).toEqual({
      acao: 'fechar_nota',
      numeroNota: 7
    });
  });

  it('descarta campos que não pertencem à ação', () => {
    expect(
      normalizeIntent({ acao: 'criar_nota', balance: 9, numeroNota: 3, itens: [{ code: 'PROD-001', quantity: 2 }] })
    ).toEqual({
      acao: 'criar_nota',
      itens: [{ code: 'PROD-001', quantity: 2 }]
    });
  });

  it('mantém apenas itens com código e quantidade positiva', () => {
    const intent = normalizeIntent({
      acao: 'criar_nota',
      itens: [
        { code: 'prod-001', quantity: 2 },
        { code: '', quantity: 3 },
        { code: 'PROD-002', quantity: 0 },
        { code: 'PROD-003', quantity: 1.9 },
        'lixo'
      ]
    });

    expect(intent.itens).toEqual([
      { code: 'PROD-001', quantity: 2 },
      { code: 'PROD-003', quantity: 1 }
    ]);
  });

  it('trata ação desconhecida e respostas inválidas', () => {
    expect(normalizeIntent({ acao: 'remover_produto' })).toEqual({ acao: 'desconhecido' });
    expect(normalizeIntent({ acao: 'desconhecido' })).toEqual({ acao: 'desconhecido' });
    expect(normalizeIntent(null)).toEqual({ acao: 'desconhecido' });
    expect(normalizeIntent('texto solto')).toEqual({ acao: 'desconhecido' });
  });

  it('ignora campos ausentes em vez de inventar valores', () => {
    expect(normalizeIntent({ acao: 'criar_produto', code: 'PROD-001' })).toEqual({
      acao: 'criar_produto',
      code: 'PROD-001'
    });
  });
});
