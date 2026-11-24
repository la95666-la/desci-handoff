import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

describe('news cover upload', () => {
  it('reuses the project-content uploader and fills the cover URL', () => {
    const source = readFileSync(
      new URL('../index.vue', import.meta.url),
      'utf8',
    );

    expect(source).toContain(
      "import { uploadFile } from '#/api/joincare/project_content';",
    );
    expect(source).toContain('const res = (await uploadFile(file)) as any;');
    expect(source).toContain('form.cover_url = res.url;');
    expect(source).toContain('accept="image/*"');
    expect(source).toContain('@change="onCoverChange"');
    expect(source).toContain('alt="新闻封面预览"');
  });
});
