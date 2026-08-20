import { expect, test, type Page } from '@playwright/test';

// Cada execução usa um código próprio para não depender do estado deixado por execuções anteriores.
const runId = Date.now();

test.describe('Jornada da nota fiscal', () => {
  test('cadastra produto, cria nota, fecha, imprime e reduz o estoque', async ({ page }) => {
    const codeNumber = `${runId}1`;
    const code = `PROD-${codeNumber}`;
    const description = 'E2E Produto principal';

    await registerProduct(page, { codeNumber, name: description, stock: '10', price: '99.90' });

    await expect(productRow(page, code)).toBeVisible();
    await expect(balanceOf(page, code)).toHaveText('10');

    await createInvoice(page, { code, quantity: '2' });

    // A nota nasce aberta e mostra os snapshots gravados pelo Billing.
    await expect(page).toHaveURL(/\/invoices\/\d+$/);
    const invoiceUrl = page.url();
    await expect(page.getByText('Aberta')).toBeVisible();
    const itemRow = page.getByRole('row').filter({ hasText: code });
    await expect(itemRow).toContainText(description);
    await expect(itemRow).toContainText('2');

    // Criar a nota não consome estoque: o estoque continua 10 antes do fechamento.
    await page.getByRole('link', { name: 'Produtos', exact: true }).click();
    await expect(balanceOf(page, code)).toHaveText('10');

    await page.goto(invoiceUrl);
    const printCalls = await stubPrint(page);
    await page.getByRole('button', { name: 'Imprimir Nota' }).click();

    await expect(page.getByText('Fechada', { exact: true })).toBeVisible();
    await expect(page.getByText('Fechada em')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Imprimir Nota' })).toHaveCount(0);
    expect(await printCalls()).toBe(1);

    await page.getByRole('link', { name: 'Produtos', exact: true }).click();
    await expect(balanceOf(page, code)).toHaveText('8');
  });

  test('recusa abrir uma nota acima do estoque disponível', async ({ page }) => {
    const codeNumber = `${runId}2`;
    const code = `PROD-${codeNumber}`;
    const description = 'E2E Produto sem estoque suficiente';

    await registerProduct(page, { codeNumber, name: description, stock: '1', price: '25' });
    await openInvoiceForm(page, code, '2');

    await expect(page.getByText('Disponível em estoque: 1 unidade.')).toBeVisible();
    const createButton = page.getByRole('button', { name: 'Criar nota' });
    await expect(createButton).toBeEnabled();
    await createButton.click();
    await expect(page).toHaveURL(/\/invoices\/new$/);

    await page.getByRole('link', { name: 'Produtos', exact: true }).click();
    await expect(balanceOf(page, code)).toHaveText('1');
  });
});

async function registerProduct(
  page: Page,
  product: { codeNumber: string; name: string; stock: string; price: string }
): Promise<void> {
  await page.goto('/products');
  await page.getByRole('link', { name: 'Novo produto' }).click();

  await page.getByLabel('Número do código').fill(product.codeNumber);
  await page.getByLabel('Nome').fill(product.name);
  await page.getByLabel('Estoque').fill(product.stock);
  await page.getByLabel('Valor/unidade').fill(product.price);
  await page.getByRole('button', { name: 'Salvar' }).click();

  await expect(page).toHaveURL(/\/products$/);
}

async function createInvoice(
  page: Page,
  invoice: { code: string; quantity: string }
): Promise<void> {
  await openInvoiceForm(page, invoice.code, invoice.quantity);
  await page.getByRole('button', { name: 'Criar nota' }).click();

  await expect(page).toHaveURL(/\/invoices\/\d+$/);
}

async function openInvoiceForm(page: Page, code: string, quantity: string): Promise<void> {
  await page.getByRole('link', { name: 'Notas Fiscais', exact: true }).click();
  await page.getByRole('link', { name: /Nova nota|Criar primeira nota/ }).first().click();

  await page.getByLabel('Produto').click();
  await page.getByRole('option', { name: new RegExp(code) }).click();
  await page.getByLabel('Quantidade').fill(quantity);
}

function productRow(page: Page, code: string) {
  return page.getByRole('row').filter({ hasText: code });
}

function balanceOf(page: Page, code: string) {
  return productRow(page, code).locator('td').nth(2);
}

// Substitui window.print para não abrir o diálogo real e permitir contar as chamadas.
async function stubPrint(page: Page): Promise<() => Promise<number>> {
  await page.evaluate(() => {
    (window as unknown as { __printCalls: number }).__printCalls = 0;
    window.print = () => {
      (window as unknown as { __printCalls: number }).__printCalls++;
    };
  });

  return () =>
    page.evaluate(() => (window as unknown as { __printCalls: number }).__printCalls);
}
