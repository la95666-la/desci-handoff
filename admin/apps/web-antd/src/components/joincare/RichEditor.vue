<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';

import { message, Space, Tooltip } from 'ant-design-vue';
// TipTap v3: named exports（v2 用 default）
import { Image } from '@tiptap/extension-image';
import { Link } from '@tiptap/extension-link';
import { Placeholder } from '@tiptap/extension-placeholder';
import { Table } from '@tiptap/extension-table';
import { TableCell } from '@tiptap/extension-table-cell';
import { TableHeader } from '@tiptap/extension-table-header';
import { TableRow } from '@tiptap/extension-table-row';
import { StarterKit } from '@tiptap/starter-kit';
import { EditorContent, useEditor } from '@tiptap/vue-3';

import { uploadFile } from '#/api/joincare/project_content';

const props = defineProps<{
  modelValue: string;
  placeholder?: string;
  minHeight?: string;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string];
}>();

const editor = useEditor({
  content: props.modelValue,
  extensions: [
    StarterKit,
    Image.configure({ inline: false, allowBase64: false }),
    Link.configure({ openOnClick: false, HTMLAttributes: { rel: 'noopener' } }),
    Table.configure({ resizable: true }),
    TableRow,
    TableHeader,
    TableCell,
    Placeholder.configure({ placeholder: props.placeholder || '开始输入...' }),
  ],
  onUpdate: ({ editor }) => {
    emit('update:modelValue', editor.getHTML());
  },
});

// 外部 v-model 变化时同步（编辑器只在初次加载 setContent；避免每次按键自反）
watch(
  () => props.modelValue,
  (v) => {
    const cur = editor.value?.getHTML();
    if (editor.value && v !== cur) {
      editor.value.commands.setContent(v || '', { emitUpdate: false });
    }
  },
);

onBeforeUnmount(() => {
  editor.value?.destroy();
});

// —— 工具栏 actions ———————————————————————————————————————

const uploadInput = ref<HTMLInputElement>();
const uploading = ref(false);

function chain() {
  return editor.value!.chain().focus();
}

async function onPickImage() {
  uploadInput.value?.click();
}
async function onImageFile(e: Event) {
  const target = e.target as HTMLInputElement;
  const file = target.files?.[0];
  if (!file) return;
  target.value = ''; // 允许再次选同一个文件
  if (!file.type.startsWith('image/')) {
    message.warning('仅支持图片');
    return;
  }
  uploading.value = true;
  try {
    const res = (await uploadFile(file)) as any;
    const url = res?.url;
    if (url) {
      chain().setImage({ src: url }).run();
    }
  } catch (err: any) {
    message.error(err?.message || '上传失败');
  } finally {
    uploading.value = false;
  }
}

function onAddLink() {
  const prev = editor.value?.getAttributes('link').href;
  const url = window.prompt('链接 URL', prev || 'https://');
  if (url === null) return; // 取消
  if (url === '') {
    chain().extendMarkRange('link').unsetLink().run();
    return;
  }
  chain()
    .extendMarkRange('link')
    .setLink({ href: url, target: '_blank' })
    .run();
}

function insertTable() {
  chain().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run();
}

const isActive = (name: string, attrs?: Record<string, any>) =>
  computed(() => editor.value?.isActive(name, attrs) === true);

const isBold = isActive('bold');
const isItalic = isActive('italic');
const isStrike = isActive('strike');
const isCode = isActive('code');
const isH1 = isActive('heading', { level: 1 });
const isH2 = isActive('heading', { level: 2 });
const isH3 = isActive('heading', { level: 3 });
const isBulletList = isActive('bulletList');
const isOrderedList = isActive('orderedList');
const isBlockquote = isActive('blockquote');
const isLink = isActive('link');
</script>

<template>
  <div class="rich-editor">
    <div v-if="editor" class="rich-editor__toolbar">
      <Space :size="4" wrap>
        <Tooltip title="加粗">
          <button
            type="button"
            class="btn"
            :class="{ active: isBold }"
            @click="chain().toggleBold().run()"
          >
            <strong>B</strong>
          </button>
        </Tooltip>
        <Tooltip title="斜体">
          <button
            type="button"
            class="btn italic"
            :class="{ active: isItalic }"
            @click="chain().toggleItalic().run()"
          >
            I
          </button>
        </Tooltip>
        <Tooltip title="删除线">
          <button
            type="button"
            class="btn"
            :class="{ active: isStrike }"
            @click="chain().toggleStrike().run()"
          >
            <s>S</s>
          </button>
        </Tooltip>
        <Tooltip title="代码">
          <button
            type="button"
            class="btn"
            :class="{ active: isCode }"
            @click="chain().toggleCode().run()"
          >
            &lt;/&gt;
          </button>
        </Tooltip>
        <span class="sep" />
        <Tooltip title="H1">
          <button
            type="button"
            class="btn"
            :class="{ active: isH1 }"
            @click="chain().toggleHeading({ level: 1 }).run()"
          >
            H1
          </button>
        </Tooltip>
        <Tooltip title="H2">
          <button
            type="button"
            class="btn"
            :class="{ active: isH2 }"
            @click="chain().toggleHeading({ level: 2 }).run()"
          >
            H2
          </button>
        </Tooltip>
        <Tooltip title="H3">
          <button
            type="button"
            class="btn"
            :class="{ active: isH3 }"
            @click="chain().toggleHeading({ level: 3 }).run()"
          >
            H3
          </button>
        </Tooltip>
        <span class="sep" />
        <Tooltip title="无序列表">
          <button
            type="button"
            class="btn"
            :class="{ active: isBulletList }"
            @click="chain().toggleBulletList().run()"
          >
            •
          </button>
        </Tooltip>
        <Tooltip title="有序列表">
          <button
            type="button"
            class="btn"
            :class="{ active: isOrderedList }"
            @click="chain().toggleOrderedList().run()"
          >
            1.
          </button>
        </Tooltip>
        <Tooltip title="引用">
          <button
            type="button"
            class="btn"
            :class="{ active: isBlockquote }"
            @click="chain().toggleBlockquote().run()"
          >
            "
          </button>
        </Tooltip>
        <span class="sep" />
        <Tooltip title="链接">
          <button
            type="button"
            class="btn"
            :class="{ active: isLink }"
            @click="onAddLink"
          >
            🔗
          </button>
        </Tooltip>
        <Tooltip title="图片（上传到 /uploads）">
          <button type="button" class="btn" @click="onPickImage">
            🖼<span v-if="uploading">...</span>
          </button>
        </Tooltip>
        <Tooltip title="表格 3×3">
          <button type="button" class="btn" @click="insertTable">⊞</button>
        </Tooltip>
        <span class="sep" />
        <Tooltip title="撤销">
          <button
            type="button"
            class="btn"
            :disabled="!editor.can().undo()"
            @click="chain().undo().run()"
          >
            ↶
          </button>
        </Tooltip>
        <Tooltip title="重做">
          <button
            type="button"
            class="btn"
            :disabled="!editor.can().redo()"
            @click="chain().redo().run()"
          >
            ↷
          </button>
        </Tooltip>
      </Space>
      <input
        ref="uploadInput"
        type="file"
        accept="image/*"
        style="display: none"
        @change="onImageFile"
      />
    </div>
    <EditorContent
      v-if="editor"
      :editor="editor"
      class="rich-editor__content"
      :style="{ minHeight: minHeight || '260px' }"
    />
  </div>
</template>

<style scoped>
.rich-editor {
  border: 1px solid var(--ant-color-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--ant-color-bg-container);
}
.rich-editor__toolbar {
  padding: 6px 8px;
  border-bottom: 1px solid var(--ant-color-border);
  background: var(--ant-color-bg-elevated);
}
.rich-editor__content {
  padding: 12px 16px;
  font-size: 14px;
  line-height: 1.7;
}
.rich-editor__content :deep(.ProseMirror) {
  outline: none;
  min-height: 100%;
}
.rich-editor__content :deep(.ProseMirror p.is-editor-empty:first-child::before) {
  content: attr(data-placeholder);
  float: left;
  color: var(--ant-color-text-placeholder);
  pointer-events: none;
  height: 0;
}
.rich-editor__content :deep(h1) {
  font-size: 22px;
  margin: 12px 0 6px;
  font-weight: 700;
}
.rich-editor__content :deep(h2) {
  font-size: 18px;
  margin: 12px 0 6px;
  font-weight: 700;
}
.rich-editor__content :deep(h3) {
  font-size: 16px;
  margin: 10px 0 4px;
  font-weight: 600;
}
.rich-editor__content :deep(blockquote) {
  border-left: 3px solid var(--ant-color-border);
  padding-left: 12px;
  color: var(--ant-color-text-secondary);
  margin: 8px 0;
}
.rich-editor__content :deep(code) {
  background: var(--ant-color-bg-layout);
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 90%;
}
.rich-editor__content :deep(img) {
  max-width: 100%;
  border-radius: 6px;
}
.rich-editor__content :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 8px 0;
}
.rich-editor__content :deep(th),
.rich-editor__content :deep(td) {
  border: 1px solid var(--ant-color-border);
  padding: 6px 10px;
  min-width: 60px;
}
.rich-editor__content :deep(th) {
  background: var(--ant-color-bg-layout);
  font-weight: 600;
}
.rich-editor__content :deep(ul),
.rich-editor__content :deep(ol) {
  padding-left: 24px;
  margin: 6px 0;
}
.btn {
  height: 26px;
  min-width: 30px;
  padding: 0 6px;
  border: 1px solid transparent;
  background: transparent;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
  color: var(--ant-color-text);
}
.btn:hover:not(:disabled) {
  background: var(--ant-color-bg-text-hover);
}
.btn.active {
  background: var(--ant-color-primary-bg);
  color: var(--ant-color-primary);
  border-color: var(--ant-color-primary-border);
}
.btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.btn.italic {
  font-style: italic;
}
.sep {
  width: 1px;
  height: 20px;
  background: var(--ant-color-split);
  display: inline-block;
  vertical-align: middle;
}
</style>
