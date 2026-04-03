<script setup lang="ts">
import { computed } from 'vue'
import type { SubtitleStyle, Color } from '@/services/types'

interface Props {
  style: SubtitleStyle | null
  text: string
}

const props = defineProps<Props>()

function colorToCSS(color: Color | undefined, fallback = 'white'): string {
  if (!color) return fallback
  const alpha = color.a / 255
  return `rgba(${color.r},${color.g},${color.b},${alpha.toFixed(3)})`
}

// ASS alignment (numpad 1-9) → CSS absolute positioning
// 1=bottom-left, 2=bottom-center, 3=bottom-right
// 4=middle-left, 5=middle-center, 6=middle-right
// 7=top-left, 8=top-center, 9=top-right
function getAlignmentCSS(alignment: number, marginL: number, marginR: number, marginV: number): Record<string, string> {
  const ml = `${marginL}px`
  const mr = `${marginR}px`
  const mv = `${marginV}px`

  const pos: Record<string, string> = {}

  // Vertical
  if (alignment >= 7) {
    pos.top = mv
  } else if (alignment >= 4) {
    pos.top = '50%'
    pos.transform = 'translateY(-50%)'
  } else {
    pos.bottom = mv
  }

  // Horizontal
  const col = ((alignment - 1) % 3) // 0=left, 1=center, 2=right
  if (col === 0) {
    pos.left = ml
    pos.textAlign = 'left'
  } else if (col === 1) {
    pos.left = '0'
    pos.right = '0'
    pos.textAlign = 'center'
  } else {
    pos.right = mr
    pos.textAlign = 'right'
  }

  return pos
}

const overlayStyle = computed(() => {
  const s = props.style
  if (!s) return {}

  const alignment = s.alignment ?? 2
  const alignCSS = getAlignmentCSS(alignment, s.marginL ?? 0, s.marginR ?? 0, s.marginV ?? 20)

  // Text stroke for outline
  const outlineColor = colorToCSS(s.outlineColour, 'black')
  const outline = s.outline ?? 0
  const textStroke = outline > 0 ? `${outline}px ${outlineColor}` : 'none'

  // Shadow
  const shadowColor = colorToCSS(s.backColour, 'rgba(0,0,0,0.5)')
  const shadow = s.shadow ?? 0
  const textShadow = shadow > 0 ? `${shadow}px ${shadow}px 0 ${shadowColor}` : 'none'

  // Scale transform
  const scaleX = (s.scaleX ?? 100) / 100
  const scaleY = (s.scaleY ?? 100) / 100
  const angle = s.angle ?? 0

  // Build transform
  const transforms: string[] = []
  if (alignCSS.transform) transforms.push(alignCSS.transform)
  if (scaleX !== 1 || scaleY !== 1) transforms.push(`scale(${scaleX}, ${scaleY})`)
  if (angle !== 0) transforms.push(`rotate(${angle}deg)`)

  const result: Record<string, string> = {
    position: 'absolute',
    fontFamily: s.fontName ?? 'Arial',
    fontSize: `${s.fontSize ?? 32}px`,
    color: colorToCSS(s.primaryColour, 'white'),
    fontWeight: s.bold ? 'bold' : 'normal',
    fontStyle: s.italic ? 'italic' : 'normal',
    textDecoration: [
      s.underline ? 'underline' : '',
      s.strikeout ? 'line-through' : '',
    ].filter(Boolean).join(' ') || 'none',
    letterSpacing: `${s.spacing ?? 0}px`,
    WebkitTextStroke: textStroke,
    textShadow,
    lineHeight: '1.2',
    maxWidth: '90%',
    wordBreak: 'break-word',
    padding: '2px 4px',
    ...alignCSS,
  }

  if (transforms.length > 0) {
    result.transform = transforms.join(' ')
  }

  return result
})
</script>

<template>
  <div class="css-subtitle-overlay" v-if="style && text">
    <span :style="overlayStyle">{{ text }}</span>
  </div>
</template>

<style scoped>
.css-subtitle-overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}
</style>
