import type { FundLedgerRow } from '#/api/joincare/fund_ledger';

import { describe, expect, it } from 'vitest';

import {
  collectLedgerExport,
  createLedgerWorkbook,
  exactText,
  ledgerExportCells,
} from './ledger-export';

const row = (id: string, patch = {}) =>
  ({
    id,
    entry_type: 'purchase_payment',
    amount_display: '10.000000000000000001',
    amount_raw: '10000000000000000001',
    tx_hash: '0xabc',
    ...patch,
  }) as unknown as FundLedgerRow;
describe('ledger Excel export', () => {
  it('exports all pages without changing page state and verifies the first page again', async () => {
    const rows = Array.from({ length: 205 }, (_, i) => row(String(i + 1)));
    const calls: number[] = [];
    const progress: number[] = [];
    const result = await collectLedgerExport(
      async (page, size) => {
        calls.push(page);
        expect(size).toBe(200);
        return {
          items: rows.slice((page - 1) * size, page * size),
          total: rows.length,
          page,
          page_size: size,
        };
      },
      () => true,
      (loaded) => progress.push(loaded),
    );
    expect(result).toHaveLength(205);
    expect(calls).toEqual([1, 2, 1]);
    expect(progress).toEqual([200, 205]);
  });
  it('cancellation, over-limit, duplicate, changed total and changed first page fail without a partial workbook', async () => {
    await expect(
      collectLedgerExport(
        async () => {
          throw new Error('must not request');
        },
        () => false,
        () => {},
      ),
    ).rejects.toThrow('取消');
    await expect(
      collectLedgerExport(
        async () => ({ items: [], total: 10_001, page: 1, page_size: 200 }),
        () => true,
        () => {},
      ),
    ).rejects.toThrow('10,000');
    await expect(
      collectLedgerExport(
        async () => ({
          items: [row('1'), row('1')],
          total: 2,
          page: 1,
          page_size: 200,
        }),
        () => true,
        () => {},
      ),
    ).rejects.toThrow('重复');
    let calls = 0;
    await expect(
      collectLedgerExport(
        async () => ({
          items: [row('1', { amount_raw: String(++calls) })],
          total: 1,
          page: 1,
          page_size: 200,
        }),
        () => true,
        () => {},
      ),
    ).rejects.toThrow('发生变化');
    calls = 0;
    await expect(
      collectLedgerExport(
        async (page) => ({
          items: Array.from({ length: 200 }, (_, i) => row(String(i))),
          total: ++calls === 1 ? 201 : 202,
          page,
          page_size: 200,
        }),
        () => true,
        () => {},
      ),
    ).rejects.toThrow('数量发生变化');
  });
  it('preserves exact long IDs and uint256 amounts, rejects already-rounded numeric IDs', () => {
    const cells = ledgerExportCells(
      [row('9007199254740993123')],
      '43368660559134718',
      { purchase_payment: '认购支付' },
    );
    expect(cells[1]).toContain('9007199254740993123');
    expect(cells[1]).toContain('10000000000000000001');
    expect(cells[1]).toContain('10.000000000000000001');
    expect(cells[1]?.[0]).toBe('43368660559134718');
    expect(() => exactText(9_007_199_254_740_992)).toThrow('安全精度');
  });
  it('exports empty results and stops after permission/context changes or request errors', async () => {
    const read = async () => ({ items: [], total: 0, page: 1, page_size: 200 });
    expect(
      await collectLedgerExport(
        read,
        () => true,
        () => {},
      ),
    ).toEqual([]);
    let current = true;
    await expect(
      collectLedgerExport(
        async () => {
          current = false;
          return read();
        },
        () => current,
        () => {},
      ),
    ).rejects.toThrow('取消');
    await expect(
      collectLedgerExport(
        async () => {
          throw new Error('forbidden');
        },
        () => true,
        () => {},
      ),
    ).rejects.toThrow('forbidden');
  });
  it('creates real ZIP/OOXML with text cells, escapes XML and never emits formulas', () => {
    const bytes = createLedgerWorkbook(
      [
        ['项目ID', '金额'],
        ['43368660559134718', '=HYPERLINK("bad")'],
        ['<tag>&', '10000000000000000001'],
      ],
      [['范围', '当前筛选']],
    );
    expect([...bytes.slice(0, 4)]).toEqual([80, 75, 3, 4]);
    const xml = new TextDecoder().decode(bytes);
    expect(xml).toContain('xl/workbook.xml');
    expect(xml).toContain(
      'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml',
    );
    expect(xml).toContain('t="inlineStr"');
    expect(xml).toContain('=HYPERLINK(&quot;bad&quot;)');
    expect(xml).toContain('&lt;tag&gt;&amp;');
    expect(xml).not.toContain('<f>');
    expect(xml).toContain('10000000000000000001');
    expect(() => createLedgerWorkbook([['x'.repeat(32_768)]], [])).toThrow(
      '长度限制',
    );
  });
});
