import { useState } from 'react'
import { CHOICE_COLORS, type Choice, type ChoiceColor, type FieldOptions } from '../model/types'
const emptyChoice = (index: number): Choice => ({ id: crypto.randomUUID(), name: '', color: CHOICE_COLORS[index % (CHOICE_COLORS.length - 1) + 1] })
export function useBitableFieldChoices(multiple: boolean, initial?: FieldOptions) {
  const [choices, setChoices] = useState<readonly Choice[]>(() => initial?.choices ? initial.choices.map(choice => ({ ...choice })) : [emptyChoice(0), emptyChoice(3)])
  const [selected, setSelected] = useState<readonly string[]>(() => typeof initial?.defaultValue === 'string' ? [initial.defaultValue] : initial?.defaultValue ? [...initial.defaultValue] : [])
  const normalized = choices.map(choice => ({ ...choice, name: choice.name.trim() }))
  const valid = normalized.length > 0 && normalized.every(choice => choice.name.length > 0) && new Set(normalized.map(choice => choice.name)).size === normalized.length
  const defaults = selected.filter(id => normalized.some(choice => choice.id === id && choice.name))
  const value = multiple ? defaults : defaults.slice(0, 1)
  const options: FieldOptions = value.length ? { defaultValue: multiple ? value : value[0] } : {}
  return {
    choices, selected: value, normalized, options, valid,
    add: () => setChoices(items => [...items, emptyChoice(items.length)]),
    update: (id: string, patch: { name?: string; color?: ChoiceColor }) => setChoices(items => items.map(item => item.id === id ? { ...item, ...patch } : item)),
    remove: (id: string) => { setChoices(items => items.filter(item => item.id !== id)); setSelected(items => items.filter(item => item !== id)) },
    select: (id: string) => setSelected(items => multiple ? items.includes(id) ? items.filter(item => item !== id) : [...items, id] : items[0] === id ? [] : [id]),
    clear: () => setSelected([]),
    move: (id: string, to: number) => setChoices(items => {
      const from = items.findIndex(item => item.id === id)
      if (from < 0 || to < 0 || to >= items.length || from === to) return items
      const next = [...items], [item] = next.splice(from, 1); next.splice(to, 0, item); return next
    }),
  }
}
