/** Extract basename from a path (handles both / and \ separators) */
export function basename(path: string): string {
  return path.replace(/^.*[/\\]/, '')
}

/** Remove file extension from a basename */
export function stripExt(name: string): string {
  const idx = name.lastIndexOf('.')
  return idx >= 0 ? name.slice(0, idx) : name
}

/**
 * Detect episode numbers from a list of video paths by finding the
 * variable digit sequence between common prefix and common suffix.
 * Returns a map from videoPath to episode number (or null if not detected).
 */
export function detectEpisodeNumbers(videoPaths: string[]): Map<string, number | null> {
  const result = new Map<string, number | null>()
  if (videoPaths.length === 0) return result
  if (videoPaths.length === 1) {
    result.set(videoPaths[0], null)
    return result
  }

  const names = videoPaths.map(p => basename(p))

  // Longest common prefix
  let prefix = names[0]
  for (const n of names) {
    while (prefix && !n.startsWith(prefix)) {
      prefix = prefix.slice(0, -1)
    }
    if (!prefix) break
  }

  // Longest common suffix
  let suffix = names[0]
  for (const n of names) {
    while (suffix && !n.endsWith(suffix)) {
      suffix = suffix.slice(1)
    }
    if (!suffix) break
  }

  // Make sure prefix+suffix don't overlap (edge case for very similar names)
  if (prefix.length + suffix.length > names[0].length) {
    const totalLen = names[0].length
    const maxSuffix = totalLen - prefix.length
    suffix = suffix.slice(-maxSuffix)
  }

  for (let i = 0; i < names.length; i++) {
    const middle = names[i].slice(prefix.length, names[i].length - suffix.length).trim()
    // Extract first digit run
    const m = middle.match(/\d+/)
    const num = m ? parseInt(m[0], 10) : NaN
    result.set(videoPaths[i], isNaN(num) ? null : num)
  }
  return result
}

/**
 * Collapse a sorted list of integers into range notation.
 * [1,2,3,5,6,7,10] → "1-3, 5-7, 10"
 * [1] → "1"
 * [] → ""
 */
export function collapseRanges(nums: number[]): string {
  if (nums.length === 0) return ''
  const sorted = [...new Set(nums)].sort((a, b) => a - b)
  const parts: string[] = []
  let start = sorted[0]
  let end = sorted[0]
  for (let i = 1; i <= sorted.length; i++) {
    if (i < sorted.length && sorted[i] === end + 1) {
      end = sorted[i]
    } else {
      parts.push(start === end ? `${start}` : `${start}-${end}`)
      if (i < sorted.length) {
        start = sorted[i]
        end = sorted[i]
      }
    }
  }
  return parts.join(', ')
}

/**
 * Compute the suffix of a subtitle file name relative to its video.
 * e.g. video "ep01.mkv" + sub "ep01.rus.[Anku].ass" → ".rus.[Anku].ass"
 * If the sub name doesn't share the video's base name, returns the full sub name.
 */
export function subtitleSuffix(videoPath: string, subPath: string): string {
  const videoBase = stripExt(basename(videoPath))
  const subName = basename(subPath)
  if (subName.startsWith(videoBase)) {
    return subName.slice(videoBase.length)
  }
  // Fallback: use full sub name as suffix
  return subName
}
