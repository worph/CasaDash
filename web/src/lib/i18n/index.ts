import { writable, derived } from 'svelte/store'
import en_us from './lang/en_us.json'
import fr_fr from './lang/fr_fr.json'
import de_de from './lang/de_de.json'
import zh_cn from './lang/zh_cn.json'

type Dict = Record<string, string>

// One JSON per language, keyed by semantic id (CasaOS ships 31 languages; the
// structure here supports adding the rest without code changes).
const messages: Record<string, Dict> = { en_us, fr_fr, de_de, zh_cn }

export const languages = [
  { code: 'en_us', name: 'English' },
  { code: 'fr_fr', name: 'Français' },
  { code: 'de_de', name: 'Deutsch' },
  { code: 'zh_cn', name: '中文' },
]

export const locale = writable('en_us')

/** Reactive translator: `$t('app')`. Falls back to en_us then the key itself. */
export const t = derived(
  locale,
  ($l) =>
    (key: string): string =>
      messages[$l]?.[key] ?? messages['en_us'][key] ?? key,
)
