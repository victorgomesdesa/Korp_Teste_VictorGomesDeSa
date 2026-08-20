# Design Técnico: Sistema de Emissão de Notas Fiscais

Este documento descreve **como** o sistema está construído e **por que** cada decisão foi tomada.

Os critérios funcionais e cenários de teste detalhados estão documentados em
[`user-stories.md`](./user-stories.md).

---

## 1. Visão Geral

Uma aplicação web para um usuário genérico cadastrar produtos, consultar estoque, criar e consultar
notas fiscais simplificadas e solicitar a ação visual **Imprimir Nota**. No domínio, essa ação fecha a
nota: consome todo o estoque necessário e altera seu status de `OPEN` para `CLOSED`.

O sistema resolve três problemas centrais:

1. **Consistência distribuída**, coordenar Billing e Inventory sem fingir que existe uma transação
   ACID única entre bancos pertencentes a serviços diferentes.
2. **Concorrência de estoque**, permitir que exatamente uma operação consuma o último item, sem saldo
   negativo nem baixa parcial de uma nota com vários produtos.
3. **Idempotência e recuperação**, repetir com segurança uma operação cujo resultado ficou incerto
   após a baixa de estoque.

| Camada | Tecnologia |
|---|---|
| Frontend | Angular + TypeScript, Reactive Forms, HttpClient, RxJS e Angular Material |
| Backend | Go + Gin, Go Modules, `pgx` e `golang-migrate` |
| Banco de dados | PostgreSQL |
| Serviços | `inventory-service` + `billing-service` |
| Assistente | Cloudflare Worker + Workers AI + TypeScript |
| Ambiente e CI | Docker + Docker Compose; GitHub Actions |

**Origem das regras.** Cadastro de produtos, criação de notas com múltiplos itens, ação de
impressão/fechamento com baixa de estoque e tratamento da indisponibilidade entre microsserviços vêm
diretamente do desafio da Korp. Consulta e listagem de produtos e notas são **necessidades derivadas**
para viabilizar esses fluxos na interface. Concorrência e idempotência são diferenciais
deliberadamente adotados. As interpretações abertas são premissas `A-*`; escolhas de implementação
são decisões técnicas.

---

## 2. Arquitetura

Dois microsserviços reais, com ownership exclusivo de dados:

```text
Angular
  ├── HTTP/JSON ──► Cloudflare Worker ─► Workers AI (inferência remota)
  ├── HTTP/JSON ──► Inventory API ─► Inventory Service ─► Inventory Repository ─► inventory_db
  └── HTTP/JSON ──► Billing API   ─► Billing Service   ─► Billing Repository   ─► billing_db
                                      │
                                      └── HTTP/JSON ──► Inventory API
```

O Angular consulta produtos diretamente no Inventory e notas diretamente no Billing. Na criação, o
Billing consulta o Inventory para validar `productId` e obter os snapshots. No fechamento, o Billing
chama o Inventory para consumir estoque. Uma única instância PostgreSQL pode hospedar os dois
bancos ou schemas no Docker Compose, mas um serviço nunca lê tabelas do outro e não existem foreign
keys atravessando a fronteira.

Em cada serviço:

```text
HTTP Handler
     ↓
Application / Service
     ↓
Repository
     ↓
PostgreSQL
```

- **Handlers não decidem regras de negócio.** Desserializam, validam forma básica e traduzem erros de
  domínio em HTTP.
- **Services são autoridades do domínio.** Billing decide o ciclo de vida da `Invoice`; Inventory
  decide disponibilidade e mutação de estoque.
- **Repositories não decidem regras de domínio.** Executam consultas e delimitam transações locais.
- **Nenhuma infraestrutura adicional é presumida.** Não há API Gateway, broker, Kubernetes ou service
  mesh neste design.

---

## 3. Decisões Técnicas

### 3.1 Separação de serviços e ownership

`inventory-service` possui `Product` e `StockOperation`; `billing-service` possui `Invoice`,
`InvoiceItem` e `InvoiceCloseOperation`. `InvoiceItem.productId` e
`StockOperation.invoiceId` são referências lógicas externas, nunca FKs entre serviços.
`InvoiceCloseOperation` e `StockOperation` são infraestrutura de consistência/orquestração, não
entidades visíveis ao usuário. Essa separação impede acoplamento por banco e torna explícitas as
chamadas que podem falhar.

### 3.2 Criação não é fechamento

O cliente envia somente `productId` e `quantity`. Em `POST /api/invoices`, o Billing consulta o
Inventory, valida todos os ids, confirma que a quantidade atual é suficiente e recebe `code`,
`description`, `balance` e `priceInCents`. Só então grava `Invoice` e `InvoiceItem` na mesma transação
local, usando os dados retornados para os snapshots. Angular nunca é autoridade sobre
`productCode`, `productDescription` ou `unitPriceInCents`.

```text
Angular → Billing → Inventory: consultar/validar productIds e disponibilidade atual
Billing ← Inventory: id + code + description + balance + priceInCents
Billing → billing_db: criar Invoice OPEN + InvoiceItems
```

Se Inventory estiver indisponível, Billing retorna `503 INVENTORY_SERVICE_UNAVAILABLE` e não persiste
`Invoice` nem `InvoiceItem`. Se algum id não existir, retorna `404 PRODUCT_NOT_FOUND`; se a quantidade
já superar o estoque consultado, retorna `409 INSUFFICIENT_STOCK`. Não há nota incompleta,
compensação ou fila; uma tentativa manual posterior é suficiente. A criação não reserva nem reduz
estoque, portanto a disponibilidade é validada novamente no fechamento. O consumo ocorre somente em
`POST /api/invoices/{id}/close` (A-4, A-13).

### 3.3 Numeração sequencial

`Invoice.number` é único, sequencial e gerado pelo PostgreSQL do Billing, nunca aceito no payload.
Sequências podem possuir lacunas após rollback; “sequencial” significa ordem monotônica e unicidade,
não numeração contígua sem lacunas. Essa interpretação adicional é A-9.

### 3.4 Atomicidade dentro do Inventory

Todos os itens de uma nota são consumidos na mesma transação do Inventory. Para cada quantidade
agregada por produto:

```sql
UPDATE products
   SET balance = balance - $1,
       updated_at = NOW()
 WHERE id = $2
   AND balance >= $1;
```

`RowsAffected = 0` significa produto inexistente ou saldo insuficiente; a causa é confirmada sem
confirmar a transação. Qualquer falha provoca `ROLLBACK`. Não existe consumo parcial.

### 3.5 Consistência entre microsserviços

Não existe transação distribuída ACID. Billing persiste uma `InvoiceCloseOperation`, solicita o
consumo idempotente no Inventory e, após sucesso, muda a nota para `CLOSED` e a operação para
`COMPLETED` em uma transação local do Billing. Se a resposta do Inventory se perder, a nota pode
permanecer `OPEN` e a operação `PROCESSING`, embora o estoque já tenha sido consumido. A recuperação
repete a mesma operação lógica com a mesma chave; Inventory devolve a `StockOperation` gravada sem
nova baixa e Billing conclui a transição.

### 3.6 Concorrência

A leitura de saldo serve para exibição, não para garantir integridade. A atualização condicional e o
lock adquirido pelo PostgreSQL ficam dentro da transação do Inventory. Para saldo `1` e duas notas
concorrentes pedindo `1`, exatamente uma atualização afeta uma linha; a outra falha com
`INSUFFICIENT_STOCK`. Nunca há `balance = -1`.

Existe uma corrida diferente no Billing: a mesma `Invoice OPEN` pode receber simultaneamente duas
chaves distintas. A criação de `InvoiceCloseOperation` ocorre em transação e possui
`UNIQUE(invoiceId)`. Exatamente uma requisição assume o fechamento; a outra recebe conflito
controlado antes de chamar Inventory. Botão desabilitado, leitura prévia de status e check em memória
não participam dessa garantia (A-12).

### 3.7 Idempotência

`POST /api/invoices/{id}/close` exige `Idempotency-Key`. Billing associa persistentemente a chave à
`Invoice` em `InvoiceCloseOperation`, cujo status mínimo é `PROCESSING | COMPLETED`, e propaga ao
`POST /api/stock/consume` a chave, o `invoiceId` e os itens. `UNIQUE(invoiceId)` garante uma única
operação lógica de fechamento; a chave também é única no Billing.

Inventory agrega e ordena os itens por `productId`, serializa essa representação canônica e calcula
uma fingerprint equivalente a `hash(invoiceId + canonicalizedItems)`. A `StockOperation`, gravada na
mesma transação da baixa, contém `invoiceId`, chave, fingerprint e resultado.

- Mesma chave + mesmo `invoiceId` + mesmos itens: retorna o resultado anterior, sem nova baixa.
- Mesma chave + `invoiceId` diferente: `409 IDEMPOTENCY_KEY_REUSED`.
- Mesma chave + itens diferentes: `409 IDEMPOTENCY_KEY_REUSED`.
- Nota já `CLOSED` + chave da execução concluída: resposta compatível com o sucesso original.
- Nota já `CLOSED` + outra chave: `409 INVOICE_ALREADY_CLOSED`.
- Nota `OPEN` + outra operação já associada: `409 INVOICE_CLOSE_ALREADY_IN_PROGRESS`.

Não se trata apenas de rejeitar uma nota fechada: a chave resolve a incerteza de uma tentativa cuja
resposta não chegou ao chamador.

### 3.8 Indisponibilidade e timeout

`context.Context` propaga cancelamento e um timeout explícito de **3 segundos** nas chamadas
Billing → Inventory (A-10). Na criação, falha de conexão ou timeout resulta em `503` e nenhuma nota é
persistida. No fechamento, resulta em `503 INVENTORY_SERVICE_UNAVAILABLE`; a nota permanece `OPEN` e
a `InvoiceCloseOperation` preserva a chave para retry. Não há retry automático cego.

### 3.9 Angular, RxJS e interface

- **Reactive Forms** modela a nota e o `FormArray` de itens, com quantidade positiva e ao menos um
  item antes do envio. O backend repete todas as validações relevantes.
- **HttpClient** encapsula os contratos REST. Um interceptor propaga `X-Request-Id`; outro ponto
  comum traduz o envelope de erro para mensagens controladas, sem mover regra de negócio ao cliente.
- **RxJS** usa `switchMap` na orquestração do assistente e `finalize` para encerrar loading em sucesso
  ou falha. Os callbacks de erro convertem o envelope HTTP em estado de tela controlado.
- **Angular Material** fornece tabela, formulário, feedback e estados desabilitados consistentes.
- **Ciclo de vida.** `ngOnInit` carrega o detalhe da nota quando o input de rota já está disponível;
  `afterNextRender` agenda `window.print()` somente depois que a resposta de fechamento atualizou a
  tela. As chamadas HTTP são finitas e tratadas por `subscribe`, com `finalize` encerrando o estado
  de carregamento.

Enquanto o fechamento estiver ativo, a interface exibe processamento e desabilita **Imprimir Nota**.
No sucesso, o mesmo fluxo atualiza a nota confirmada como `CLOSED` e inicia a impressão visual; não
exige um segundo clique. Isso previne cliques acidentais, mas nunca substitui idempotência ou
validação no backend.

### 3.10 Go e persistência

Go Modules gerencia dependências; Gin concentra a camada HTTP; `pgx` acessa PostgreSQL;
`golang-migrate` versiona schemas. Erros são valores explícitos: services retornam erros de domínio e
handlers os mapeiam para o contrato HTTP. `context.Context` atravessa handler, service, cliente HTTP e
repository. Logs usam `log/slog` estruturado.

### 3.11 Observabilidade

Cada request recebe ou preserva `X-Request-Id`; Billing propaga o mesmo valor ao Inventory. Logs
relevantes contêm `request_id`, `invoice_id`, `product_id` quando aplicável, `operation`, `result`,
`duration_ms` e `error_type`, sem payloads completos ou dados desnecessários.

Exemplo de investigação de indisponibilidade:

```json
{"level":"ERROR","request_id":"req-7f2","invoice_id":15,"operation":"consume_stock","result":"error","duration_ms":3001,"error_type":"inventory_timeout"}
```

Com `request_id=req-7f2`, o operador correlaciona a tentativa no Billing e a ausência ou atraso no
Inventory. Métricas e traces podem ser adicionados depois sem acoplar o domínio.

### 3.12 Assistente IA

O Cloudflare Worker expõe `POST /api/voice/intent` para texto JSON ou bytes de áudio. Whisper Large
v3 Turbo transcreve áudio com idioma `pt`; Llama 3.3 70B extrai uma intenção conforme JSON Schema. Se
uma intenção conhecida vier sem campos obrigatórios, DeepSeek R1 Distill Qwen 32B faz uma segunda
revisão da transcrição original e da extração parcial. A normalização final é determinística: aceita
o número isolado de produto, aplica o prefixo `PROD-`, remove campos incompatíveis com a ação e
descarta itens inválidos.

O Worker não chama Inventory nem Billing. O Angular apresenta a interpretação em uma conversa,
resolve códigos ou nomes contra os produtos disponíveis e exige confirmação antes de executar a ação
nas APIs de domínio. Essa fronteira evita que uma inferência probabilística cause escrita sem
consentimento. Texto é limitado a 2.000 caracteres, áudio a 4 MiB e CORS aceita apenas a origem
configurada. Em desenvolvimento, o processo Wrangler é local, mas o binding `AI` é remoto.

O contrato operacional completo está em [`ai-assistant.md`](./ai-assistant.md).

---

## 4. Modelo de Dados

```text
Billing                                         Inventory
Invoice 1 ──◆ 1..* InvoiceItem                 Product
    │               productId ···············► Product.id
    └──── 0..1 InvoiceCloseOperation            StockOperation
                         StockOperation.invoiceId ···► Invoice.id
                         referências lógicas; sem FK entre serviços
```

| Domínio | Entidade | Atributos | Restrições |
|---|---|---|---|
| Inventory | `Product` | `id BIGINT`, `code`, `description`, `balance`, `priceInCents`, `createdAt`, `updatedAt` | `code` único; `balance >= 0`; `priceInCents >= 0` |
| Inventory | `StockOperation` | `id BIGINT`, `invoiceId BIGINT`, `idempotencyKey`, `fingerprint`, `result`, `createdAt` | chave única; `invoiceId` é referência lógica sem FK; infraestrutura de consistência |
| Billing | `Invoice` | `id BIGINT`, `number`, `status`, `createdAt`, `closedAt` | `number` único/sequencial; `status ∈ {OPEN,CLOSED}`; fechada é imutável |
| Billing | `InvoiceItem` | `id BIGINT`, `invoiceId BIGINT`, `productId BIGINT`, `productCode`, `productDescription`, `unitPriceInCents`, `quantity` | `quantity > 0`; `unitPriceInCents >= 0`; FK local somente para `Invoice` |
| Billing | `InvoiceCloseOperation` | `id BIGINT`, `invoiceId BIGINT`, `idempotencyKey`, `status`, `result`, `createdAt`, `completedAt` | FK local para `Invoice`; `UNIQUE(invoiceId)`; chave única; `status ∈ {PROCESSING,COMPLETED}` |

IDs de entidade são `BIGINT` no PostgreSQL e `int64` em Go. A `Idempotency-Key` continua sendo uma
string, normalmente um UUID da operação, e não se confunde com o id numérico de nenhuma entidade.

`productCode`, `productDescription` e `unitPriceInCents` são snapshots obtidos na criação. Assim, uma
alteração futura no cadastro não reescreve a representação histórica nem o total da nota. Itens
repetidos do mesmo produto são rejeitados como payload inválido (A-11); isso mantém o contrato
inequívoco, embora Inventory ainda agregue defensivamente as quantidades antes do consumo.

---

## 5. Fluxo de Fechamento da Nota

| # | Etapa | Falha |
|---|---|---|
| 1 | Angular envia `POST /api/invoices/{id}/close` + `Idempotency-Key` | `400` se chave ausente/malformada |
| 2 | Billing busca a `Invoice` e sua `InvoiceCloseOperation` | `404`; ou resultado anterior/`409` conforme a chave |
| 3 | Billing cria a operação `PROCESSING` em transação, protegida por `UNIQUE(invoiceId)` | `409` se outra operação assumiu a nota |
| 4 | Billing chama Inventory com `invoiceId`, itens, chave e `X-Request-Id` | `503`; nota continua `OPEN` |
| 5 | Inventory valida chave + `invoiceId` + fingerprint canônica | resultado anterior, ou `409 IDEMPOTENCY_KEY_REUSED` |
| 6 | Inventory consome todos os itens e grava `StockOperation` na mesma transação local | `409`; `ROLLBACK`; nota continua `OPEN` |
| 7 | Billing atualiza `Invoice OPEN → CLOSED` e `InvoiceCloseOperation → COMPLETED` localmente | retry seguro se falhar após a baixa |
| 8 | Angular recebe sucesso, atualiza a nota e inicia a impressão visual no mesmo fluxo | `200 OK` |

O caso perigoso é uma baixa confirmada no Inventory seguida de perda da resposta ou falha no Billing.
Na nova tentativa, `InvoiceCloseOperation` identifica a chave legítima e a mesma chave recupera o
resultado da `StockOperation`; não há segunda baixa. Billing então conclui `OPEN → CLOSED` e marca a
operação `COMPLETED`. A operação é recuperável, não atomicamente distribuída.

---

## 6. API

| Método | Rota | Serviço | Sucesso | Objetivo |
|---|---|---|---|---|
| `POST` | `/api/products` | Inventory | `201` | Cadastrar produto |
| `GET` | `/api/products` | Inventory | `200` | Listar produtos e saldos |
| `GET` | `/api/products/{id}` | Inventory | `200` | Consultar produto |
| `POST` | `/api/stock/consume` | Inventory | `200` | Consumir itens atomicamente; uso interno pelo Billing |
| `POST` | `/api/invoices` | Billing | `201` | Criar nota `OPEN` |
| `GET` | `/api/invoices` | Billing | `200` | Listar notas |
| `GET` | `/api/invoices/{id}` | Billing | `200` | Consultar detalhes |
| `POST` | `/api/invoices/{id}/close` | Billing | `200` | Fechar/processar nota |

Exemplo de criação:

```json
{"items":[{"productId":1,"quantity":2},{"productId":2,"quantity":1}]}
```

`number`, `status`, snapshots, totais e datas não são aceitos do cliente. A listagem devolve até os
códigos dos produtos e `totalInCents`; o detalhe devolve o preço unitário de cada item, permitindo o
mesmo total sem consultar novamente o Inventory.

Para formar os snapshots, Billing consulta Inventory com os `productId`. Produto inexistente resulta
em `404 PRODUCT_NOT_FOUND`, quantidade acima do estoque atual em `409 INSUFFICIENT_STOCK` e
indisponibilidade em `503`; em todos esses casos nenhuma linha é persistida no banco Billing.

Contrato conceitual interno de consumo:

```json
{
  "invoiceId": 1001,
  "items": [
    { "productId": 1, "quantity": 2 }
  ]
}
```

`invoiceId` participa da fingerprint junto aos itens agregados e ordenados. Ele é referência lógica
no Inventory, sem FK para Billing.

Exemplo de fechamento:

```http
POST /api/invoices/15/close
Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000
X-Request-Id: req-7f2
```

Envelope de erro:

```json
{"code":"INSUFFICIENT_STOCK","message":"Estoque insuficiente para o produto PROD-001."}
```

| HTTP | Códigos de domínio aplicáveis |
|---|---|
| `200 OK` | leitura, consumo interno ou fechamento concluído/repetido com a mesma chave |
| `201 Created` | produto ou nota criada |
| `400 Bad Request` | JSON inválido, parâmetro ou `Idempotency-Key` ausente/malformado |
| `404 Not Found` | `PRODUCT_NOT_FOUND`, `INVOICE_NOT_FOUND` |
| `409 Conflict` | `PRODUCT_CODE_ALREADY_EXISTS`, `INVOICE_ALREADY_CLOSED`, `INVOICE_CLOSE_ALREADY_IN_PROGRESS`, `INSUFFICIENT_STOCK`, `IDEMPOTENCY_KEY_REUSED` |
| `422 Unprocessable Entity` | `INVALID_QUANTITY`, lista de itens vazia ou produto duplicado |
| `503 Service Unavailable` | `INVENTORY_SERVICE_UNAVAILABLE` |

---

## 7. Diagramas UML

Os diagramas são largos demais para garantir texto legível quando incorporados na largura padrão do
Markdown. Prefira abri-los em tamanho completo.

### Diagrama de Casos de Uso

[Visualizar em tamanho completo](./diagrams/01-use-case-diagram.svg)

Ator `Usuário`, fronteira do sistema, operações de produto e nota e os comportamentos incluídos no
fechamento. Notas laterais distinguem regras do desafio dos diferenciais de concorrência e
idempotência.

### Diagrama de Classes

[Visualizar em tamanho completo](./diagrams/02-class-diagram.svg)

Domínios Billing e Inventory, `InvoiceCloseOperation`, `StockOperation`, constraints locais e
referências lógicas sem FK entre serviços.

### Diagrama de Sequência: Compacto

[Visualizar em tamanho completo](./diagrams/03-invoice-close-sequence-compact.svg)

Mesmo fechamento da versão detalhada, com camadas internas colapsadas para leitura rápida.

### Diagrama de Sequência: Detalhado

[Visualizar em tamanho completo](./diagrams/03-invoice-close-sequence-detailed.svg)

Orquestração Billing → Inventory, aquisição exclusiva de uma operação lógica, blocos `alt` e
`critical`, idempotência, rollback, indisponibilidade e recuperação após resposta perdida.

---

## 8. Premissas Técnicas

São interpretações nossas, não requisitos atribuídos à Korp. A prova ou justificativa de cada uma
está em [`user-stories.md`](./user-stories.md#2-premissas).

| ID | Premissa |
|---|---|
| **A-1** | Código do produto é único |
| **A-2** | Número da nota é sequencial, gerado exclusivamente pelo Billing e não aceito do cliente |
| **A-3** | Não há autenticação; existe um usuário genérico |
| **A-4** | Criar nota não reserva nem reduz estoque; a baixa ocorre no fechamento |
| **A-5** | Nota `CLOSED` é imutável |
| **A-6** | Billing orquestra o fechamento; Inventory é autoridade sobre estoque |
| **A-7** | Não existem foreign keys atravessando microsserviços |
| **A-8** | Retries da mesma operação lógica reutilizam a mesma `Idempotency-Key` |
| **A-9** | Sequência da nota é monotônica e única, mas pode possuir lacunas após rollback |
| **A-10** | Timeout Billing → Inventory é inicialmente 3 segundos e configurável |
| **A-11** | Uma nota não aceita o mesmo `productId` em duas linhas; quantidades são agregadas defensivamente no Inventory |
| **A-12** | Uma `Invoice` possui uma única operação lógica efetiva de fechamento; a garantia é a constraint `UNIQUE(invoiceId)` no Billing |
| **A-13** | A criação rejeita quantidade acima do estoque consultado, sem reservar; o fechamento revalida e efetua a baixa |

### Fora do escopo

Autenticação e autorização; clientes e fornecedores; impostos; cálculo fiscal real; NF-e oficial;
XML fiscal; SEFAZ; certificados digitais; pagamentos; edição de nota fechada; Kubernetes; message
broker; API Gateway; dashboards complexos.

O assistente opcional foi implementado em um Cloudflare Worker. Ele interpreta texto e voz em
português do Brasil, mas toda escrita continua condicionada à confirmação explícita na interface.

### Convenção de idioma

Documentação, interface e mensagens ao usuário em português do Brasil. Identificadores técnicos,
como `Product`, `Invoice`, `InventoryService`, `productId`, `OPEN`, `CLOSED` e endpoints, permanecem
em inglês.
