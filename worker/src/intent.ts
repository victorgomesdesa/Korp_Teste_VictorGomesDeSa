// Intenção que o assistente devolve para a interface. A execução continua sendo do frontend,
// que confirma com o usuário e chama as APIs existentes de Inventory e Billing.
export type VoiceAction = 'criar_produto' | 'criar_nota' | 'fechar_nota' | 'desconhecido';

export interface VoiceIntent {
  acao: VoiceAction;
  code?: string;
  description?: string;
  balance?: number;
  price?: number;
  itens?: { code: string; quantity: number }[];
  numeroNota?: number;
}

// JSON Mode do Workers AI: o modelo é obrigado a responder neste formato.
export const intentSchema = {
  type: 'object',
  properties: {
    acao: {
      type: 'string',
      description: 'Ação solicitada pelo usuário.',
      enum: ['criar_produto', 'criar_nota', 'fechar_nota', 'desconhecido']
    },
    code: { type: 'string', description: 'Número ou código do produto, por exemplo 2 ou PROD-02.' },
    description: { type: 'string', description: 'Nome do produto.' },
    balance: {
      type: 'number',
      description: 'Estoque inicial do produto. Use somente em criar_produto.'
    },
    price: {
      type: 'number',
      description: 'Valor unitário do produto em reais. Use somente em criar_produto.'
    },
    itens: {
      type: 'array',
      description: 'Itens da nota fiscal. Use somente em criar_nota.',
      items: {
        type: 'object',
        properties: {
          code: { type: 'string', description: 'Código, número ou nome do produto mencionado.' },
          quantity: { type: 'number', description: 'Quantidade do item.' }
        },
        required: ['code', 'quantity']
      }
    },
    numeroNota: {
      type: 'number',
      description: 'Número da nota fiscal a fechar. Use somente em fechar_nota.'
    }
  },
  required: ['acao']
} as const;

export const systemPrompt = [
  'Você interpreta comandos de voz em português de um sistema de notas fiscais.',
  'O idioma padrão é português do Brasil (pt-BR). Interprete pelo significado, não por frases exatas.',
  'Considere pequenas falhas de transcrição e palavras foneticamente parecidas antes de usar desconhecido.',
  'Palavras podem chegar incompletas ou cortadas. Reconstrua o significado usando a ação, a posição do termo, os campos esperados e o restante da frase.',
  'Como a interface sempre pedirá confirmação, prefira a interpretação semanticamente mais provável quando houver contexto suficiente; não exija correspondência literal.',
  'Responda apenas com o JSON do schema, sem texto extra.',
  '',
  'Ações possíveis:',
  '- criar_produto: preencha code, description, balance e price. O usuário pode dizer apenas o número do código; normalize sempre para PROD- seguido do número.',
  '- criar_nota: preencha itens, cada um com code (código, número ou nome falado) e quantity (inteiro >= 1).',
  '- fechar_nota: preencha numeroNota com o número da nota. Nunca use balance nesta ação.',
  '- desconhecido: use quando o comando não corresponder a nenhuma ação acima.',
  '',
  'Regras:',
  '- Para códigos, "produto 2", "código 2" e "PROD-2" significam PROD-2. Normalize para maiúsculas.',
  '- Nunca invente números que o usuário não falou. Se um campo obrigatório realmente faltar, omita-o.',
  '- Em criar_produto, use também a estrutura da frase: depois do código e do nome, um número intermediário antes do valor/preço representa balance quando seu rótulo estiver incompleto ou irreconhecível.',
  '- Se o usuário disser estoque ou saldo, sempre devolva esse número em balance.',
  '- Se o usuário disser valor ou preço, sempre devolva esse número em price.',
  '- Converta números por extenso em dígitos: "cinco" = 5, "doze" = 12, "vinte" = 20.',
  '- Use exatamente o número que o usuário falou. Nunca reaproveite números dos exemplos.'
].join('\n');

// Exemplos curtos ajudam o modelo a manter o formato e as regras do domínio.
export const fewShotMessages = [
  {
    role: 'user' as const,
    content: 'cadastrar produto 10 nome teclado mecânico com estoque dez e valor 199,90'
  },
  {
    role: 'assistant' as const,
    content: '{"acao":"criar_produto","code":"PROD-10","description":"Teclado mecânico","balance":10,"price":199.9}'
  },
  {
    role: 'user' as const,
    content: 'criar uma nota com duas unidades do PROD-001 e uma do PROD-002'
  },
  {
    role: 'assistant' as const,
    content:
      '{"acao":"criar_nota","itens":[{"code":"PROD-001","quantity":2},{"code":"PROD-002","quantity":1}]}'
  },
  {
    role: 'user' as const,
    content: 'fechar a nota número quinze'
  },
  {
    role: 'assistant' as const,
    content: '{"acao":"fechar_nota","numeroNota":15}'
  },
  {
    role: 'user' as const,
    content: 'novo produto 7 cabo HDMI estoque três valor 25 reais'
  },
  {
    role: 'assistant' as const,
    content: '{"acao":"criar_produto","code":"PROD-7","description":"Cabo HDMI","balance":3,"price":25}'
  }
];

// O modelo pode devolver campos fora do formato esperado; normalizamos antes de entregar à interface.
export function normalizeIntent(value: unknown): VoiceIntent {
  if (typeof value !== 'object' || value === null) {
    return { acao: 'desconhecido' };
  }

  const candidate = value as Record<string, unknown>;
  const acao = candidate['acao'];
  if (
    acao !== 'criar_produto' &&
    acao !== 'criar_nota' &&
    acao !== 'fechar_nota'
  ) {
    return { acao: 'desconhecido' };
  }

  const intent: VoiceIntent = { acao };

  if (typeof candidate['code'] === 'string' && candidate['code'].trim() !== '') {
    intent.code = normalizeProductCode(candidate['code']);
  }
  if (typeof candidate['description'] === 'string' && candidate['description'].trim() !== '') {
    intent.description = candidate['description'].trim();
  }
  if (Number.isFinite(candidate['balance'])) {
    intent.balance = Math.trunc(candidate['balance'] as number);
  }
  if (Number.isFinite(candidate['price'])) {
    intent.price = Math.round((candidate['price'] as number) * 100) / 100;
  }
  if (Number.isFinite(candidate['numeroNota'])) {
    intent.numeroNota = Math.trunc(candidate['numeroNota'] as number);
  }

  // Modelos podem trocar estoque e número da nota entre si. Dada a ação, o significado do número
  // é inequívoco, então corrigimos aqui em vez de depender do comportamento do modelo.
  if (intent.acao === 'criar_produto' && intent.balance === undefined && intent.numeroNota !== undefined) {
    intent.balance = intent.numeroNota;
    delete intent.numeroNota;
  }
  if (intent.acao === 'fechar_nota' && intent.numeroNota === undefined && intent.balance !== undefined) {
    intent.numeroNota = intent.balance;
    delete intent.balance;
  }
  if (intent.acao !== 'criar_produto') {
    delete intent.balance;
    delete intent.price;
  }
  if (intent.acao !== 'fechar_nota') {
    delete intent.numeroNota;
  }

  if (Array.isArray(candidate['itens'])) {
    const itens = (candidate['itens'] as unknown[])
      .map((item) => {
        if (typeof item !== 'object' || item === null) {
          return null;
        }
        const entry = item as Record<string, unknown>;
        const code = typeof entry['code'] === 'string' ? normalizeProductCode(entry['code']) : '';
        const quantity = Number.isFinite(entry['quantity'])
          ? Math.trunc(entry['quantity'] as number)
          : 0;
        return code !== '' && quantity > 0 ? { code, quantity } : null;
      })
      .filter((item): item is { code: string; quantity: number } => item !== null);

    if (itens.length > 0) {
      intent.itens = itens;
    }
  }

  return intent;
}

function normalizeProductCode(value: string): string {
  const normalized = value.trim().toUpperCase();
  const number = normalized.match(/^(?:PROD[-\s]?)?(\d+)$/)?.[1];
  return number === undefined ? normalized : `PROD-${number}`;
}
