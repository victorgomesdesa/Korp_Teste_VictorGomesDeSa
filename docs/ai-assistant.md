# Assistente IA

O assistente permite cadastrar produtos, criar notas e fechar notas por texto ou voz em português do
Brasil. Ele interpreta a solicitação, mas não possui acesso direto aos bancos e não executa ações sem
confirmação do usuário.

## Fluxo e responsabilidades

```text
Usuário ─► Angular ─► Worker local ─► Workers AI remoto
   ▲           │            │
   │           │            └─ devolve transcrição + intenção estruturada
   │           ├─ mostra o resumo e exige Confirmar/Descartar
   │           └─ após Confirmar, chama Inventory ou Billing
   └────────────── recebe o resultado da API de domínio
```

- O **Worker** transcreve, interpreta e normaliza a intenção.
- O **frontend** apresenta a interpretação, resolve referências de produto e exige confirmação.
- **Inventory e Billing** continuam sendo as autoridades sobre validação, persistência, estoque,
  idempotência e erros de domínio.

Ao criar uma nota, o usuário pode mencionar código ou nome. Códigos numéricos são normalizados para
`PROD-<número>`; nomes são comparados na interface sem acentos e com tolerância a pequenas diferenças.
Um resultado só é aceito quando a melhor correspondência ultrapassa o limiar e não é ambígua.

## Modelos

| Etapa | Modelo | Quando é usado |
|---|---|---|
| Transcrição | `@cf/openai/whisper-large-v3-turbo` | Apenas em comandos de áudio; idioma fixado como português |
| Intenção | `@cf/meta/llama-3.3-70b-instruct-fp8-fast` | Em todo comando, com temperatura zero e saída por JSON Schema |
| Revisão | `@cf/deepseek-ai/deepseek-r1-distill-qwen-32b` | Somente se uma ação conhecida vier sem campos obrigatórios |

A revisão recebe a transcrição original e o JSON parcial. O prompt permite reconstruir palavras
cortadas pelo contexto, mas proíbe inventar números que não apareçam no comando. Depois dos modelos,
uma normalização determinística valida a ação, os tipos, as quantidades e os campos permitidos.

## Contrato HTTP

### Saúde

```http
GET /health
```

```json
{ "status": "ok" }
```

### Interpretar texto

```http
POST /api/voice/intent
Content-Type: application/json

{ "text": "Cadastrar produto 20, nome Monitor, estoque 5, valor 900 reais" }
```

Resposta:

```json
{
  "transcript": "Cadastrar produto 20, nome Monitor, estoque 5, valor 900 reais",
  "intent": {
    "acao": "criar_produto",
    "code": "PROD-20",
    "description": "Monitor",
    "balance": 5,
    "price": 900
  }
}
```

### Interpretar áudio

O frontend envia os bytes gravados diretamente:

```http
POST /api/voice/intent
Content-Type: application/octet-stream

<bytes do áudio>
```

A resposta usa o mesmo formato do texto, preenchendo `transcript` com a saída do Whisper.

### Intenções

| `acao` | Campos esperados |
|---|---|
| `criar_produto` | `code`, `description`, `balance`, `price` |
| `criar_nota` | `itens[]` com `code` e `quantity` |
| `fechar_nota` | `numeroNota` |
| `desconhecido` | Nenhum campo adicional obrigatório |

`price` é expresso em reais na conversa; somente ao confirmar o Angular converte para
`priceInCents`. `numeroNota` é o número público da nota, não seu id interno.

## Limites e erros

| Condição | HTTP | Código |
|---|---:|---|
| JSON inválido ou texto/áudio ausente | 400 | `INVALID_REQUEST` |
| Transcrição vazia | 422 | `EMPTY_TRANSCRIPT` |
| Texto acima de 2.000 caracteres | 413 | `TEXT_TOO_LARGE` |
| Áudio acima de 4 MiB | 413 | `AUDIO_TOO_LARGE` |
| Rota inexistente | 404 | `NOT_FOUND` |
| Falha inesperada ou de inferência | 500 | `INTERNAL_ERROR` |

CORS libera somente `ALLOWED_ORIGIN`, atualmente `http://localhost:4200`. A aplicação não possui
autenticação por decisão de escopo; antes de qualquer publicação, autenticação, rate limiting e uma
origem HTTPS explícita devem ser adicionados.

## Desenvolvimento local

```bash
npm --prefix worker ci
npm --prefix worker exec wrangler login
make worker
```

O endereço HTTP é local (`http://localhost:8787`), mas `wrangler.jsonc` configura o binding `AI` com
`remote: true`. Assim, transcrição e interpretação acontecem na infraestrutura da Cloudflare e
exigem internet e autenticação válidas. Não há modelo baixado ou executado na máquina local.

Para apontar outro frontend ao Worker, altere `ALLOWED_ORIGIN`; para publicar, ajuste também
`voiceApiUrl` no environment do Angular.

## Validação

```bash
make test-worker
```

O comando executa testes unitários, verificação TypeScript, conferência dos tipos gerados pelo
Wrangler e um dry-run do deploy. O CI não chama modelos remotos: valida normalização, limites e
contratos locais de forma determinística. A qualidade semântica dos modelos deve ser acompanhada com
um conjunto separado de comandos reais, pois respostas de inferência podem variar com o serviço e
com futuras versões dos modelos.
