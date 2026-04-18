<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NInput,
  NInputNumber,
  NButton,
  NButtonGroup,
  NColorPicker,
  NDivider,
  NGrid,
  NGridItem,
  NFormItem,
  NText,
} from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import type { Color } from '@/services/types'

const { t } = useI18n()
const projectStore = useProjectStore()

const selectedStyles = computed(() => projectStore.selectedStyles)

const firstStyle = computed(() => selectedStyles.value[0]?.style ?? null)
const firstFileId = computed(() => selectedStyles.value[0]?.fileId ?? '')

function update(field: string, value: unknown) {
  if (!firstStyle.value || !firstFileId.value) return
  projectStore.updateStyle(firstFileId.value, firstStyle.value.name, field, value)
}

// Color conversion: ASS Color {r,g,b,a} <-> CSS rgba string
// ASS: a=255 = opaque (note: this differs from CSS alpha=1 = opaque)
function colorToCSS(color: Color | undefined): string {
  if (!color) return 'rgba(255,255,255,1)'
  const alpha = color.a / 255
  return `rgba(${color.r},${color.g},${color.b},${alpha.toFixed(3)})`
}

function cssToColor(css: string): Color {
  // Parse rgba(r,g,b,a) or #rrggbbaa
  const rgbaMatch = css.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)(?:\s*,\s*([\d.]+))?\s*\)/)
  if (rgbaMatch) {
    const r = parseInt(rgbaMatch[1])
    const g = parseInt(rgbaMatch[2])
    const b = parseInt(rgbaMatch[3])
    const alpha = rgbaMatch[4] !== undefined ? parseFloat(rgbaMatch[4]) : 1
    return { r, g, b, a: Math.round(alpha * 255) }
  }
  // Fallback: try hex #rrggbbaa or #rrggbb
  const hexMatch = css.match(/^#([0-9a-fA-F]{8}|[0-9a-fA-F]{6})$/)
  if (hexMatch) {
    const hex = hexMatch[1]
    const r = parseInt(hex.slice(0, 2), 16)
    const g = parseInt(hex.slice(2, 4), 16)
    const b = parseInt(hex.slice(4, 6), 16)
    const a = hex.length === 8 ? parseInt(hex.slice(6, 8), 16) : 255
    return { r, g, b, a }
  }
  return { r: 255, g: 255, b: 255, a: 255 }
}

// Alignment grid: numpad layout 7-8-9 / 4-5-6 / 1-2-3
const alignmentRows = [
  [7, 8, 9],
  [4, 5, 6],
  [1, 2, 3],
]

function alignmentLabel(n: number): string {
  const map: Record<number, string> = {
    1: '↙', 2: '↓', 3: '↘',
    4: '←', 5: '·', 6: '→',
    7: '↖', 8: '↑', 9: '↗',
  }
  return map[n] ?? String(n)
}

// --- RAW debug view ---
// Format a style as the ASS "Style:" line matches our Go writer's output.
// Mirrors renderStyleLine in internal/parser/writer.go.

function boolToASS(b: boolean): string {
  return b ? '-1' : '0'
}

function formatASSColor(c: Color): string {
  // ASS format &HAABBGGRR — alpha is inverted (00=opaque, FF=transparent).
  const toHex = (n: number) => n.toString(16).toUpperCase().padStart(2, '0')
  const aa = 255 - c.a
  return `&H${toHex(aa)}${toHex(c.b)}${toHex(c.g)}${toHex(c.r)}`
}

function formatFloat4g(n: number): string {
  // Matches Go's %.4g formatter — up to 4 significant digits, strips trailing zeros.
  if (!isFinite(n)) return String(n)
  if (n === 0) return '0'
  const abs = Math.abs(n)
  let s: string
  if (abs >= 1e-4 && abs < 1e5) {
    s = n.toPrecision(4)
    if (s.includes('.')) {
      s = s.replace(/0+$/, '').replace(/\.$/, '')
    }
  } else {
    s = n.toExponential(3).replace(/e\+?(-?)0*(\d)/, 'e$1$2')
  }
  return s
}

function rawStyleLine(style: {
  name: string; fontName: string; fontSize: number
  bold: boolean; italic: boolean; underline: boolean; strikeout: boolean
  primaryColour: Color; secondaryColour: Color; outlineColour: Color; backColour: Color
  outline: number; shadow: number
  scaleX: number; scaleY: number; spacing: number; angle: number
  alignment: number; marginL: number; marginR: number; marginV: number
}): string {
  return (
    `Style: ${style.name},${style.fontName},${style.fontSize.toFixed(0)},` +
    `${formatASSColor(style.primaryColour)},${formatASSColor(style.secondaryColour)},` +
    `${formatASSColor(style.outlineColour)},${formatASSColor(style.backColour)},` +
    `${boolToASS(style.bold)},${boolToASS(style.italic)},${boolToASS(style.underline)},${boolToASS(style.strikeout)},` +
    `${style.scaleX.toFixed(0)},${style.scaleY.toFixed(0)},` +
    `${formatFloat4g(style.spacing)},${formatFloat4g(style.angle)},` +
    `1,${formatFloat4g(style.outline)},${formatFloat4g(style.shadow)},` +
    `${style.alignment},${style.marginL},${style.marginR},${style.marginV},1`
  )
}

const rawFormatHeader =
  'Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding'

const rawLines = computed(() =>
  selectedStyles.value.map(s => ({
    fileId: s.fileId,
    line: rawStyleLine(s.style),
  })),
)

const showRaw = ref(false)
</script>

<template>
  <div class="style-editor">
    <div class="style-editor-header">{{ t('editor.title') }}</div>

    <div v-if="selectedStyles.length === 0" class="no-selection">
      <NText depth="3">{{ t('editor.noSelection') }}</NText>
    </div>

    <div v-else-if="!firstStyle" class="no-selection">
      <NText depth="3">{{ t('editor.noSelection') }}</NText>
    </div>

    <div v-else class="editor-content">
      <div v-if="selectedStyles.length > 1" class="multi-badge">
        {{ t('editor.multipleSelected', { count: selectedStyles.length }) }}
      </div>

      <!-- Font -->
      <NFormItem :label="t('editor.fontName')" label-placement="left" label-width="100">
        <NInput
          :value="firstStyle.fontName"
          @update:value="(v) => update('fontName', v)"
          size="small"
        />
      </NFormItem>

      <NFormItem :label="t('editor.fontSize')" label-placement="left" label-width="100">
        <NInputNumber
          :value="firstStyle.fontSize"
          @update:value="(v) => update('fontSize', v ?? 0)"
          size="small"
          :min="1"
          :max="500"
          style="width: 100%"
        />
      </NFormItem>

      <!-- Style toggles -->
      <NFormItem :label="t('editor.bold')" label-placement="left" label-width="100">
        <NButtonGroup size="small">
          <NButton
            :type="firstStyle.bold ? 'primary' : 'default'"
            @click="update('bold', !firstStyle.bold)"
          >B</NButton>
          <NButton
            :type="firstStyle.italic ? 'primary' : 'default'"
            @click="update('italic', !firstStyle.italic)"
            style="font-style: italic"
          >I</NButton>
          <NButton
            :type="firstStyle.underline ? 'primary' : 'default'"
            @click="update('underline', !firstStyle.underline)"
            style="text-decoration: underline"
          >U</NButton>
          <NButton
            :type="firstStyle.strikeout ? 'primary' : 'default'"
            @click="update('strikeout', !firstStyle.strikeout)"
            style="text-decoration: line-through"
          >S</NButton>
        </NButtonGroup>
      </NFormItem>

      <NDivider style="margin: 8px 0" />

      <!-- Colors -->
      <NFormItem :label="t('editor.primaryColour')" label-placement="left" label-width="100">
        <NColorPicker
          :value="colorToCSS(firstStyle.primaryColour)"
          @update:value="(v) => update('primaryColour', cssToColor(v))"
          :modes="['rgb']"
          :show-alpha="true"
          size="small"
        />
      </NFormItem>

      <NFormItem :label="t('editor.secondaryColour')" label-placement="left" label-width="100">
        <NColorPicker
          :value="colorToCSS(firstStyle.secondaryColour)"
          @update:value="(v) => update('secondaryColour', cssToColor(v))"
          :modes="['rgb']"
          :show-alpha="true"
          size="small"
        />
      </NFormItem>

      <NFormItem :label="t('editor.outlineColour')" label-placement="left" label-width="100">
        <NColorPicker
          :value="colorToCSS(firstStyle.outlineColour)"
          @update:value="(v) => update('outlineColour', cssToColor(v))"
          :modes="['rgb']"
          :show-alpha="true"
          size="small"
        />
      </NFormItem>

      <NFormItem :label="t('editor.backColour')" label-placement="left" label-width="100">
        <NColorPicker
          :value="colorToCSS(firstStyle.backColour)"
          @update:value="(v) => update('backColour', cssToColor(v))"
          :modes="['rgb']"
          :show-alpha="true"
          size="small"
        />
      </NFormItem>

      <NDivider style="margin: 8px 0" />

      <!-- Outline & Shadow -->
      <NFormItem :label="t('editor.outline')" label-placement="left" label-width="100">
        <NInputNumber
          :value="firstStyle.outline"
          @update:value="(v) => update('outline', v ?? 0)"
          :min="0"
          :max="20"
          :step="0.5"
          size="small"
          style="width: 100%"
        />
      </NFormItem>

      <NFormItem :label="t('editor.shadow')" label-placement="left" label-width="100">
        <NInputNumber
          :value="firstStyle.shadow"
          @update:value="(v) => update('shadow', v ?? 0)"
          :min="0"
          :max="20"
          :step="0.5"
          size="small"
          style="width: 100%"
        />
      </NFormItem>

      <NDivider style="margin: 8px 0" />

      <!-- Scale -->
      <NFormItem :label="t('editor.scaleX')" label-placement="left" label-width="100">
        <NInputNumber
          :value="firstStyle.scaleX"
          @update:value="(v) => update('scaleX', v ?? 100)"
          :min="1"
          :max="500"
          size="small"
          style="width: 100%"
        />
      </NFormItem>

      <NFormItem :label="t('editor.scaleY')" label-placement="left" label-width="100">
        <NInputNumber
          :value="firstStyle.scaleY"
          @update:value="(v) => update('scaleY', v ?? 100)"
          :min="1"
          :max="500"
          size="small"
          style="width: 100%"
        />
      </NFormItem>

      <!-- Spacing & Angle -->
      <NFormItem :label="t('editor.spacing')" label-placement="left" label-width="100">
        <NInputNumber
          :value="firstStyle.spacing"
          @update:value="(v) => update('spacing', v ?? 0)"
          :min="0"
          :step="0.5"
          size="small"
          style="width: 100%"
        />
      </NFormItem>

      <NFormItem :label="t('editor.angle')" label-placement="left" label-width="100">
        <NInputNumber
          :value="firstStyle.angle"
          @update:value="(v) => update('angle', v ?? 0)"
          :min="-360"
          :max="360"
          :step="1"
          size="small"
          style="width: 100%"
        />
      </NFormItem>

      <NDivider style="margin: 8px 0" />

      <!-- Alignment grid -->
      <div class="form-label">{{ t('editor.alignment') }}</div>
      <div class="alignment-grid">
        <div v-for="row in alignmentRows" :key="row[0]" class="alignment-row">
          <NButton
            v-for="num in row"
            :key="num"
            size="small"
            :type="firstStyle.alignment === num ? 'primary' : 'default'"
            @click="update('alignment', num)"
            class="alignment-btn"
          >{{ alignmentLabel(num) }}</NButton>
        </div>
      </div>

      <NDivider style="margin: 8px 0" />

      <!-- Margins -->
      <NGrid :cols="3" :x-gap="8">
        <NGridItem>
          <NFormItem :label="t('editor.marginL')" label-placement="top">
            <NInputNumber
              :value="firstStyle.marginL"
              @update:value="(v) => update('marginL', v ?? 0)"
              :min="0"
              size="small"
              style="width: 100%"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem>
          <NFormItem :label="t('editor.marginR')" label-placement="top">
            <NInputNumber
              :value="firstStyle.marginR"
              @update:value="(v) => update('marginR', v ?? 0)"
              :min="0"
              size="small"
              style="width: 100%"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem>
          <NFormItem :label="t('editor.marginV')" label-placement="top">
            <NInputNumber
              :value="firstStyle.marginV"
              @update:value="(v) => update('marginV', v ?? 0)"
              :min="0"
              size="small"
              style="width: 100%"
            />
          </NFormItem>
        </NGridItem>
      </NGrid>

      <!-- RAW debug view (hidden by default) -->
      <div class="raw-debug">
        <button class="raw-toggle" @click="showRaw = !showRaw">
          {{ showRaw ? '▼' : '▶' }} RAW ({{ rawLines.length }})
        </button>
        <div v-if="showRaw" class="raw-body">
          <pre class="raw-header">{{ rawFormatHeader }}</pre>
          <pre
            v-for="(entry, idx) in rawLines"
            :key="idx"
            class="raw-line"
            :title="entry.fileId"
          >{{ entry.line }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.style-editor {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.style-editor-header {
  padding: 8px 12px;
  font-weight: 600;
  font-size: 13px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e6);
  flex-shrink: 0;
}

.no-selection {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  padding: 20px;
  text-align: center;
}

.editor-content {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px;
}

.multi-badge {
  font-size: 12px;
  color: var(--n-text-color-3, #999);
  margin-bottom: 8px;
}

.form-label {
  font-size: 13px;
  color: var(--n-text-color-2, #666);
  margin-bottom: 6px;
}

.alignment-grid {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 8px;
}

.alignment-row {
  display: flex;
  gap: 4px;
  justify-content: center;
}

.alignment-btn {
  width: 36px;
  height: 30px;
  font-size: 14px;
}

.raw-debug {
  margin-top: 12px;
  border-top: 1px dashed var(--n-border-color, #e0e0e6);
  padding-top: 8px;
}

.raw-toggle {
  background: none;
  border: none;
  color: var(--n-text-color-3, #888);
  cursor: pointer;
  font-size: 11px;
  font-family: 'Consolas', 'Courier New', monospace;
  padding: 2px 0;
  user-select: none;
}
.raw-toggle:hover {
  color: var(--n-text-color-2, #555);
}

.raw-body {
  margin-top: 6px;
  background: #f5f5f5;
  padding: 6px 8px;
  border-radius: 3px;
  max-height: 200px;
  overflow: auto;
}

.raw-header,
.raw-line {
  margin: 0;
  padding: 2px 0;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 10px;
  line-height: 1.4;
  white-space: pre;
  color: #333;
  word-break: keep-all;
}

.raw-header {
  color: #888;
  border-bottom: 1px solid #ddd;
  margin-bottom: 2px;
  padding-bottom: 2px;
}

.raw-line {
  cursor: text;
  user-select: text;
}
</style>
