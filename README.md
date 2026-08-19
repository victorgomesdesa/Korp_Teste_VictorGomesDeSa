# Sistema de Emissão de Notas Fiscais

Aplicação full stack desenvolvida como desafio técnico da Korp para cadastro de produtos, criação de
notas fiscais e fechamento com baixa de estoque entre microsserviços.

O frontend em Angular consome dois serviços Go independentes: o **Inventory Service**, dono dos
produtos e do saldo, e o **Billing Service**, dono das notas fiscais. Cada serviço possui seu próprio
banco PostgreSQL. Criar uma nota não reserva nem reduz estoque; a baixa acontece somente no
fechamento, quando o Billing solicita o consumo ao Inventory. Esse ponto concentra os problemas
interessantes do desafio: concorrência sobre o mesmo saldo, idempotência da operação e recuperação
quando uma das partes falha no meio do caminho.

A documentação de arquitetura anterior ao desenvolvimento está em
[`docs/technical-design.md`](./docs/technical-design.md) e os critérios de aceitação em
[`docs/user-stories.md`](./docs/user-stories.md).

---

## Funcionalidades

- Cadastro e listagem de produtos, com consulta de saldo.
- Criação de notas fiscais com múltiplos itens.
- Snapshots de código e descrição do produto gravados na nota.
- Listagem e detalhe de notas, com status `OPEN`/`CLOSED`.
- Fechamento da nota pela ação **Imprimir Nota**, com baixa atômica de estoque.
- Idempotência do fechamento via `Idempotency-Key`.
- Proteção de concorrência no banco, sem lock de aplicação.
- Recuperação de operação após falha ou resposta perdida entre os serviços.
- Mensagens de erro controladas em português, sem detalhes técnicos na interface.
- Impressão da nota pelo navegador após o fechamento bem-sucedido.

Não existem edição ou exclusão de notas: uma nota `CLOSED` é imutável.

---

## Arquitetura

O frontend fala com os dois serviços: diretamente com o Inventory para produtos e com o Billing para
notas. A comunicação entre Billing e Inventory acontece apenas no servidor, no fechamento.

```text
Frontend Angular
      │
      ├──────────────► Inventory Service ──────► Inventory DB
      │                        ▲
      │                        │ POST /api/stock/consume
      │                        │
      └──────────────► Billing Service ────────► Billing DB
```

### Inventory Service

Responsável por `Product`, saldo, consumo atômico de estoque, `StockOperation`, controle de
concorrência e idempotência da baixa.

### Billing Service

Responsável por `Invoice`, `InvoiceItem`, `InvoiceCloseOperation`, criação com snapshots, fechamento,
orquestração com o Inventory e recuperação de operações interrompidas.

---

## Tecnologias

**Frontend:** Angular 22, TypeScript, Angular Material, RxJS, Signals para estado local.

**Backend:** Go 1.25, Gin, pgx/v5.

**Banco:** PostgreSQL 16.

**Infraestrutura:** Docker, Docker Compose, golang-migrate, GitHub Actions.

**Testes:** `testing` e `httptest` da biblioteca padrão do Go, Vitest com o runner de testes do
Angular, e Playwright para a jornada E2E do frontend.

---

## Estrutura do projeto

```text
.
├── frontend/                  Aplicação Angular
│   ├── src/app/core/          Configuração, interceptors e modelos compartilhados
│   ├── src/app/features/      Products e Invoices
│   ├── src/app/shared/        Loading, empty state e mensagem de erro
│   └── e2e/                   Jornada E2E com Playwright
├── services/
│   ├── billing-service/       Invoice, fechamento e orquestração
│   └── inventory-service/     Product, saldo e consumo de estoque
├── docs/
│   ├── technical-design.md
│   ├── user-stories.md
│   └── diagrams/
├── .github/workflows/ci.yml
├── docker-compose.yml
├── Makefile
└── .env.example
```

Cada serviço Go segue a mesma organização interna: `cmd/api`, `internal/http` (handler, middleware,
dto), `internal/service`, `internal/repository`, `internal/domain`, `migrations` e `tests`.

---

## Pré-requisitos

- Docker e Docker Compose.
- Node.js 22 e npm, para rodar o frontend e seus testes.
- Go não é necessário: os testes do backend rodam dentro de containers pelo Makefile.

---

## Configuração

O `docker-compose.yml` define valores padrão para todas as variáveis, então o ambiente sobe sem
nenhuma configuração adicional. O arquivo [`.env.example`](./.env.example) documenta as variáveis
disponíveis caso você queira sobrescrever portas, credenciais ou a origem liberada por CORS:

```bash
cp .env.example .env
```

As principais são `INVENTORY_SERVICE_PORT`, `BILLING_SERVICE_PORT`, as credenciais de cada banco,
`INVENTORY_SERVICE_URL` e `INVENTORY_SERVICE_TIMEOUT` (usadas pelo Billing para alcançar o Inventory)
e `INVENTORY_ALLOWED_ORIGIN`/`BILLING_ALLOWED_ORIGIN`, que liberam o frontend de desenvolvimento.

---

## Como executar

As migrations não rodam automaticamente na subida dos serviços. Cada microsserviço tem seu próprio
conjunto de migrations, aplicadas por containers `migrate/migrate` separados.

```bash
make up           # sobe bancos e serviços
make migrate-up   # aplica as migrations dos dois serviços
make frontend     # inicia o Angular em modo desenvolvimento
```

Com o ambiente no ar:

| Aplicação | URL |
|---|---|
| Frontend | http://localhost:4200 |
| Inventory Service | http://localhost:8080 |
| Billing Service | http://localhost:8081 |

Outros comandos úteis: `make ps`, `make logs`, `make build`, `make down` e `make reset` (este último
recria o ambiente e **apaga os dados locais**). `make help` lista todos os targets.

---

## API

Ambos os serviços expõem `GET /health` e respondem JSON. O envelope de erro é o mesmo nos dois.

### Inventory

| Método | Rota | Descrição |
|---|---|---|
| `POST` | `/api/products` | Cadastra produto |
| `GET` | `/api/products` | Lista produtos e saldos |
| `GET` | `/api/products/{id}` | Consulta produto |
| `POST` | `/api/stock/consume` | Consome estoque de forma atômica — **uso interno pelo Billing** |

### Billing

| Método | Rota | Descrição |
|---|---|---|
| `POST` | `/api/invoices` | Cria nota `OPEN` |
| `GET` | `/api/invoices` | Lista notas |
| `GET` | `/api/invoices/{id}` | Consulta nota com itens |
| `POST` | `/api/invoices/{id}/close` | Fecha a nota e consome o estoque |

`POST /api/invoices/{id}/close` exige o header `Idempotency-Key` e não recebe corpo.

### Exemplos

Criar produto:

```json
{ "code": "PROD-001", "description": "Teclado Mecânico", "balance": 10 }
```

Criar nota:

```json
{ "items": [{ "productId": 1, "quantity": 2 }] }
```

Fechar nota:

```http
POST /api/invoices/15/close
Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000
```

O cliente envia apenas `productId` e `quantity`. `number`, `status`, datas e os snapshots de código e
descrição são definidos pelo backend.

---

## Regras de negócio

- Código de produto é único.
- Saldo nunca fica negativo, garantido por `CHECK (balance >= 0)` além da lógica de consumo.
- Quantidade de item precisa ser maior que zero.
- Nota nasce `OPEN` com número sequencial gerado pelo banco.
- Criar nota não reserva nem reduz estoque.
- O fechamento é o único momento de baixa, e não existe baixa parcial.
- Nota `CLOSED` é imutável e não pode ser fechada novamente.
- Os itens guardam snapshots de código e descrição; alterações posteriores no produto não afetam
  notas existentes.
- Uma nota possui no máximo uma operação lógica de fechamento efetiva.

---

## Concorrência

A proteção do saldo fica no PostgreSQL, não na aplicação. O Inventory consome cada item com uma
atualização condicional dentro de uma transação:

```sql
UPDATE products
   SET balance = balance - $1,
       updated_at = NOW()
 WHERE id = $2
   AND balance >= $1;
```

`RowsAffected = 0` significa produto inexistente ou saldo insuficiente; a causa é confirmada dentro
da mesma transação e qualquer falha provoca `ROLLBACK` de todos os itens. Com duas notas disputando a
última unidade, exatamente uma atualização afeta uma linha e a outra recebe `409 INSUFFICIENT_STOCK`.
Os itens são agregados e ordenados por `productId` antes do consumo, o que evita deadlock quando duas
operações tocam os mesmos produtos em ordens diferentes.

No Billing, `UNIQUE(invoice_id)` em `invoice_close_operations` garante que duas requisições
simultâneas não iniciem dois fechamentos lógicos da mesma nota: uma assume a operação e a outra
recebe `409`. Não há mutex, lock em memória nem coordenação no processo Go, então o comportamento se
mantém com múltiplas instâncias.

---

## Idempotência

O fechamento exige `Idempotency-Key`, propagada pelo Billing ao Inventory. Cada lado persiste sua
própria operação: `InvoiceCloseOperation` no Billing e `StockOperation` no Inventory, ambas com
unicidade da chave no banco.

O Inventory calcula uma fingerprint sobre a representação canônica da operação — `invoiceId` mais os
itens agregados por produto e ordenados por `productId` — e a grava junto da chave, na mesma transação
da baixa. A partir daí:

- mesma chave e mesma operação lógica devolvem o resultado anterior, sem nova baixa;
- mesma chave com itens ou nota diferentes resultam em `409 IDEMPOTENCY_KEY_REUSED`.

Como a fingerprint ignora ordem e duplicações equivalentes dos itens, um retry que reenvie o mesmo
conteúdo em outra ordem continua sendo reconhecido como a mesma operação. É isso que impede a baixa
duplicada quando uma tentativa é repetida.

---

## Recuperação de falhas

Não existe transação ACID distribuída entre os serviços. A consistência vem de transações locais,
estado persistido e retry seguro pela `Idempotency-Key`.

O cenário central é o Inventory concluir a baixa e o Billing falhar antes de marcar a nota como
`CLOSED` — por timeout, queda ou resposta perdida. O estado resultante é observável: a nota continua
`OPEN`, a `InvoiceCloseOperation` continua `PROCESSING` com a chave original e o estoque foi reduzido
uma única vez.

Ao repetir o fechamento com a mesma chave, o Billing reconhece a operação `PROCESSING` como sua,
reenvia o consumo ao Inventory, que devolve o resultado já registrado sem consumir novamente, e então
conclui a nota para `CLOSED` e a operação para `COMPLETED`.

A distinção entre falha definitiva e ambígua orienta o que acontece com a chave:

- **definitiva** (`INSUFFICIENT_STOCK`, `PRODUCT_NOT_FOUND`): nada foi consumido, a operação é
  liberada e uma nova tentativa é outra operação lógica;
- **ambígua** (timeout, 5xx, resposta ilegível): o resultado é desconhecido, a operação permanece
  `PROCESSING` e a mesma chave é preservada para o retry.

---

## Observabilidade

Os dois serviços usam `log/slog` com saída JSON. Cada requisição recebe ou preserva um
`X-Request-Id`, que é devolvido na resposta, entra no contexto e é propagado pelo Billing na chamada
ao Inventory — a mesma correlação aparece nos logs dos dois serviços. Os registros relevantes
incluem `operation`, `invoice_id`, `product_id`, `operation_id`, `result`, `error_type` e
`duration_ms` quando a informação existe naquele ponto. Falhas inesperadas registram a causa apenas
no log; a resposta ao cliente permanece controlada.

Não há tracing distribuído nem coleta de métricas.

---

## Tratamento de erros

Toda falha responde com o mesmo envelope:

```json
{ "code": "INSUFFICIENT_STOCK", "message": "Estoque insuficiente para fechar a nota fiscal." }
```

Alguns códigos relevantes:

| Código | HTTP | Situação |
|---|---|---|
| `PRODUCT_CODE_ALREADY_EXISTS` | 409 | Código de produto já cadastrado |
| `PRODUCT_NOT_FOUND` | 404 | Produto inexistente |
| `INVOICE_NOT_FOUND` | 404 | Nota inexistente |
| `INSUFFICIENT_STOCK` | 409 | Saldo insuficiente para o fechamento |
| `INVOICE_ALREADY_CLOSED` | 409 | Nota já fechada por outra operação |
| `INVOICE_CLOSE_ALREADY_IN_PROGRESS` | 409 | Outra operação já assumiu o fechamento |
| `IDEMPOTENCY_KEY_REUSED` | 409 | Chave reutilizada para outra operação lógica |
| `INVENTORY_SERVICE_UNAVAILABLE` | 503 | Inventory indisponível ou resultado ambíguo |

Validações semânticas de payload respondem `422` e erros inesperados respondem `500 INTERNAL_ERROR`.

---

## Frontend

A aplicação é standalone, com rotas carregadas sob demanda e estado local em Signals. Os formulários
usam Reactive Forms tipados; a nota fiscal monta seus itens com `FormArray`, permitindo adicionar e
remover linhas, e bloqueia o mesmo produto repetido tanto no validator quanto desabilitando a opção
já escolhida nos outros itens. Produtos com saldo zero continuam selecionáveis, porque a criação não
depende de estoque.

Cada tela trata loading, estado vazio e erro de forma consistente: falhas de carregamento viram
estado de página com ação de tentar novamente, enquanto falhas de ação aparecem em snackbar. Um
interceptor adiciona `X-Request-Id` a todas as requisições.

No fechamento, a página gera uma `Idempotency-Key` para a operação lógica, desabilita o botão
enquanto processa e a mantém quando o resultado é ambíguo, de modo que um novo clique reenvia a mesma
chave. A impressão usa `window.print()` e só é disparada depois que a resposta de sucesso chega e a
tela já reflete `CLOSED`; um bloco `@media print` esconde navegação, botões e notificações. Não há
geração de PDF nem emissão fiscal.

---

## Testes

```bash
make test        # unitários de backend e frontend
make test-all    # unitários, integração e E2E
```

Targets individuais:

| Comando | Escopo |
|---|---|
| `make test-inventory` | Unitários do Inventory |
| `make test-billing` | Unitários do Billing |
| `make test-frontend` | Unitários do frontend |
| `make test-inventory-integration` | Integração do Inventory com PostgreSQL real e race detector |
| `make test-billing-integration` | Integração do Billing com PostgreSQL real e race detector |
| `make test-billing-e2e` | Billing ↔ Inventory sobre HTTP real, incluindo cenário com Inventory offline |
| `make test-frontend-e2e` | Jornada completa pelo navegador com Playwright |

Os testes de integração e E2E do backend usam bancos descartáveis do perfil `test` do Compose e
limpam os containers ao final. O E2E do frontend roda contra os serviços de desenvolvimento, cria
suas próprias fixtures com códigos únicos por execução e remove os dados ao terminar.

### Cenários críticos cobertos

- Duas notas disputando a última unidade: exatamente uma fecha, saldo final zero, nunca negativo.
- Duas chaves distintas fechando a mesma nota: uma única operação efetiva, uma única baixa.
- Mesma chave em requisições simultâneas: uma única `StockOperation` e um único `closedAt`.
- Rollback multi-item: falha em um item não deixa outro parcialmente consumido.
- Recuperação após resposta perdida: retry com a mesma chave conclui a nota sem segunda baixa.
- Jornada do usuário: cadastrar produto, criar nota, confirmar que o saldo não mudou, fechar,
  imprimir e ver o saldo reduzido.

---

## Integração contínua

O workflow [`ci.yml`](./.github/workflows/ci.yml) roda em push e pull request para `main`, com cinco
jobs: testes e build do frontend, `go vet` e unitários de cada serviço, integração e E2E do backend,
e por fim o E2E do frontend com Playwright. Os dois últimos sobem a stack com Docker Compose e
publicam logs em caso de falha. O pipeline não faz deploy.

---

## Documentação técnica

- [`docs/technical-design.md`](./docs/technical-design.md) — arquitetura, decisões e premissas.
- [`docs/user-stories.md`](./docs/user-stories.md) — histórias, critérios de aceitação e cenários de teste.
- [`docs/diagrams/`](./docs/diagrams/) — casos de uso, diagrama de classes e sequência do fechamento.

---

## Decisões técnicas

- **Bancos separados por serviço**, sem chave estrangeira cruzada: o `invoiceId` é referência lógica
  no Inventory.
- **Snapshots na nota**: código e descrição são copiados na criação, então a nota permanece fiel ao
  que foi emitido.
- **Transações locais e idempotência** no lugar de transação distribuída.
- **Garantias no banco** — atualização condicional, `UNIQUE` e `CHECK` — em vez de coordenação em
  memória.
- **Chave de idempotência controlada pela interface**, preservada em falhas ambíguas e descartada em
  falhas definitivas.

---

## Fora do escopo

- Autenticação e autorização.
- Emissão fiscal real, NF-e, SEFAZ ou cálculo de impostos.
- Edição e exclusão de notas.
- Mensageria, filas e orquestração externa.
- Kubernetes e deploy automatizado.
- Geração de PDF: a impressão usa o próprio navegador.