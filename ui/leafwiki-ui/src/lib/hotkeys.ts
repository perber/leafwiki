const modifierOrder = ['Mod', 'Shift', 'Alt'] as const

const modifierAliases: Record<string, (typeof modifierOrder)[number]> = {
  alt: 'Alt',
  command: 'Mod',
  cmd: 'Mod',
  control: 'Mod',
  ctrl: 'Mod',
  meta: 'Mod',
  mod: 'Mod',
  option: 'Alt',
  shift: 'Shift',
}

const keyAliases: Record<string, string> = {
  del: 'Delete',
  delete: 'Delete',
  enter: 'Enter',
  esc: 'Escape',
  escape: 'Escape',
  space: 'Space',
  spacebar: 'Space',
  tab: 'Tab',
}

function normalizeEventCode(code: string) {
  if (/^Key[A-Za-z]$/.test(code)) {
    return `Key${code.slice(3).toUpperCase()}`
  }

  if (/^Digit\d$/.test(code)) {
    return code
  }

  return ''
}

// Prefer physical key codes for letter and digit shortcuts so hotkeys stay
// layout-independent across non-Latin keyboard layouts such as Cyrillic,
// Greek, or Serbian. IME composition behavior is intentionally out of scope.
function normalizeHotkeyKey(key: string) {
  const trimmedKey = key.trim()
  if (!trimmedKey) {
    return ''
  }

  const aliasedKey = keyAliases[trimmedKey.toLowerCase()]
  if (aliasedKey) {
    return aliasedKey
  }

  const normalizedCode = normalizeEventCode(trimmedKey)
  if (normalizedCode) {
    return normalizedCode
  }

  if (/^[A-Za-z]$/.test(trimmedKey)) {
    return `Key${trimmedKey.toUpperCase()}`
  }

  if (/^\d$/.test(trimmedKey)) {
    return `Digit${trimmedKey}`
  }

  return trimmedKey
}

export function normalizeHotkeyCombo(keyCombo: string) {
  const parts = keyCombo
    .split('+')
    .map((part) => part.trim())
    .filter(Boolean)

  const modifierSet = new Set<(typeof modifierOrder)[number]>()
  const keys: string[] = []

  for (const part of parts) {
    const modifier = modifierAliases[part.toLowerCase()]
    if (modifier) {
      modifierSet.add(modifier)
      continue
    }

    keys.push(normalizeHotkeyKey(part))
  }

  return [
    ...modifierOrder.filter((modifier) => modifierSet.has(modifier)),
    ...keys,
  ]
    .filter(Boolean)
    .join('+')
}

export function getHotkeyComboFromEvent(event: KeyboardEvent) {
  const parts: string[] = []

  if (event.ctrlKey || event.metaKey) parts.push('Mod')
  if (event.shiftKey) parts.push('Shift')
  if (event.altKey) parts.push('Alt')

  const normalizedCode = normalizeEventCode(event.code)
  if (normalizedCode) {
    parts.push(normalizedCode)
  } else {
    parts.push(normalizeHotkeyKey(event.key))
  }

  return normalizeHotkeyCombo(parts.join('+'))
}
