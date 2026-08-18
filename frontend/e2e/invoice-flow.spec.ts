import { expect, test, type Page } from '@playwright/test';

// Cada execução usa um código próprio para não depender do estado deixado por execuções anteriores.
const runId = Date.now();

test.describe('Jornada da nota fiscal', () => {
  test('cadastra produto, cria nota, fecha, imprime e reduz o estoque', async ({ page }) => {
    const code = `E2E-PROD-${runId}`;
    const description = 'Produto E2E';

    await registerProduct(page, { code, description, balance: '10' });

    await expect(productRow(page, code)).toBeVisible();
    await expect(balanceOf(page, code)).toHaveText('10');

    await createInvoice(page, { code, description, quantity: '2' });

    // A nota nasce aberta e mostra os snapshots gravados pelo Billing.
    await expect(page).toHaveURL(/\/invoices\/\d+$/);
    const invoiceUrl = page.url();
    await expect(page.getByText('Aberta')).toBeVisible();
    const itemRow = page.getByRole('row').filter({ hasText: code });
    await expect(itemRow).toContainText(description);
    await expect(itemRow).toContainText('2');

    // Criar a nota não consome estoque: o saldo continua 10 antes do fechamento.
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

  test('recusa o fechamento sem estoque, mantém a nota aberta e não imprime', async ({ page }) => {
    const code = `E2E-LOW-${runId}`;

    await registerProduct(page, { code, description: 'Produto E2E sem saldo', balance: '1' });
    await createInvoice(page, { code, description: 'Produto E2E sem saldo', quantity: '2' });

    const printCalls = await stubPrint(page);
    await page.getByRole('button', { name: 'Imprimir Nota' }).click();

    await expect(page.getByText('Estoque insuficiente para fechar a nota fiscal.')).toBeVisible();
    await expect(page.getByText('Aberta')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Imprimir Nota' })).toBeEnabled();
    expect(await printCalls()).toBe(0);

    await page.getByRole('link', { name: 'Produtos', exact: true }).click();
    await expect(balanceOf(page, code)).toHaveText('1');
  });
});

async function registerProduct(
  page: Page,
  product: { code: string; description: string; balance: string }
): Promise<void> {
  await page.goto('/products');
  await page.getByRole('link', { name: 'Novo produto' }).click();

  await page.getByLabel('Código').fill(product.code);
  await page.getByLabel('Descrição').fill(product.description);
  await page.getByLabel('Saldo').fill(product.balance);
  await page.getByRole('button', { name: 'Salvar' }).click();

  await expect(page).toHaveURL(/\/products$/);
}

async function createInvoice(
  page: Page,
  invoice: { code: string; description: string; quantity: string }
): Promise<void> {
  await page.getByRole('link', { name: 'Notas Fiscais', exact: true }).click();
  await page.getByRole('link', { name: /Nova nota|Criar primeira nota/ }).first().click();

  await page.getByLabel('Produto').click();
  await page.getByRole('option', { name: new RegExp(invoice.code) }).click();
  await page.getByLabel('Quantidade').fill(invoice.quantity);
  await page.getByRole('button', { name: 'Criar nota' }).click();

  await expect(page).toHaveURL(/\/invoices\/\d+$/);
}

function productRow(page: Page, code: string) {
  return page.getByRole('row').filter({ hasText: code });
}

function balanceOf(page: Page, code: string) {
  return productRow(page, code).locator('td').last();
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
