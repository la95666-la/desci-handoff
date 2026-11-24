import type { FundLedgerRow } from '#/api/joincare/fund_ledger';

export interface LedgerExportScope {
  projectId: string;
  entryType?: string;
  status?: string;
}

export interface LedgerExportPage {
  items: FundLedgerRow[];
  total: number;
  page: number;
  page_size: number;
}

export const LEDGER_EXPORT_LIMIT = 10_000;

// Page API is not a database snapshot. Abort on visible mutations rather than
// silently downloading partial/duplicated financial history as a complete file.
export async function collectLedgerExport(
  read: (page: number, pageSize: number) => Promise<LedgerExportPage>,
  current: () => boolean,
  progress: (loaded: number, total: number) => void,
) {
  const rows: FundLedgerRow[] = [];
  const ids = new Set<string>();
  let total: number | undefined;
  let first = '';
  const check = () => {
    if (!current()) throw new Error('导出已取消，或项目/筛选条件已变化');
  };
  for (let page = 1; ; page++) {
    check();
    const result = await read(page, 200);
    check();
    if (
      !Number.isSafeInteger(result.total) ||
      result.total < 0 ||
      !Array.isArray(result.items)
    )
      throw new Error('账本响应无效');
    if (result.total > LEDGER_EXPORT_LIMIT)
      throw new Error('记录超过10,000条，请缩小筛选范围后导出');
    total ??= result.total;
    if (
      total !== result.total ||
      result.page !== page ||
      result.page_size !== 200
    )
      throw new Error('账本分页或记录数量发生变化，请刷新后重新导出');
    if (page === 1) first = JSON.stringify(result.items);
    for (const row of result.items) {
      const id = exactText(row.id);
      if (!id || ids.has(id))
        throw new Error('账本记录重复或ID无效，请刷新后重新导出');
      ids.add(id);
      rows.push(row);
    }
    if (
      rows.length > total ||
      (rows.length < total && result.items.length !== 200)
    )
      throw new Error('账本分页发生变化，未生成文件，请重试');
    progress(rows.length, total);
    if (rows.length === total) break;
  }
  check();
  const verify = await read(1, 200);
  check();
  if (verify.total !== total || JSON.stringify(verify.items) !== first)
    throw new Error('导出期间账本发生变化，未生成文件，请重试');
  return rows;
}

export function exactText(value: unknown): string {
  if (typeof value === 'number' && !Number.isSafeInteger(value))
    throw new Error('账本ID超出安全精度，请联系管理员');
  return value === null || value === undefined ? '' : String(value);
}

const columns: [string, keyof FundLedgerRow][] = [
  ['账本ID', 'id'],
  ['链ID', 'chain_id'],
  ['合约地址', 'contract_address'],
  ['合约版本', 'contract_version'],
  ['类型代码', 'entry_type'],
  ['资产角色', 'asset_role'],
  ['资产', 'token_symbol'],
  ['代币地址', 'token_address'],
  ['代币精度', 'token_decimals'],
  ['金额', 'amount_display'],
  ['原始金额', 'amount_raw'],
  ['钱包地址', 'wallet_address'],
  ['对手方地址', 'counterparty_address'],
  ['X账号', 'twitter_handle'],
  ['X名称', 'twitter_name'],
  ['TG账号', 'telegram_username'],
  ['TG名称', 'telegram_name'],
  ['状态', 'status'],
  ['区块', 'block_number'],
  ['区块哈希', 'block_hash'],
  ['交易哈希', 'tx_hash'],
  ['日志索引', 'log_index'],
  ['创建时间（原始时区）', 'created_at'],
  ['更新时间（原始时区）', 'updated_at'],
];

export function ledgerExportCells(
  rows: FundLedgerRow[],
  projectId: string,
  labels: Record<string, string>,
) {
  return [
    ['项目ID', '类型', ...columns.map(([label]) => label)],
    ...rows.map((row) => [
      projectId,
      labels[row.entry_type] || row.entry_type,
      ...columns.map(([, key]) => exactText(row[key])),
    ]),
  ];
}

function xml(value: string) {
  if (value.length > 32_767)
    throw new Error('账本字段超过Excel单元格长度限制，未生成文件');
  // XML 1.0 permits tab/LF/CR and printable Unicode only.
  return [...value]
    .filter((char) => {
      const code = char.codePointAt(0) ?? 0;
      return (
        code === 9 ||
        code === 10 ||
        code === 13 ||
        (code >= 32 && code <= 0xd7_ff) ||
        (code >= 0xe0_00 && code <= 0xff_fd) ||
        code >= 0x1_00_00
      );
    })
    .join('')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;');
}

// Minimal standards-compliant OOXML workbook. All cells are inline strings:
// preserves uint256/long IDs and prevents formula injection (=,+,-,@).
export function createLedgerWorkbook(
  cells: string[][],
  metadata: string[][],
): Uint8Array {
  const sheet = (rows: string[][]) =>
    `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><sheetData>${rows.map((row, i) => `<row r="${i + 1}">${row.map((value) => `<c t="inlineStr"><is><t xml:space="preserve">${xml(value)}</t></is></c>`).join('')}</row>`).join('')}</sheetData></worksheet>`;
  return zipStored({
    '[Content_Types].xml':
      '<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>',
    '_rels/.rels':
      '<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>',
    'xl/workbook.xml':
      '<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="逐笔账本" sheetId="1" r:id="rId1"/><sheet name="导出说明" sheetId="2" r:id="rId2"/></sheets></workbook>',
    'xl/_rels/workbook.xml.rels':
      '<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/></Relationships>',
    'xl/worksheets/sheet1.xml': sheet(cells),
    'xl/worksheets/sheet2.xml': sheet(metadata),
  });
}

function zipStored(files: Record<string, string>) {
  const encode = new TextEncoder();
  const directory: Uint8Array[] = [];
  const parts: Uint8Array[] = [];
  let offset = 0;
  const header = (size: number) => {
    const bytes = new Uint8Array(size);
    return { bytes, view: new DataView(bytes.buffer) };
  };
  const crc = (bytes: Uint8Array) => {
    let n = 0xff_ff_ff_ff;
    for (const byte of bytes) {
      n ^= byte;
      for (let bit = 0; bit < 8; bit++)
        n = (n >>> 1) ^ (n & 1 ? 0xed_b8_83_20 : 0);
    }
    const signed = n ^ 0xff_ff_ff_ff;
    return signed < 0 ? signed + 0x1_00_00_00_00 : signed;
  };
  for (const [path, text] of Object.entries(files)) {
    const data = encode.encode(text);
    const checksum = crc(data);
    const name = encode.encode(path);
    const local = header(30);
    local.view.setUint32(0, 0x04_03_4b_50, true);
    local.view.setUint16(4, 20, true);
    local.view.setUint32(14, checksum, true);
    local.view.setUint32(18, data.length, true);
    local.view.setUint32(22, data.length, true);
    local.view.setUint16(26, name.length, true);
    parts.push(local.bytes, name, data);
    const central = header(46);
    central.view.setUint32(0, 0x02_01_4b_50, true);
    central.view.setUint16(4, 20, true);
    central.view.setUint16(6, 20, true);
    central.view.setUint32(16, checksum, true);
    central.view.setUint32(20, data.length, true);
    central.view.setUint32(24, data.length, true);
    central.view.setUint16(28, name.length, true);
    central.view.setUint32(42, offset, true);
    directory.push(central.bytes, name);
    offset += 30 + name.length + data.length;
  }
  const directorySize = directory.reduce((n, p) => n + p.length, 0);
  const end = header(22);
  end.view.setUint32(0, 0x06_05_4b_50, true);
  end.view.setUint16(8, Object.keys(files).length, true);
  end.view.setUint16(10, Object.keys(files).length, true);
  end.view.setUint32(12, directorySize, true);
  end.view.setUint32(16, offset, true);
  const output = new Uint8Array(offset + directorySize + 22);
  let position = 0;
  for (const part of [...parts, ...directory, end.bytes]) {
    output.set(part, position);
    position += part.length;
  }
  return output;
}
