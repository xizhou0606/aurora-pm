export type TableColumnOption<TId extends string = string> = {
  id: TId
  label: string
  required?: boolean
  fixed?: boolean
  defaultHidden?: boolean
}

export type TableColumnPreference<TId extends string = string> = {
  order: TId[]
  hidden: TId[]
}

export function normalizeTableColumnPreference<TId extends string>(
  columns: readonly TableColumnOption<TId>[],
  stored?: Partial<TableColumnPreference<string>>,
): TableColumnPreference<TId> {
  const valid = new Set(columns.map(({ id }) => id))
  const knownOrder = (stored?.order || []).filter((id): id is TId => valid.has(id as TId))
  const requestedOrder = [...new Set([...knownOrder, ...columns.map(({ id }) => id)])]
  const fixedColumns = columns
    .map((column, index) => ({ ...column, index }))
    .filter((column) => column.fixed)
  const order = requestedOrder.filter((id) => !fixedColumns.some((column) => column.id === id))
  fixedColumns.forEach((column) => {
    order.splice(Math.min(column.index, order.length), 0, column.id)
  })
  const required = new Set(columns.filter((column) => column.required).map(({ id }) => id))
  const storedFields = new Set([...(stored?.order || []), ...(stored?.hidden || [])])
  const defaultHidden = columns
    .filter((column) => column.defaultHidden && !storedFields.has(column.id))
    .map(({ id }) => id)
  const hidden = [...new Set([...(stored?.hidden || []), ...defaultHidden])]
    .filter((id): id is TId => valid.has(id as TId) && !required.has(id as TId))
  return { order, hidden }
}

export function applyTableColumnPreference<TColumn extends { id?: string; accessorKey?: unknown }>(
  columns: readonly TColumn[],
  preference: TableColumnPreference,
): TColumn[] {
  const byId = new Map(columns.map((column) => [
    column.id || (typeof column.accessorKey === 'string' ? column.accessorKey : ''),
    column,
  ]))
  return preference.order.flatMap((id) => {
    if (preference.hidden.includes(id)) return []
    const column = byId.get(id)
    return column ? [column] : []
  })
}
