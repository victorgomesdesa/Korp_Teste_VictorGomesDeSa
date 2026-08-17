# Histórias de Usuário e Critérios de Aceitação

Documento complementar ao [`technical-design.md`](./technical-design.md). O design técnico descreve
**como** o sistema será construído; este documento define **o que ele deve fazer** e **como provamos
isso**.

Todo critério segue `Dado / Quando / Então` e pode falhar. Cada cenário da
[§19](#19-cenários-de-teste-derivados) deriva de um critério. O desafio da Korp é a fonte dos
requisitos; premissas e diferenciais adotados por nós estão identificados, sem serem apresentados
como exigências do enunciado.

---

## 1. Convenções

### 1.1 Vocabulário

| Termo | Definição |
|---|---|
| **Nota aberta** | `Invoice.status = OPEN`; pode ser fechada e ainda não consumiu estoque. |
| **Nota fechada** | `Invoice.status = CLOSED`; estoque consumido e conteúdo imutável. |
| **Imprimir Nota** | Uma única ação: solicita `POST /api/invoices/{id}/close`, aguarda `CLOSED`, atualiza a nota e inicia a impressão visual sem segundo clique. |
| **Operação lógica** | Uma tentativa de fechar uma nota e seus retries, todos com a mesma `Idempotency-Key`. |
| **Snapshot** | `productCode` e `productDescription` copiados para `InvoiceItem` na criação. |
| **Consumo atômico** | Todos os itens são baixados na mesma transação local do Inventory, ou nenhum é. |

### 1.2 Níveis de teste

`U` unitário (lógica pura, sem I/O) · `I` integração (API + PostgreSQL real e, quando indicado, os
dois serviços) · `E` ponta a ponta (navegador).

---

## 2. Premissas

Cada item é uma **interpretação nossa**, não um requisito atribuído à Korp.

**A-1: `Product.code` é único.** Um código identifica inequivocamente o produto; duplicidade retorna
`409 PRODUCT_CODE_ALREADY_EXISTS`. Provada por T-01.3.

**A-2: `Invoice.number` é sequencial e gerado exclusivamente pelo Billing.** O cliente não informa o
número. A sequência é monotônica e única; lacunas após rollback são aceitas conforme A-9. Provada por
T-03.2 e T-03.3.

**A-3: não há autenticação no desafio.** A interface representa um usuário genérico e não envia uma
identidade de negócio. É uma decisão de escopo justificada arquiteturalmente; T-E2E.1 percorre a
jornada sem login.

**A-4: criar uma nota não reserva nem reduz estoque.** A baixa acontece somente no fechamento.
Provada por T-03.4 e T-09.1.

**A-5: uma nota `CLOSED` é imutável.** Não há rota para editar ou excluir nota fechada. Provada por
T-08.1 e T-08.2.

**A-6: Billing orquestra o fechamento e Inventory é a autoridade sobre estoque.** O frontend não
consome estoque diretamente. Provada pelo contrato de integração em T-09.1 e pela arquitetura.

**A-7: não existem foreign keys atravessando microsserviços.** `InvoiceItem.productId` é referência
lógica e os snapshots preservam o histórico. Verificada estruturalmente por T-04.2 e migrations.

**A-8: retries da mesma operação lógica reutilizam a mesma `Idempotency-Key`.** Uma nova chave
representa outra operação. Provada por T-12.2 e T-14.1.

**A-9: a sequência pode possuir lacunas.** Rollback não obriga reutilizar números já alocados;
unicidade e ordem são mais importantes que continuidade. Provada por T-03.3.

**A-10: o timeout Billing → Inventory começa em 3 segundos e é configurável.** O valor torna o
comportamento verificável sem acoplar o domínio. Provada por T-11.1.

**A-11: o mesmo `productId` não pode aparecer duas vezes na nota.** A API responde `422`; Inventory
ainda agrega quantidades defensivamente. Isso elimina duas representações equivalentes do mesmo
pedido. Provada por T-04.3.

**A-12: uma `Invoice` possui uma única operação lógica efetiva de fechamento.** A interpretação é
materializada, não presumida: `InvoiceCloseOperation` possui `UNIQUE(invoiceId)` no PostgreSQL do
Billing. Assim, duas chaves concorrentes para a mesma nota não podem alcançar Inventory como duas
operações distintas. Provada por T-14.3.

---

## 3. Fixture de Testes

Infraestrutura de teste, não requisito da Korp.

| `id` | Código | Descrição | Saldo inicial |
|---:|---|---|---:|
| 1 | `PROD-001` | Teclado Mecânico | 10 |
| 2 | `PROD-002` | Mouse | 5 |
| 3 | `PROD-003` | Monitor | 1 |
| 4 | `PROD-004` | Cabo HDMI | 0 |

| Nota | Número | Status | Itens |
|---|---:|---|---|
| `INV-OPEN-1` | 1001 | `OPEN` | `PROD-001 × 2`, `PROD-002 × 1` |
| `INV-OPEN-LAST-A` | 1002 | `OPEN` | `PROD-003 × 1` |
| `INV-OPEN-LAST-B` | 1003 | `OPEN` | `PROD-003 × 1` |
| `INV-NO-STOCK` | 1004 | `OPEN` | `PROD-001 × 1`, `PROD-004 × 1` |
| `INV-CLOSED-1` | 1005 | `CLOSED` | `PROD-002 × 1` |

`INV-CLOSED-1` possui uma `InvoiceCloseOperation COMPLETED` associada à chave original da fixture.
Cada teste restaura a fixture ou usa uma transação isolada; cenários concorrentes partem do mesmo
saldo confirmado.

---

## 4. US-01: Cadastrar produto

> **Como** usuário, **quero** cadastrar um produto, **para** disponibilizá-lo para notas futuras.

**AC-01.1**: Dado `code = PROD-005`, descrição `Webcam` e `balance = 3`, quando o cadastro for
enviado, então a API responde `201 Created` e o produto persistido possui exatamente esses valores.

**AC-01.2**: Dado um cadastro com `code` vazio, descrição vazia ou `balance < 0`, quando enviado,
então a API responde `422` com código estável e nenhum produto é criado.

**AC-01.3**: Dado que `PROD-001` já existe, quando outro produto usar o mesmo código, então a API
responde `409 PRODUCT_CODE_ALREADY_EXISTS` e o produto original permanece inalterado. *(A-1)*

## 5. US-02: Visualizar produtos

> **Como** usuário, **quero** consultar produtos e saldos, **para** escolher itens para uma nota.

**AC-02.1**: Dada a fixture, quando `GET /api/products` for solicitado, então a resposta `200`
contém os quatro produtos e seus saldos atuais, inclusive `PROD-004` com saldo zero.

**AC-02.2**: Dado `productId = 1`, quando seus detalhes forem solicitados, então a API responde
`200` com `PROD-001`; para um id inexistente, responde `404 PRODUCT_NOT_FOUND`.

## 6. US-03: Criar nota fiscal

> **Como** usuário, **quero** criar uma nota, **para** preparar os itens antes do fechamento.

**AC-03.1**: Dados dois itens válidos, quando a nota for criada, então a API responde `201`, atribui
`status = OPEN`, `closedAt = null` e não aceita status informado pelo cliente.

**AC-03.2**: Dadas duas criações sucessivas, quando forem confirmadas, então os números gerados pelo
Billing são únicos e o segundo é maior que o primeiro. *(A-2)*

**AC-03.3**: Dado que uma transação de criação falha após alocar um número, quando a próxima nota for
criada, então ela recebe número maior e único; a lacuna não é reutilizada. *(A-9)*

**AC-03.4**: Dado `PROD-001` com saldo 10, quando uma nota `PROD-001 × 7` for criada, então o saldo
continua 10. *(A-4)*

**AC-03.5**: Dado um `productId` inexistente, quando a criação for enviada, então Billing responde
`404 PRODUCT_NOT_FOUND` e não persiste `Invoice` nem `InvoiceItem`.

**AC-03.6**: Dado Inventory indisponível durante a validação dos produtos, quando a criação for
enviada, então Billing responde `503 INVENTORY_SERVICE_UNAVAILABLE` e não persiste `Invoice` nem
`InvoiceItem`; uma tentativa manual posterior continua possível.

## 7. US-04: Adicionar produtos à nota

> **Como** usuário, **quero** incluir múltiplos produtos e quantidades, **para** representar a nota.

**AC-04.1**: Dados `PROD-001 × 2` e `PROD-002 × 1`, quando a nota for criada, então ela contém dois
`InvoiceItem` com as quantidades informadas.

**AC-04.2**: Dado um produto válido, quando a nota for criada e depois sua descrição for alterada no
Inventory, então a nota mantém `productCode` e `productDescription` originais. *(A-7)*

**AC-04.3**: Dada uma nota sem itens, com `quantity <= 0` ou `productId` duplicado, quando enviada,
então responde `422` (`INVALID_QUANTITY` quando aplicável) e nada é persistido. *(A-11)*

## 8. US-05: Visualizar notas fiscais

> **Como** usuário, **quero** listar notas, **para** acompanhar seu processamento.

**AC-05.1**: Dada a fixture, quando `GET /api/invoices` for solicitado, então responde `200` com
número, status e data das cinco notas, sem calcular status no frontend.

## 9. US-06: Visualizar detalhes da nota

> **Como** usuário, **quero** abrir uma nota, **para** ver status e itens.

**AC-06.1**: Dada `INV-OPEN-1`, quando seus detalhes forem solicitados, então a API responde `200`
com `OPEN`, dois itens, snapshots e quantidades; para id inexistente responde `404 INVOICE_NOT_FOUND`.

## 10. US-07: Imprimir/fechar uma nota aberta

> **Como** usuário, **quero** imprimir uma nota aberta, **para** concluir seu processamento.

**AC-07.1**: Dada `INV-OPEN-1` e uma chave inédita, quando **Imprimir Nota** for acionado, então o
frontend envia `POST /api/invoices/{id}/close`, a API responde `200` e a nota retorna `CLOSED` com
`closedAt` preenchido.

**AC-07.2**: Dado que o usuário acionou **Imprimir Nota**, quando o fechamento retornar sucesso,
então o mesmo fluxo atualiza a nota com `CLOSED` confirmado pelo backend e inicia a impressão visual,
sem exigir um segundo clique.

## 11. US-08: Impedir processamento de nota fechada

> **Como** sistema, **quero** rejeitar novo processamento, **para** preservar uma nota concluída.

**AC-08.1**: Dada `INV-CLOSED-1`, quando for fechada com uma chave diferente da operação original,
então responde `409 INVOICE_ALREADY_CLOSED` e nenhum saldo muda. *(A-5)*

**AC-08.2**: Dada uma nota `CLOSED`, quando qualquer alteração de itens ou exclusão for tentada,
então não existe rota aplicável (`404`/`405`) e a nota permanece inalterada. *(A-5)*

## 12. US-09: Atualizar estoque no fechamento

> **Como** sistema, **quero** consumir estoque ao fechar, **para** refletir os itens processados.

**AC-09.1**: Dada `INV-OPEN-1`, quando fechada, então `PROD-001` passa de 10 para 8,
`PROD-002` de 5 para 4 e somente depois a nota passa a `CLOSED`. *(A-4, A-6)*

## 13. US-10: Impedir fechamento com estoque insuficiente

> **Como** sistema, **quero** rejeitar falta de estoque sem baixa parcial, **para** manter consistência.

**AC-10.1**: Dada `INV-NO-STOCK`, quando fechada, então responde `409 INSUFFICIENT_STOCK` para
`PROD-004`, `PROD-001` continua 10, `PROD-004` continua 0 e a nota permanece `OPEN`.

## 14. US-11: Tratar indisponibilidade do Inventory Service

> **Como** usuário, **quero** receber um erro compreensível, **para** tentar novamente mais tarde.

**AC-11.1**: Dada uma nota `OPEN` e Inventory indisponível ou excedendo 3 segundos, quando o
fechamento for solicitado, então Billing responde `503 INVENTORY_SERVICE_UNAVAILABLE`, mantém a nota
`OPEN` e registra o erro com `request_id`. *(A-10)*

**AC-11.2**: Dado o `503`, quando a interface apresentar a falha, então exibe mensagem em português,
encerra o loading, não mostra stack trace/HTTP cru e permite nova tentativa manual.

## 15. US-12: Recuperar operação após falha

> **Como** sistema, **quero** recuperar uma baixa confirmada cuja resposta se perdeu, **para** concluir a nota sem consumo duplicado.

**AC-12.1**: Dado que Inventory consumiu os itens e gravou a `StockOperation`, mas Billing falhou
antes de fechar a nota, quando o estado for inspecionado, então a nota está `OPEN`, sua
`InvoiceCloseOperation` está `PROCESSING` com a chave original e cada saldo foi reduzido exatamente
uma vez.

**AC-12.2**: Dado o estado anterior, quando a mesma operação for repetida com a mesma chave, então
Inventory retorna o resultado anterior sem nova baixa, Billing conclui `OPEN → CLOSED`, marca
`InvoiceCloseOperation COMPLETED` e responde `200`. *(A-8)*

## 16. US-13: Proteger estoque contra concorrência

> **Como** sistema, **quero** serializar consumos concorrentes, **para** nunca produzir saldo negativo.

**AC-13.1**: Dado `PROD-003.balance = 1` e as notas A e B pedindo uma unidade, quando os dois
fechamentos alcançarem simultaneamente o `UPDATE ... WHERE balance >= 1` em conexões distintas,
então exatamente um responde `200`/fica `CLOSED`, o outro responde `409`/fica `OPEN`, e o saldo final
é 0, nunca -1.

## 17. US-14: Garantir idempotência

> **Como** sistema, **quero** reconhecer a mesma operação lógica, **para** tornar retries seguros.

**AC-14.1**: Dada uma nota fechada com sucesso usando a chave K, quando a mesma requisição com K for
repetida, então responde com resultado compatível ao primeiro `200`, existe uma única
`InvoiceCloseOperation`, uma única `StockOperation` e o estoque não muda novamente. *(A-8)*

**AC-14.2**: Dada uma chave K associada a uma nota e seus itens canônicos, quando K for reutilizada
para a mesma `invoiceId` com itens diferentes, então responde `409 IDEMPOTENCY_KEY_REUSED` e nenhum
estoque muda.

**AC-14.3**: Dada a mesma `Invoice OPEN`, quando duas requisições simultâneas usarem chaves A e B,
então exatamente uma cria/assume a `InvoiceCloseOperation`; a outra recebe
`409 INVOICE_CLOSE_ALREADY_IN_PROGRESS`, a nota termina `CLOSED`, o estoque é consumido uma vez e
existe uma única operação efetiva. *(A-12)*

**AC-14.4**: Dada uma chave K já associada à Invoice 1001 e aos itens `PROD-001 × 2`, quando K for
reutilizada para a Invoice 1002 com os mesmos itens, então a fingerprint, que inclui `invoiceId`, é
diferente; Inventory responde `409 IDEMPOTENCY_KEY_REUSED` e nenhum estoque muda.

## 18. US-15: Informar loading, sucesso e erros na interface

> **Como** usuário, **quero** feedback durante o fechamento, **para** não repetir ações nem interpretar erro cru.

**AC-15.1**: Dado um fechamento em andamento, quando a requisição estiver ativa, então a interface
exibe indicador de processamento e desabilita **Imprimir Nota**; em sucesso ou falha, `finalize`
encerra o loading e reabilita a ação quando o status permitir.

**AC-15.2**: Dadas respostas `409 INSUFFICIENT_STOCK`, `409 INVOICE_ALREADY_CLOSED`, `409
INVOICE_CLOSE_ALREADY_IN_PROGRESS` e `503 INVENTORY_SERVICE_UNAVAILABLE`, quando tratadas, então a
interface mostra mensagens específicas em português, sem stack trace, corpo JSON bruto ou decisão
local sobre o status.

---

## 19. Cenários de Teste Derivados

| ID | Critério | Cenário | Nível | Esperado |
|---|---|---|:---:|---|
| T-01.1 | AC-01.1 | Cadastrar `PROD-005`, Webcam, saldo 3 | I | `201`; valores persistidos |
| T-01.2 | AC-01.2 | Código/descrição vazios e saldos -1 | I | `422`; zero inserções |
| T-01.3 | AC-01.3 | Cadastrar outro `PROD-001` | I | `409 PRODUCT_CODE_ALREADY_EXISTS`; original intacto |
| T-02.1 | AC-02.1 | Listar produtos da fixture | I | Quatro produtos, inclusive saldo zero |
| T-02.2 | AC-02.2 | Buscar id 1 e id inexistente | I | `200 PROD-001`; `404 PRODUCT_NOT_FOUND` |
| T-03.1 | AC-03.1 | Criar nota com dois itens e tentar enviar status | I | Nasce `OPEN`; campo controlado pelo servidor |
| T-03.2 | AC-03.2 | Criar duas notas sucessivas | I | Números únicos e crescentes |
| T-03.3 | AC-03.3 | Forçar rollback após `nextval`, criar outra nota | I | Lacuna aceita; número não reutilizado |
| T-03.4 | AC-03.4 | Criar nota `PROD-001 × 7` | I | Saldo permanece 10 |
| T-03.5 | AC-03.5 | Criar nota com `productId = 999999` | I | `404 PRODUCT_NOT_FOUND`; nenhuma nota ou item |
| T-03.6 | AC-03.6 | Inventory offline durante a criação | I | `503`; nenhuma nota ou item |
| T-04.1 | AC-04.1 | Criar nota com dois produtos | I | Dois `InvoiceItem` |
| T-04.2 | AC-04.2 | Criar nota e alterar descrição no Inventory | I | Snapshot da nota permanece original; sem FK cruzada |
| T-04.3 | AC-04.3 | Vazio, quantidade 0/-1 ou `productId` duplicado | I | `422`; nenhuma nota ou item |
| T-05.1 | AC-05.1 | Listar notas | I | Cinco notas e status do backend |
| T-06.1 | AC-06.1 | Detalhar `INV-OPEN-1` e id inexistente | I | Detalhes completos; depois `404` |
| T-07.1 | AC-07.1 | Fechar `INV-OPEN-1` com chave inédita | I | `200`, `CLOSED`, `closedAt` preenchido |
| T-07.2 | AC-07.2 | Acionar Imprimir Nota | E | Um clique: loading → `CLOSED` confirmado → impressão visual |
| T-08.1 | AC-08.1 | Fechar `INV-CLOSED-1` com nova chave | I | `409`; saldos inalterados |
| T-08.2 | AC-08.2 | `PUT`/`PATCH`/`DELETE` em fechada | I | `404`/`405`; registro intacto |
| T-09.1 | AC-09.1 | Fechar `INV-OPEN-1` | I | Saldos 8 e 4; nota `CLOSED` |
| T-10.1 | AC-10.1 | Fechar `INV-NO-STOCK` | I | `409`; rollback de `PROD-001`; nota `OPEN` |
| T-11.1 | AC-11.1 | Inventory offline e depois resposta > 3 s | I | `503`; nota `OPEN`; log correlacionável |
| T-11.2 | AC-11.2 | Simular `503` no navegador | E | Mensagem controlada; loading termina; retry disponível |
| T-12.1 | AC-12.1 | Confirmar baixa e interromper Billing antes do update | I | Nota `OPEN`; operação Billing `PROCESSING`; uma baixa |
| T-12.2 | AC-12.2 | Retry de T-12.1 com a mesma chave | I | Sem nova baixa; nota `CLOSED`; operação `COMPLETED` |
| T-13.1 | AC-13.1 | Duas conexões e barreira antes do update do último Monitor | I | Um `200`, um `409`, saldo 0; uma nota `CLOSED` |
| T-14.1 | AC-14.1 | Mesma chave + mesma Invoice + mesmos itens | I | Mesmo resultado; uma operação em cada serviço; saldo estável |
| T-14.2 | AC-14.2 | Mesma chave + mesma Invoice + itens diferentes | I | `409 IDEMPOTENCY_KEY_REUSED`; nenhuma baixa |
| T-14.3 | AC-14.3 | Mesma Invoice, duas conexões e chaves A/B concorrentes | I | Um fechamento; outro `409`; uma operação Billing; uma baixa |
| T-14.4 | AC-14.4 | Mesma chave + Invoice diferente + mesmos itens | I | `409 IDEMPOTENCY_KEY_REUSED`; nenhuma baixa |
| T-15.1 | AC-15.1 | Manter resposta pendente e depois concluir/falhar | E | Loading e botão refletem a request ativa |
| T-15.2 | AC-15.2 | Injetar cada erro de domínio | U | Mensagem PT-BR específica; nenhum erro cru |
| T-E2E.1 | US-01 → US-09, US-15 | Cadastrar produtos → criar nota → ver `OPEN` → Imprimir Nota → ver `CLOSED` → consultar estoque | E | Nota fechada e saldos reduzidos exatamente pelas quantidades |

### 19.1 Força do teste de concorrência

T-13.1 usa duas conexões reais ao PostgreSQL e uma barreira para que ambas as operações iniciem com
o mesmo saldo. A checagem prévia de disponibilidade é substituída por um stub que permite as duas
tentativas. Assim, somente o `UPDATE` condicional e a transação do Inventory podem impedir o saldo
negativo. Se o teste passar sem essa proteção no banco, o teste está incorreto.

T-14.3 usa duas conexões reais ao PostgreSQL do Billing e uma barreira depois de ambas lerem a mesma
`Invoice OPEN`. As duas tentam inserir `InvoiceCloseOperation` com chaves distintas; somente
`UNIQUE(invoiceId)` pode escolher uma vencedora. O stub do cliente Inventory contabiliza chamadas e
deve receber exatamente uma. Se o teste passar sem a constraint, o teste está incorreto.

### 19.2 Rastreabilidade das premissas

| Premissa | Prova ou justificativa |
|---|---|
| A-1 | T-01.3 |
| A-2 | T-03.2, T-03.3 |
| A-3 | T-E2E.1 e decisão explícita de escopo |
| A-4 | T-03.4, T-09.1 |
| A-5 | T-08.1, T-08.2 |
| A-6 | T-09.1 e diagrama de sequência |
| A-7 | T-04.2 e revisão das migrations |
| A-8 | T-12.2, T-14.1 |
| A-9 | T-03.3 |
| A-10 | T-11.1 |
| A-11 | T-04.3 |
| A-12 | T-14.3 |

---

## 20. Notas

### 20.1 Regras de negócio consolidadas

| ID | Regra |
|---|---|
| RN-01 | Código do produto é único |
| RN-02 | Saldo nunca pode ficar negativo |
| RN-03 | Quantidade deve ser maior que zero |
| RN-04 | Toda nota nasce `OPEN` |
| RN-05 | Número é sequencial e gerado no backend |
| RN-06 | Nota `CLOSED` não pode ser processada novamente por outra operação |
| RN-07 | Criar nota não altera estoque |
| RN-08 | Fechar nota consome estoque |
| RN-09 | Estoque insuficiente em qualquer item impede consumo parcial |
| RN-10 | Nota fechada é imutável |
| RN-11 | Repetir uma operação idempotente não consome estoque novamente |
| RN-12 | Operações concorrentes nunca produzem saldo negativo |
| RN-13 | Uma `Invoice` possui no máximo uma operação lógica efetiva de fechamento |
| RN-14 | A `Idempotency-Key` identifica a operação de uma `Invoice` específica, não apenas seus produtos |

### 20.2 Fora do escopo deliberadamente

Autenticação e autorização; clientes e fornecedores; impostos; cálculo fiscal real; NF-e oficial;
XML fiscal; integração SEFAZ; certificados digitais; pagamentos; edição de nota fechada; Kubernetes;
message broker; API Gateway; dashboards complexos; IA no MVP.

IA aparece como opcional no desafio, mas foi deixada de fora para priorizar concorrência,
idempotência, resiliência e observabilidade, que protegem diretamente a confiabilidade do domínio.

### 20.3 Política de idioma

Documentação, interface e mensagens ao usuário em português do Brasil. Classes, campos, status,
serviços e rotas permanecem em inglês: `Product`, `InvoiceItem`, `OPEN`, `productId`,
`InventoryService` e `POST /api/invoices/{id}/close`.
